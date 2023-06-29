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

	grpchantesting "github.com/fullstorydev/grpchan/grpchantesting"
)

type Channel struct {
	ShmQueueInfo *QueueInfo
	//URL of endpoint (might be useful in the future)
	BaseURL *url.URL
	//shm state info etc that might be needed
}

var _ grpc.ClientConnInterface = (*Channel)(nil)

var (
	cserReqData     [600]byte
	cserReqLen      int
	cserReqWritten  bool = false
	cserRespStruct  ShmMessage
	cserRespWritten bool = false

	cserPayload            []byte
	cserPayloadWritten     bool = false
	cserPayloadResp        grpchantesting.Message
	cserPayloadRespWritten bool = false
)

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
	if !cserPayloadWritten {
		serializedPayload, err := codec.Marshal(req)
		if err != nil {
			return err
		}
		cserPayload = serializedPayload
		cserPayloadWritten = true
	}

	messageRequest := &ShmMessage{
		Method:  methodName,
		Context: ctx,
		Headers: headersFromContext(ctx),
		Payload: ByteSlice2String(cserPayload),
	}

	// Create a fixed-length byte array
	// var byteArray [unsafe.Sizeof(messageRequest)]byte

	// Copy the bytes of the struct into the byte array
	// messageRequestBytes := *(*[unsafe.Sizeof(messageRequest)]byte)(unsafe.Poier(&messageRequest))
	// messageRequestBytes := fmt.Sprintf("%+v\n", messageRequest)
	// copy(byteArray[:], messageRequestBytes[:])

	// we have the meta request
	// Marshall to build rest of system
	var serializedMessage []byte
	var data [600]byte
	if !cserReqWritten {
		serializedMessage, err = json.Marshal(messageRequest)
		cserReqLen = copy(cserReqData[:], serializedMessage)
		data = cserReqData
		cserReqWritten = true
		if err != nil {
			return err
		}
	} else {
		data = cserReqData
	}

	//START MESSAGING
	requestQueue := GetQueue(ch.ShmQueueInfo.RequestShmaddr)
	responseQueue := GetQueue(ch.ShmQueueInfo.ResponseShmaddr)

	message := Message{
		Header: MessageHeader{Size: int32(cserReqLen)},
		Data:   data,
	}

	// pass into shared mem queue
	produceMessage(requestQueue, message)

	//Receive Request
	read_message, err := consumeMessage(responseQueue)
	if err != nil {
		//This should hopefully not happen
		return err
	}

	//Parse bytes into object
	slice := read_message.Data[0:read_message.Header.Size]
	var message_resp_meta ShmMessage
	if !cserRespWritten {
		json.Unmarshal(slice, &message_resp_meta)
		cserRespStruct = message_resp_meta
		cserRespWritten = true
		if err != nil {
			return err
		}
	} else {
		message_resp_meta = cserRespStruct
	}

	payload := unsafeGetBytes(message_resp_meta.Payload)

	copts.SetHeaders(message_resp_meta.Headers)
	copts.SetTrailers(message_resp_meta.Trailers)

	// ipc.Msgctl(qid, ipc.IPC_RMID)
	var ret_err error
	// if !cserPayloadRespWritten {
	// copy(cserPayloadResp, resp)
	ret_err = codec.Unmarshal(payload, resp)
	// cserPayloadRespWritten = true
	// }
	// resp = cserPayloadResp

	if !SERIALIZATION {
		cserReqWritten = false
		cserRespWritten = false
		cserPayloadWritten = false
		cserPayloadRespWritten = false
	}

	return ret_err
}

func (ch *Channel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}
