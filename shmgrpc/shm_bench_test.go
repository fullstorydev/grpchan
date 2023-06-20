package shmgrpc_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/shmgrpc"
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

	// svr := &grpchantesting.TestServer{}
	svc := &grpchantesting.TestServer{}
	svr := shmgrpc.NewServer(&qi, "/")

	//Register Server and instantiate with necessary information
	//Server can create queue
	//Server Can have
	go grpchantesting.RegisterTestServiceServer(svr, svc)

	//Placeholder URL????
	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:8080"))
	if err != nil {
		b.Fatalf("failed to parse base URL: %v", err)
	}

	// Construct Channel with necessary parameters to talk to the Server
	cc := shmgrpc.Channel{
		BaseURL:      u,
		ShmQueueInfo: &qi,
	}

	// grpchantesting.RunChannelTestCases(t, &cc, true)
	grpchantesting.RunChannelBenchmarkCases(b, &cc, false)

}
