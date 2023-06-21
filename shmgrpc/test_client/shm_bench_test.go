package shmgrpc_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/shmgrpc"
	"github.com/siadat/ipc"
)

func BenchmarkGrpcOverSharedMemory(b *testing.B) {

	qi := shmgrpc.QueueInfo{
		QueuePath:         "/Users/estebanramos/projects/microservices_work/app_testing/grpchan/shmgrpc/shm_test.go",
		QueueId:           42,
		QueueReqType:      2,
		QueueReqTypeMeta:  1,
		QueueRespType:     4,
		QueueRespTypeMeta: 3,
	}
	//Placeholder URL????
	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:8080"))
	if err != nil {
		b.Fatalf("failed to parse base URL: %v", err)
	}

	//Instantiate queue for processing
	key, err := ipc.Ftok(qi.QueuePath, qi.QueueId)
	if err != nil {
		panic(fmt.Sprintf("SERVER: Failed to generate key: %s\n", err))
	} else {
		// fmt.Printf("SERVER: Generate key %d\n", key)
	}

	qid, err := ipc.Msgget(key, ipc.IPC_CREAT|0666)
	if err != nil {
		panic(fmt.Sprintf("SERVER: Failed to create ipc key %d: %s\n", key, err))
	} else {
		// fmt.Printf("SERVER: Create ipc queue id %d\n", qid)
	}

	qi.Qid = qid

	// Construct Channel with necessary parameters to talk to the Server
	cc := shmgrpc.Channel{
		BaseURL:      u,
		ShmQueueInfo: &qi,
	}

	// grpchantesting.RunChannelTestCases(t, &cc, true)
	grpchantesting.RunChannelBenchmarkCases(b, &cc, false)

}
