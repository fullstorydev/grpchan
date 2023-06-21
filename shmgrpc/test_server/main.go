package main

import (
	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/shmgrpc"
)

func main() {

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
	grpchantesting.RegisterTestServiceServer(svr, svc)

}
