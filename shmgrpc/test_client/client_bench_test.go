package shmgrpc_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/shmgrpc"
)

func BenchmarkGrpcOverSharedMemory(b *testing.B) {

	requestShmid, requestShmaddr := shmgrpc.InitializeShmRegion(shmgrpc.RequestKey, shmgrpc.Size, uintptr(shmgrpc.ClientSegFlag))
	responseShmid, responseShmaddr := shmgrpc.InitializeShmRegion(shmgrpc.ResponseKey, shmgrpc.Size, uintptr(shmgrpc.ClientSegFlag))

	qi := shmgrpc.QueueInfo{
		RequestShmid:    requestShmid,
		RequestShmaddr:  requestShmaddr,
		ResponseShmid:   responseShmid,
		ResponseShmaddr: responseShmaddr,
	}
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

	defer shmgrpc.StopPollingQueue(shmgrpc.GetQueue(requestShmaddr))
	defer shmgrpc.StopPollingQueue(shmgrpc.GetQueue(responseShmaddr))

	defer shmgrpc.Detach(requestShmaddr)
	defer shmgrpc.Detach(responseShmaddr)

}
