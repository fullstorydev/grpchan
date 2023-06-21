package shmgrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"

	"github.com/fullstorydev/grpchan/internal"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	grpcproto "google.golang.org/grpc/encoding/proto"

	"github.com/siadat/ipc"
)

type Channel struct {
	ShmQueueInfo *QueueInfo
	//URL of endpoint (might be useful in the future)
	BaseURL *url.URL
	//shm state info etc that might be needed
}

var _ grpc.ClientConnInterface = (*Channel)(nil)

func (ch *Channel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {

	qid := ch.ShmQueueInfo.Qid

	//Get Call Options for
	copts := internal.GetCallOptions(opts)

	//Get headersFromContext
	reqUrl := *ch.BaseURL
	reqUrl.Path = path.Join(reqUrl.Path, methodName)
	reqUrlStr := reqUrl.String()

	ctx, err := internal.ApplyPerRPCCreds(ctx, copts, fmt.Sprintf("shm:0%s", reqUrlStr), true)
	if err != nil {
		return err
	}

	codec := encoding.GetCodec(grpcproto.Name)

	temp_serialized_payload, err := codec.Marshal(req)
	if err != nil {
		return err
	}

	message_request := &ShmMessage{
		Method:  methodName,
		Context: ctx,
		Headers: headersFromContext(ctx),
		Payload: ByteSlice2String(temp_serialized_payload),
	}

	// we have the meta request
	// Marshall to build rest of system
	serialized_message, err := json.Marshal(message_request)
	if err != nil {
		return err
	}

	// //Alternative serialization
	// alt_message := &ShmMessage{
	// 	Method:  methodName,
	// 	Context: ctx,
	// 	Headers: headersFromContext(ctx),
	// }

	// // we have the meta request
	// // Marshall to build rest of system
	// alt_serialized, err := json.Marshal(alt_message)
	// if err != nil {
	// 	return err
	// }

	// final_serial := append(alt_serialized, temp_serialized_payload...)

	// fmt.Println(bytes.Equal(serialized_message, final_serial)) // true

	// pass into shared mem queue
	msg_req := &ipc.Msgbuf{
		Mtype: ch.ShmQueueInfo.QueueReqTypeMeta, //Request message type
		Mtext: serialized_message,               // Isnt this technically serialization?
	}

	err = ipc.Msgsnd(qid, msg_req, 0)
	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to send message to ipc id %d: %s\n", qid, err))
	} else {
		// fmt.Printf("CLIENT: Message %v send to ipc id %d\n", msg_req, qid)
	}

	//Receive metadata payload
	msg_resp_meta := &ipc.Msgbuf{
		Mtype: ch.ShmQueueInfo.QueueRespTypeMeta}

	err = ipc.Msgrcv(qid, msg_resp_meta, 0)
	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to receive meta message to ipc id %d: %s\n", qid, err))
	} else {
		// fmt.Printf("CLIENT: Metadata Message %v meta receive to ipc id %d\n", msg_resp_meta.Mtext, qid)
	}

	//Parse bytes into object
	var message_resp_meta ShmMessage
	json.Unmarshal(msg_resp_meta.Mtext, &message_resp_meta)
	payload := unsafeGetBytes(message_resp_meta.Payload)

	copts.SetHeaders(message_resp_meta.Headers)
	copts.SetTrailers(message_resp_meta.Trailers)

	// ipc.Msgctl(qid, ipc.IPC_RMID)
	return codec.Unmarshal(payload, resp)
}

func (ch *Channel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}
