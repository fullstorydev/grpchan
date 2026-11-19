package httpgrpc_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/synctest"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/httpgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// TestTimeoutHeader covers handling of GRPC-Timeout values.
func TestTimeoutHeader(t *testing.T) {
	type observed struct {
		called      bool
		hasDeadline bool
		remaining   time.Duration
	}
	var got observed
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		deadline, ok := ctx.Deadline()
		got = observed{called: true, hasDeadline: ok, remaining: time.Until(deadline)}
		return handler(ctx, req)
	}
	svr := httpgrpc.NewServer(httpgrpc.WithServerUnaryInterceptor(interceptor))
	grpchantesting.RegisterTestServiceServer(svr, &grpchantesting.TestServer{})

	body, err := proto.Marshal(&grpchantesting.Message{})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	testCases := []struct {
		name     string
		timeout  string
		duration time.Duration
		// If non-zero, the call fails with this HTTP status, which the client
		// reads as a gRPC code, without calling the handler.
		failStatus int
	}{
		{name: "valid", timeout: "30S", duration: 30 * time.Second},
		// More digits than the format allows, as older clients sent.
		{name: "many digits", timeout: "123456789m", duration: 123456789 * time.Millisecond},
		// DeadlineExceeded
		{name: "zero", timeout: "0S", failStatus: http.StatusGatewayTimeout},
		// InvalidArgument
		{name: "negative", timeout: "-5S", failStatus: http.StatusBadRequest},
		{name: "unknown unit", timeout: "5X", failStatus: http.StatusBadRequest},
		{name: "no digits", timeout: "S", failStatus: http.StatusBadRequest},
		{name: "not a number", timeout: "abcS", failStatus: http.StatusBadRequest},
		// Too long to represent, so this should not result in a deadline.
		{name: "overflow", timeout: "99999999H", duration: -1},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// In a synctest bubble, the handler will see the deadline exactly
			// as set (no virtual time passes between the context deadline
			// being set and the interceptor above observing it). Calling
			// ServeHTTP directly, rather than over a real connection, keeps
			// the whole call in the synctest bubble.
			synctest.Test(t, func(t *testing.T) {
				// Reset so we don't inadvertently observe prior case's capture:
				got.called = false
				req := httptest.NewRequest(http.MethodPost, "/grpchantesting.TestService/Unary", bytes.NewReader(body))
				req.Header.Set("Content-Type", httpgrpc.UnaryRpcContentType_V1)
				req.Header.Set("GRPC-Timeout", testCase.timeout)
				rec := httptest.NewRecorder()
				svr.ServeHTTP(rec, req)

				if testCase.failStatus != 0 {
					if got.called {
						t.Fatal("handler was called, want the call to fail first")
					}
					if rec.Code != testCase.failStatus {
						t.Fatalf("HTTP status: got %d, want %d", rec.Code, testCase.failStatus)
					}
					return
				}
				if !got.called {
					t.Fatalf("handler was not called: HTTP status %d", rec.Code)
				}
				switch wantDeadline := testCase.duration >= 0; {
				case wantDeadline && !got.hasDeadline:
					t.Fatalf("handler's context has no deadline: want %v", testCase.duration)
				case got.hasDeadline && !wantDeadline:
					t.Fatalf("handler's context has unexpected deadline: got %v", got.remaining)
				case wantDeadline && got.remaining != testCase.duration:
					t.Fatalf("handler's deadline is %v away, want %v", got.remaining, testCase.duration)
				}
			})
		})
	}
}

// TestMetadataCannotSetProtocolHeaders checks that metadata can't masquerade as
// the headers that HTTP or httpgrpc itself uses, in either direction. Sent as is,
// these could change/corrupt how the peer reads the call.
func TestMetadataCannotSetProtocolHeaders(t *testing.T) {
	svr := httpgrpc.NewServer()
	grpchantesting.RegisterTestServiceServer(svr, &grpchantesting.TestServer{})
	httpServer := httptest.NewServer(svr)
	defer httpServer.Close()

	u, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	ch, err := httpgrpc.NewChannel(u, http.DefaultTransport)
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	cli := grpchantesting.NewTestServiceClient(ch)

	reserved := map[string][]byte{
		"grpc-timeout":       []byte("1n"),
		"x-grpc-status":      []byte("5:not found"),
		"x-grpc-trailer-foo": []byte("bar"),
		"content-encoding":   []byte("gzip"),
	}
	checkFiltered := func(t *testing.T, what string, got map[string]bool) {
		t.Helper()
		for k := range reserved {
			if got[k] {
				t.Errorf("%s includes %q", what, k)
			}
		}
		if !got["custom"] {
			t.Errorf("%s is missing ordinary metadata", what)
		}
	}

	t.Run("request", func(t *testing.T) {
		md := grpchantesting.MetadataNew(reserved)
		md.Set("custom", "value")
		ctx := metadata.NewOutgoingContext(context.Background(), md)
		resp, err := cli.Unary(ctx, &grpchantesting.Message{})
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}
		// The server echoes the metadata it received.
		got := map[string]bool{}
		for k := range resp.Headers {
			got[k] = true
		}
		checkFiltered(t, "server's metadata", got)
	})
	t.Run("response", func(t *testing.T) {
		headers := map[string][]byte{"custom": []byte("value")}
		for k, v := range reserved {
			headers[k] = v
		}
		var hdr, tlr metadata.MD
		_, err := cli.Unary(context.Background(), &grpchantesting.Message{Headers: headers}, grpc.Header(&hdr), grpc.Trailer(&tlr))
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}
		got := map[string]bool{}
		for k := range hdr {
			got[k] = true
		}
		checkFiltered(t, "client's headers", got)
		if len(tlr) != 0 {
			t.Errorf("client's trailers: got %v, want none", tlr)
		}
	})
}
