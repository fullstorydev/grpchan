package inprocgrpc_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
)

func BenchmarkInProcessChannel(b *testing.B) {
	svr := &grpchantesting.TestServer{}

	var cc inprocgrpc.Channel
	grpchantesting.RegisterTestServiceServer(&cc, svr)

	before := runtime.NumGoroutine()

	grpchantesting.RunChannelBenchmarkCases(b, &cc, true)

	// check for goroutine leaks
	deadline := time.Now().Add(time.Second * 5)
	after := 0
	for deadline.After(time.Now()) {
		after = runtime.NumGoroutine()
		if after <= before {
			// number of goroutines returned to previous level: no leak!
			return
		}
		time.Sleep(time.Millisecond * 50)
	}
	b.Errorf("%d goroutines leaked", after-before)
}
