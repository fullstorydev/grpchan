package shmgrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fullstorydev/grpchan"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/siadat/ipc"
)

// Server is grpc over shared memory. It
// acts as a grpc ServiceRegistrar
type Server struct {
	handlers     grpchan.HandlerMap
	basePath     string
	ShmQueueInfo QueueInfo
	opts         handlerOpts
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
	for _, o := range opts {
		o.apply(&s)
	}
	return &s
}

// Attempt of a HandlerFunc
// type HandlerFunc func(Response, *http.Request)

// func (f HandlerFunc) ServeSHM() {
// 	f()
// }

func (s *Server) RegisterService(desc *grpc.ServiceDesc, svr interface{}) {
	s.handlers.RegisterService(desc, svr)

	//Instantiate polling go routine
	go func() {

		msg_req := &ipc.Msgbuf{
			Mtype: s.ShmQueueInfo.QueueReqType}

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

		//Mesrcv on response message type
		err = ipc.Msgrcv(qid, msg_req, 0)
		if err != nil {
			panic(fmt.Sprintf("SERVER: Failed to receive message to ipc id %d: %s\n", qid, err))
		} else {
			fmt.Printf("SERVER: Message %v receive to ipc id %d\n", msg_req.Mtext, qid)
		}

		//Parse bytes into object
		var message_req ShmMessage
		json.Unmarshal(msg_req.Mtext, message_req)

		fmt.Println(message_req)
	}()

	// Handle Methods?
	// for i := range desc.Methods {
	// 	md := desc.Methods[i]
	// 	go handleMethod(svr, desc.ServiceName, &md)
	// }
}

// func handleMethod(svr interface{}, serviceName string, desc *grpc.MethodDesc) func() {

// 	fullMethod := fmt.Sprintf("/%s/%s", serviceName, desc.MethodName)
// 	fmt.Println(fullMethod)
// 	return f()
// }
