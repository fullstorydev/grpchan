package httpgrpc_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/httpgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// TestPeerTLSInfo checks that a call over TLS reports the connection's TLS
// state in its peer, for unary as well as streaming RPCs.
func TestPeerTLSInfo(t *testing.T) {
	svr := httpgrpc.NewServer()
	grpchantesting.RegisterTestServiceServer(svr, &grpchantesting.TestServer{})
	httpServer := httptest.NewTLSServer(svr)
	defer httpServer.Close()

	u, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	ch, err := httpgrpc.NewChannel(u, httpServer.Client().Transport)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	cli := grpchantesting.NewTestServiceClient(ch)

	checkPeer := func(t *testing.T, p *peer.Peer) {
		t.Helper()
		info, ok := p.AuthInfo.(credentials.TLSInfo)
		if !ok {
			t.Fatalf("peer's auth info: got %T, want credentials.TLSInfo", p.AuthInfo)
		}
		if len(info.State.PeerCertificates) == 0 {
			t.Fatal("peer's TLS state has no certificates")
		}
	}

	t.Run("unary", func(t *testing.T) {
		var p peer.Peer
		if _, err := cli.Unary(context.Background(), &grpchantesting.Message{}, grpc.Peer(&p)); err != nil {
			t.Fatalf("RPC failed: %v", err)
		}
		checkPeer(t, &p)
	})
	t.Run("stream", func(t *testing.T) {
		var p peer.Peer
		stream, err := cli.ServerStream(context.Background(), &grpchantesting.Message{}, grpc.Peer(&p))
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}
		if _, err := stream.Header(); err != nil {
			t.Fatalf("failed to receive headers: %v", err)
		}
		checkPeer(t, &p)
	})
}

// TestSendMsgMarshalErrorEndsStream checks that a message the client can't
// encode ends the RPC, rather than leaving both sides waiting for it.
func TestSendMsgMarshalErrorEndsStream(t *testing.T) {
	svr := httpgrpc.NewServer()
	grpchantesting.RegisterTestServiceServer(svr, &grpchantesting.TestServer{})
	httpServer := httptest.NewServer(svr)
	defer httpServer.Close()

	u, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	ch, err := httpgrpc.NewChannel(u, httpServer.Client().Transport)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	cli := grpchantesting.NewTestServiceClient(ch)

	// If the RPC isn't ended, it stays open until this deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := cli.BidiStream(ctx)
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}
	sendErr := stream.SendMsg("not a proto message")
	if sendErr == nil {
		t.Fatal("SendMsg of a message that can't be encoded succeeded")
	}
	if _, err := stream.Recv(); err == nil || err.Error() != sendErr.Error() {
		t.Fatalf("Recv: got %v, want the error from SendMsg: %v", err, sendErr)
	}
	if status.Code(sendErr) != codes.Internal {
		t.Fatalf("SendMsg: got %v, want an Internal error", sendErr)
	}
	if err := stream.Send(&grpchantesting.Message{}); err != io.EOF {
		t.Fatalf("Send after the stream ended: got %v, want io.EOF", err)
	}
}

// expiredContext reports a deadline that has passed, though the context hasn't
// ended: its timer hasn't fired yet.
type expiredContext struct {
	context.Context
}

func (expiredContext) Deadline() (time.Time, bool) {
	return time.Now().Add(-time.Second), true
}

// TestExpiredDeadlineFailsBeforeSending checks that a call whose deadline has
// already passed fails without sending a request that could only fail too.
func TestExpiredDeadlineFailsBeforeSending(t *testing.T) {
	svr := httpgrpc.NewServer()
	grpchantesting.RegisterTestServiceServer(svr, &grpchantesting.TestServer{})
	var requests atomic.Int32
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		svr.ServeHTTP(w, r)
	}))
	defer httpServer.Close()

	u, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	ch, err := httpgrpc.NewChannel(u, httpServer.Client().Transport)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	cli := grpchantesting.NewTestServiceClient(ch)

	ended, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	contexts := map[string]context.Context{
		"context ended":   ended,
		"deadline passed": expiredContext{context.Background()},
	}
	for name, ctx := range contexts {
		t.Run(name, func(t *testing.T) {
			_, err := cli.Unary(ctx, &grpchantesting.Message{})
			if status.Code(err) != codes.DeadlineExceeded {
				t.Errorf("unary: got %v, want a DeadlineExceeded error", err)
			}
			_, err = cli.ServerStream(ctx, &grpchantesting.Message{})
			if status.Code(err) != codes.DeadlineExceeded {
				t.Errorf("stream: got %v, want a DeadlineExceeded error", err)
			}
			if n := requests.Load(); n != 0 {
				t.Errorf("server got %d requests, want none", n)
			}
		})
	}
}
