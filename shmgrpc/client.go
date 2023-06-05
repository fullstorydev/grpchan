package shmgrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/fullstorydev/grpchan/internal"

	"google.golang.org/grpc"

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
	copts := internal.GetCallOptions(opts)

	reqUrl := *ch.BaseURL
	reqUrl.Path = path.Join(reqUrl.Path, methodName)
	reqUrlStr := reqUrl.String()

	ctx, err := internal.ApplyPerRPCCreds(ctx, copts, fmt.Sprintf("shm:0%s", reqUrlStr), true)

	if err != nil {
		return err
	}

	key, err := ipc.Ftok(ch.ShmQueueInfo.QueuePath, ch.ShmQueueInfo.QueueId)
	if err != nil {
		panic(err)
	}

	qid, err := ipc.Msgget(key, ipc.IPC_CREAT|0666)
	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to create ipc key %d: %s\n", key, err))
	} else {
		fmt.Printf("CLIENT: Create ipc queue id %d\n", qid)
	}

	message_request := &ShmMessage{
		Method:  methodName,
		Payload: req,
	}

	// we have the request
	// Marshall it temporarily to build rest of system
	serialized_message, err := json.Marshal(message_request)
	if err != nil {
		return err
	}

	// pass into shared mem queue
	msg_req := &ipc.Msgbuf{
		Mtype: ch.ShmQueueInfo.QueueReqType, //Request message type
		Mtext: serialized_message,           // Isnt this technically serialization?
	}

	err = ipc.Msgsnd(qid, msg_req, 0)
	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to send message to ipc id %d: %s\n", qid, err))
	} else {
		fmt.Printf("CLIENT: Message %v send to ipc id %d\n", msg_req, qid)
	}

	//Construct receive buffer (this will have to change)
	msg_rep := &ipc.Msgbuf{
		Mtype: ch.ShmQueueInfo.QueueRespType}

	time.Sleep(5 * time.Second)

	err = ipc.Msgrcv(qid, msg_rep, 0)

	if err != nil {
		panic(fmt.Sprintf("CLIENT: Failed to receive message to ipc id %d: %s\n", qid, err))
	} else {
		fmt.Printf("CLIENT: Message %v receive to ipc id %d\n", msg_rep.Mtext, qid)
	}
	// ipc.Msgctl(qid, ipc.IPC_RMID)

	return nil
}

func (ch *Channel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}
