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
)

type Channel struct {
	ShmQueueInfo *QueueInfo
	//URL of endpoint (might be useful in the future)
	BaseURL *url.URL
	//shm state info etc that might be needed
	DetachQueue chan bool
}

var _ grpc.ClientConnInterface = (*Channel)(nil)

func (ch *Channel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {

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

	serialized_payload, err := codec.Marshal(req)
	if err != nil {
		return err
	}

	message_request := &ShmMessage{
		Method:  methodName,
		Context: ctx,
		Headers: headersFromContext(ctx),
		Payload: ByteSlice2String(serialized_payload),
	}

	// we have the meta request
	// Marshall to build rest of system
	serialized_message, err := json.Marshal(message_request)
	if err != nil {
		return err
	}

	//START MESSAGING
	requestQueue := GetQueue(ch.ShmQueueInfo.RequestShmaddr)
	responseQueue := GetQueue(ch.ShmQueueInfo.ResponseShmaddr)

	var data [600]byte
	len := copy(data[:], serialized_message)

	message := Message{
		Header: MessageHeader{Size: int32(len)},
		Data:   data,
	}

	// pass into shared mem queue
	produceMessage(requestQueue, message, ch.DetachQueue)

	//Receive Request
	read_message, err := consumeMessage(responseQueue, ch.DetachQueue)
	if err != nil {
		//This should hopefully not happen
		return err
	}

	//Parse bytes into object
	slice := read_message.Data[0:read_message.Header.Size]

	var message_resp_meta ShmMessage
	json.Unmarshal(slice, &message_resp_meta)
	payload := unsafeGetBytes(message_resp_meta.Payload)

	copts.SetHeaders(message_resp_meta.Headers)
	copts.SetTrailers(message_resp_meta.Trailers)

	// ipc.Msgctl(qid, ipc.IPC_RMID)
	return codec.Unmarshal(payload, resp)
}

func (ch *Channel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}
