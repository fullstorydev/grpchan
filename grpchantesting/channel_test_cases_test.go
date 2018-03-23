package grpchantesting

import (
	"net"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// We test all of our channel test cases by running them against a normal
// *grpc.Server and *grpc.ClientConn, to make sure they are asserting the
// same behavior exhibited by the standard HTTP/2 channel implementation.
func TestChannelTestCases(t *testing.T) {
	s := grpc.NewServer()
	RegisterTestServiceServer(s, &TestServer{})

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on socket: %v", err)
	}
	go s.Serve(l)
	defer s.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addr := l.Addr().String()
	cc, err := grpc.DialContext(ctx, addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.FailOnNonTempDialError(true))
	if err != nil {
		t.Fatalf("failed to dial address: %s", addr)
	}
	defer cc.Close()

	RunChannelTestCases(t, cc, true)
}
