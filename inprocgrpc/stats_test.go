package inprocgrpc_test

import (
	"context"
	"sync"
	"testing"

	"google.golang.org/grpc/stats"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
)

// testStatsHandler is a simple stats handler that records events
type testStatsHandler struct {
	mu     sync.Mutex
	events []stats.RPCStats
}

func (h *testStatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	return ctx
}

func (h *testStatsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, s)
}

func (h *testStatsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

func (h *testStatsHandler) HandleConn(ctx context.Context, s stats.ConnStats) {
	// Not used for in-process channels
}

func (h *testStatsHandler) getEvents() []stats.RPCStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]stats.RPCStats, len(h.events))
	copy(result, h.events)
	return result
}

func TestStatsHandlerUnary(t *testing.T) {
	handler := &testStatsHandler{}

	channel := &inprocgrpc.Channel{}
	channel = channel.WithClientStatsHandler(handler)
	channel = channel.WithServerStatsHandler(handler)

	svc := &grpchantesting.TestServer{}
	grpchantesting.RegisterTestServiceServer(channel, svc)

	client := grpchantesting.NewTestServiceClient(channel)

	// Make a unary call
	req := &grpchantesting.Message{Payload: []byte("hello")}
	resp, err := client.Unary(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Payload) != "hello" {
		t.Errorf("expected payload 'hello', got '%s'", string(resp.Payload))
	}

	// Check that stats events were recorded
	events := handler.getEvents()
	if len(events) == 0 {
		t.Fatal("expected stats events, got none")
	}

	// Verify we got Begin and End events for both client and server
	var clientBegin, clientEnd, serverBegin, serverEnd bool
	for _, e := range events {
		switch v := e.(type) {
		case *stats.Begin:
			if v.Client {
				clientBegin = true
			} else {
				serverBegin = true
			}
		case *stats.End:
			if v.Client {
				clientEnd = true
			} else {
				serverEnd = true
			}
		}
	}

	if !clientBegin {
		t.Error("missing client Begin event")
	}
	if !clientEnd {
		t.Error("missing client End event")
	}
	if !serverBegin {
		t.Error("missing server Begin event")
	}
	if !serverEnd {
		t.Error("missing server End event")
	}
}

func TestStatsHandlerStreaming(t *testing.T) {
	handler := &testStatsHandler{}

	channel := &inprocgrpc.Channel{}
	channel = channel.WithClientStatsHandler(handler)
	channel = channel.WithServerStatsHandler(handler)

	svc := &grpchantesting.TestServer{}
	grpchantesting.RegisterTestServiceServer(channel, svc)

	client := grpchantesting.NewTestServiceClient(channel)

	// Make a client streaming call
	stream, err := client.ClientStream(context.Background())
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	// Send some messages
	for i := 0; i < 3; i++ {
		if err := stream.Send(&grpchantesting.Message{Payload: []byte("msg")}); err != nil {
			t.Fatalf("unexpected error sending: %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("unexpected error closing: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Check that stats events were recorded
	events := handler.getEvents()
	if len(events) == 0 {
		t.Fatal("expected stats events, got none")
	}

	// Verify we got Begin and End events
	var clientBegin, clientEnd, serverBegin, serverEnd bool
	var outPayloadCount, inPayloadCount int
	for _, e := range events {
		switch v := e.(type) {
		case *stats.Begin:
			if v.Client {
				clientBegin = true
				if !v.IsClientStream {
					t.Error("expected IsClientStream to be true")
				}
			} else {
				serverBegin = true
			}
		case *stats.End:
			if v.Client {
				clientEnd = true
			} else {
				serverEnd = true
			}
		case *stats.OutPayload:
			outPayloadCount++
		case *stats.InPayload:
			inPayloadCount++
		}
	}

	if !clientBegin {
		t.Error("missing client Begin event")
	}
	if !clientEnd {
		t.Error("missing client End event")
	}
	if !serverBegin {
		t.Error("missing server Begin event")
	}
	if !serverEnd {
		t.Error("missing server End event")
	}
	if outPayloadCount < 3 {
		t.Errorf("expected at least 3 OutPayload events, got %d", outPayloadCount)
	}
	if inPayloadCount < 1 {
		t.Errorf("expected at least 1 InPayload event, got %d", inPayloadCount)
	}
}

func TestStatsHandlerError(t *testing.T) {
	handler := &testStatsHandler{}

	channel := &inprocgrpc.Channel{}
	channel = channel.WithClientStatsHandler(handler)
	channel = channel.WithServerStatsHandler(handler)

	svc := &grpchantesting.TestServer{}
	grpchantesting.RegisterTestServiceServer(channel, svc)

	client := grpchantesting.NewTestServiceClient(channel)

	// Make a call that will fail (code 2 = UNKNOWN)
	req := &grpchantesting.Message{Code: 2}
	_, err := client.Unary(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check that End events contain the error
	events := handler.getEvents()
	var foundErrorInEnd bool
	for _, e := range events {
		if end, ok := e.(*stats.End); ok {
			if end.Error != nil {
				foundErrorInEnd = true
				break
			}
		}
	}

	if !foundErrorInEnd {
		t.Error("expected End event to contain error")
	}
}
