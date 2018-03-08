package inprocgrpc_test

import (
	"testing"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
)

func TestInProcessChannel(t *testing.T) {
	svr := &grpchantesting.TestServer{}

	var ch inprocgrpc.Channel
	grpchantesting.RegisterTestServiceHandler(&ch, svr)

	grpchantesting.RunChannelTestCases(t, &ch, true)
}
