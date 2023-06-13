package shmgrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/internal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	grpcproto "google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siadat/ipc"
)

// Server is grpc over shared memory. It
// acts as a grpc ServiceRegistrar
type Server struct {
	handlers         grpchan.HandlerMap
	basePath         string
	ShmQueueInfo     QueueInfo
	opts             handlerOpts
	unaryInterceptor grpc.UnaryServerInterceptor
}

var _ grpc.ServiceRegistrar = (*Server)(nil)

// ServerOption is an option used when constructing a NewServer.
type ServerOption interface {
	apply(*Server)
}

type serverOptFunc func(*Server)

func (fn serverOptFunc) apply(s *Server) {
	fn(s)
}

type handlerOpts struct {
	errFunc func(context.Context, *status.Status, http.ResponseWriter)
}

func NewServer(ShmQueueInfo QueueInfo, basePath string, opts ...ServerOption) *Server {
	var s Server
	s.basePath = basePath
	s.handlers = grpchan.HandlerMap{}
	s.ShmQueueInfo = ShmQueueInfo
	return &s
}

// Attempt of a HandlerFunc
// type HandlerFunc func(Response, *http.Request)

// func (f HandlerFunc) ServeSHM() {
// 	f()
// }

func (s *Server) RegisterService(desc *grpc.ServiceDesc, svr interface{}) {
	s.handlers.RegisterService(desc, svr)
	s.unaryInterceptor = nil

	//Instantiate queue for processing
	key, err := ipc.Ftok(s.ShmQueueInfo.QueuePath, s.ShmQueueInfo.QueueId)
	if err != nil {
		panic(fmt.Sprintf("SERVER: Failed to generate key: %s\n", err))
	} else {
		fmt.Printf("SERVER: Generate key %d\n", key)
	}

	qid, err := ipc.Msgget(key, ipc.IPC_CREAT|0666)
	if err != nil {
		panic(fmt.Sprintf("SERVER: Failed to create ipc key %d: %s\n", key, err))
	} else {
		fmt.Printf("SERVER: Create ipc queue id %d\n", qid)
	}

	//Check for valid meta data message
	msg_req_meta := &ipc.Msgbuf{
		Mtype: s.ShmQueueInfo.QueueReqTypeMeta}

	//Mesrcv on response message type
	err = ipc.Msgrcv(qid, msg_req_meta, 0)
	if err != nil || msg_req_meta.Mtext == nil {
		panic(fmt.Sprintf("SERVER: Failed to receive metadata message to ipc id %d: %s\n", qid, err))
	} else {
		fmt.Printf("SERVER: Metadata Message %v receive to ipc id %d\n", msg_req_meta.Mtext, qid)
	}

	//Parse bytes into object
	var message_req_meta ShmMessage
	json.Unmarshal(msg_req_meta.Mtext, &message_req_meta)

	fullName := message_req_meta.Method
	strs := strings.SplitN(fullName[1:], "/", 2)
	serviceName := strs[0]
	methodName := strs[1]
	clientCtx := message_req_meta.Context
	if clientCtx == nil { // Temp in case of empty context.
		clientCtx = context.Background()
	}

	//Instantiate handling go routine
	go func() {

		msg_req := &ipc.Msgbuf{
			Mtype: s.ShmQueueInfo.QueueReqType}

		err = ipc.Msgrcv(qid, msg_req, 0)
		if err != nil {
			panic(fmt.Sprintf("SERVER: Failed to receive message to ipc id %d: %s\n", qid, err))
		} else {
			fmt.Printf("SERVER: Message %v receive to ipc id %d\n", msg_req_meta.Mtext, qid)
		}

		//Get Service Descriptor and Handler
		sd, handler := s.handlers.QueryService(serviceName)
		if sd == nil {
			// service name not found
			status.Errorf(codes.Unimplemented, "service %s not implemented", message_req_meta.Method)
		}

		//Get Method Descriptor
		md := internal.FindUnaryMethod(methodName, sd.Methods)
		if md == nil {
			// method name not found
			status.Errorf(codes.Unimplemented, "method %s/%s not implemented", serviceName, methodName)
		}

		//Get Codec for content type.
		codec := encoding.GetCodec(grpcproto.Name)

		// Function to unmarshal payload using proto
		dec := func(msg interface{}) error {
			val := msg_req.Mtext
			if err := codec.Unmarshal(val, msg); err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			return nil
		}

		// Implements server transport stream
		sts := internal.UnaryServerTransportStream{Name: methodName}
		ctx := grpc.NewContextWithServerTransportStream(makeServerContext(clientCtx), &sts)

		//Get resp write back
		resp, err := md.Handler(
			handler,
			ctx,
			dec,
			s.unaryInterceptor)
		if err != nil {
			status.Errorf(codes.Unknown, "Handler error: %s ", err.Error())
			//TODO: Error code must be sent back to client
		}

		sts.GetHeaders()
		sts.GetTrailers()

		serialized_payload, err := codec.Marshal(resp)
		if err != nil {
			status.Errorf(codes.Unknown, "Codec Marshalling error: %s ", err.Error())
		}

		//Construct respond buffer (this will have to change)
		msg_resp := &ipc.Msgbuf{
			Mtype: s.ShmQueueInfo.QueueRespType,
			Mtext: serialized_payload,
		}
		//Write back
		err = ipc.Msgsnd(qid, msg_resp, 0)
		if err != nil {
			panic(fmt.Sprintf("SERVER: Failed to send resp to ipc id %d: %s\n", qid, err))
		} else {
			fmt.Printf("SERVER: Message %v send to ipc id %d\n", msg_req, qid)
		}
		//We now have method name
	}()

}

// noValuesContext wraps a context but prevents access to its values. This is
// useful when you need a child context only to propagate cancellations and
// deadlines, but explicitly *not* to propagate values.
type noValuesContext struct {
	context.Context
}

func makeServerContext(ctx context.Context) context.Context {
	// We don't want the server have any of the values in the client's context
	// since that can inadvertently leak state from the client to the server.
	// But we do want a child context, just so that request deadlines and client
	// cancellations work seamlessly.
	newCtx := context.Context(noValuesContext{ctx})

	if meta, ok := metadata.FromOutgoingContext(ctx); ok {
		newCtx = metadata.NewIncomingContext(newCtx, meta)
	}
	// newCtx = peer.NewContext(newCtx, &inprocessPeer)
	// newCtx = context.WithValue(newCtx, &clientContextKey, ctx)
	return newCtx
}

// func handleMethod(svr interface{}, serviceName string, desc *grpc.MethodDesc) func() {

// 	fullMethod := fmt.Sprintf("/%s/%s", serviceName, desc.MethodName)
// 	fmt.Println(fullMethod)
// 	return f()
// }
