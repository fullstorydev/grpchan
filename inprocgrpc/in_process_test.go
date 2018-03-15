package inprocgrpc_test

import (
	"testing"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
)

func TestInProcessChannel(t *testing.T) {
	svr := &grpchantesting.TestServer{}

	var cc inprocgrpc.Channel
	grpchantesting.RegisterHandlerTestService(&cc, svr)

	grpchantesting.RunChannelTestCases(t, &cc, true)
}
