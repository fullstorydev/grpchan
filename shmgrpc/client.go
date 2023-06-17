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
	ShmQueueInfo QueueInfo
	//URL of endpoint (might be useful in the future)
	BaseURL *url.URL
	//shm state info etc that might be needed
}

var _ grpc.ClientConnInterface = (*Channel)(nil)

func (ch *Channel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {

	//Generate Key
	key, err := ipc.Ftok(ch.ShmQueueInfo.QueuePath, ch.ShmQueueInfo.QueueId)
	if err != nil {
		panic(err)
	}

	//Get qid
	qid, err := ipc.Msgget(key, ipc.IPC_CREAT|0666)
	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to create ipc key %d: %s\n", key, err))
	} else {
		// fmt.Printf("CLIENT: Create ipc queue id %d\n", qid)
	}

	//Get Call Options for
	copts := internal.GetCallOptions(opts)

	//Get headersFromContext
	reqUrl := *ch.BaseURL
	reqUrl.Path = path.Join(reqUrl.Path, methodName)
	reqUrlStr := reqUrl.String()

	ctx, err = internal.ApplyPerRPCCreds(ctx, copts, fmt.Sprintf("shm:0%s", reqUrlStr), true)
	if err != nil {
		return err
	}

	message_request_meta := &ShmMessage{
		Method:  methodName,
		Context: ctx,
		Headers: headersFromContext(ctx),
		// Trailers: ,
	}
	// we have the meta request
	// Marshall to build rest of system
	serialized_message_meta, err := json.Marshal(message_request_meta)
	if err != nil {
		return err
	}
	// pass into shared mem queue
	msg_req_meta := &ipc.Msgbuf{
		Mtype: ch.ShmQueueInfo.QueueReqTypeMeta, //Request message type
		Mtext: serialized_message_meta,          // Isnt this technically serialization?
	}

	codec := encoding.GetCodec(grpcproto.Name)

	serialized_payload, err := codec.Marshal(req)
	// pass into shared mem queue
	msg_req := &ipc.Msgbuf{
		Mtype: ch.ShmQueueInfo.QueueReqType, //Request message type
		Mtext: serialized_payload,           // Isnt this technically serialization?
	}

	err = ipc.Msgsnd(qid, msg_req_meta, 0)
	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to send message to ipc id %d: %s\n", qid, err))
	} else {
		// fmt.Printf("CLIENT: Metadata Message %v send to ipc id %d\n", msg_req, qid)
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

	copts.SetHeaders(message_resp_meta.Headers)
	copts.SetTrailers(message_resp_meta.Trailers)

	//Construct receive buffer (this will have to change)
	msg_resp := &ipc.Msgbuf{
		Mtype: ch.ShmQueueInfo.QueueRespType}

	err = ipc.Msgrcv(qid, msg_resp, 0)
	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to receive message to ipc id %d: %s\n", qid, err))
	} else {
		// fmt.Printf("CLIENT: Message %v receive to ipc id %d\n", msg_resp.Mtext, qid)
	}

	// ipc.Msgctl(qid, ipc.IPC_RMID)
	return codec.Unmarshal(msg_resp.Mtext, resp)
}

func (ch *Channel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}
