package grpchantesting

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/struct"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fullstorydev/grpchan"
)

// RunChannelTestCases runs numerous test cases to exercise the behavior of the
// given channel. The server side of the channel needs to have a *TestServer (in
// this package) registered to provide the implementation of fsgrpc.TestService
// (proto in this package). If the channel does not support full-duplex
// communication, it must provide at least half-duplex support for bidirectional
// streams.
//
// The test cases will be defined as child tests by invoking t.Run on the given
// *testing.T.
func RunChannelTestCases(t *testing.T, ch grpchan.Channel, supportsFullDuplex bool) {
	cli := NewTestServiceChannelClient(ch)
	t.Run("unary", func(t *testing.T) { testUnary(t, cli) })
	t.Run("client-stream", func(t *testing.T) { testClientStream(t, cli) })
	t.Run("server-stream", func(t *testing.T) { testServerStream(t, cli) })
	t.Run("half-duplex bidi-stream", func(t *testing.T) { testHalfDuplexBidiStream(t, cli) })
	if supportsFullDuplex {
		t.Run("full-duplex bidi-stream", func(t *testing.T) { testFullDuplexBidiStream(t, cli) })
	}
}

var (
	testPayload = []byte{100, 90, 80, 70, 60, 50, 40, 30, 20, 10, 0}

	testOutgoingMd = map[string]string{
		"foo":        "bar",
		"baz":        "bedazzle",
		"pickle-bin": string(testPayload),
	}

	testMdHeaders = map[string]string{
		"foo1":        "bar4",
		"baz2":        "bedazzle5",
		"pickle3-bin": string(testPayload),
	}

	testMdTrailers = map[string]string{
		"4foo4":        "7bar7",
		"5baz5":        "8bedazzle8",
		"6pickle6-bin": string(testPayload),
	}

	testErrorMessages = []proto.Message{
		&structpb.ListValue{
			Values: []*structpb.Value{
				{Kind: &structpb.Value_NumberValue{NumberValue: 123}},
				{Kind: &structpb.Value_StringValue{StringValue: "foo"}},
			},
		},
		&structpb.Struct{
			Fields: map[string]*structpb.Value{
				"FOO": {Kind: &structpb.Value_NumberValue{NumberValue: 456}},
				"BAR": {Kind: &structpb.Value_StringValue{StringValue: "bar"}},
			},
		},
	}
	testErrorDetails []*any.Any
)

func init() {
	for _, msg := range testErrorMessages {
		if a, err := ptypes.MarshalAny(msg); err != nil {
			panic(err)
		} else {
			testErrorDetails = append(testErrorDetails, a)
		}
	}
}

func testUnary(t *testing.T, cli TestServiceClient) {
	// NB(jh): implementations of channel.Channel currently can't support
	// response headers and trailers for unary RPCs due to roadblocks in
	// the underlying gRPC APIs:
	//  * https://github.com/grpc/grpc-go/issues/1494
	//  * https://github.com/grpc/grpc-go/issues/1495
	//  * https://github.com/grpc/grpc-go/issues/1802

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(testOutgoingMd))

	t.Run("success", func(t *testing.T) {
		rsp, err := cli.Unary(ctx, &Message{
			Payload: testPayload,
		})
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}
		if !bytes.Equal(testPayload, rsp.Payload) {
			t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, rsp.Payload)
		}
		checkHeaders(t, testOutgoingMd, rsp.Headers)
	})

	t.Run("failure", func(t *testing.T) {
		_, err := cli.Unary(ctx, &Message{
			Code:         int32(codes.AlreadyExists),
			ErrorDetails: testErrorDetails,
		})
		checkError(t, err, codes.AlreadyExists, testErrorMessages...)
	})

	t.Run("timeout", func(t *testing.T) {
		tctx, _ := context.WithTimeout(ctx, 100*time.Millisecond)
		_, err := cli.Unary(tctx, &Message{
			DelayMillis: 500,
		})
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		_, err := cli.Unary(cctx, &Message{
			DelayMillis: 500,
		})
		checkError(t, err, codes.Canceled)
	})
}

func testClientStream(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(testOutgoingMd))
	reqMsg := &Message{
		Payload:  testPayload,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	t.Run("success", func(t *testing.T) {
		cs, err := cli.ClientStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		err = cs.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}

		err = cs.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		err = cs.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #3 failed: %v", err)
		}

		m, err := cs.CloseAndRecv()
		if err != nil {
			t.Fatalf("receiving message failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
		}
		if m.Count != 3 {
			t.Fatalf("wrong count returned: expecting %d; got %d", 3, m.Count)
		}
		checkHeaders(t, testOutgoingMd, m.Headers)

		h, err := cs.Header()
		if err != nil {
			t.Fatalf("failed to get header metadata: %v", err)
		}
		checkMdHeaders(t, testMdHeaders, h)
		checkMdHeaders(t, testMdTrailers, cs.Trailer())
	})

	t.Run("failure", func(t *testing.T) {
		cs, err := cli.ClientStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		reqMsg.Code = int32(codes.ResourceExhausted)
		reqMsg.ErrorDetails = testErrorDetails
		err = cs.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		_, err = cs.CloseAndRecv()
		checkError(t, err, codes.ResourceExhausted, testErrorMessages...)
	})

	t.Run("timeout", func(t *testing.T) {
		tctx, _ := context.WithTimeout(ctx, 100*time.Millisecond)
		cs, err := cli.ClientStream(tctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		err = cs.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		_, err = cs.CloseAndRecv()
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		cs, err := cli.ClientStream(cctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		err = cs.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		_, err = cs.CloseAndRecv()
		checkError(t, err, codes.Canceled)
	})
}

func testServerStream(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(testOutgoingMd))
	reqMsg := &Message{
		Payload:  testPayload,
		Count:    5,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	t.Run("success", func(t *testing.T) {
		ss, err := cli.ServerStream(ctx, reqMsg)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		h, err := ss.Header()
		if err != nil {
			t.Fatalf("failed to get header metadata: %v", err)
		}
		checkMdHeaders(t, testMdHeaders, h)

		for i := 0; i < 5; i++ {
			m, err := ss.Recv()
			if err != nil {
				t.Fatalf("receiving message #%d failed: %v", i+1, err)
			}
			if !bytes.Equal(testPayload, m.Payload) {
				t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
			}
			checkHeaders(t, testOutgoingMd, m.Headers)
		}
		_, err = ss.Recv()
		if err != io.EOF {
			t.Fatalf("expected EOF; got %v", err)
		}

		checkMdHeaders(t, testMdTrailers, ss.Trailer())
	})

	t.Run("failure", func(t *testing.T) {
		reqMsg.Count = 2
		reqMsg.Code = int32(codes.FailedPrecondition)
		reqMsg.ErrorDetails = testErrorDetails
		ss, err := cli.ServerStream(ctx, reqMsg)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		for i := 0; i < 2; i++ {
			m, err := ss.Recv()
			if err != nil {
				t.Fatalf("receiving message #%d failed: %v", i+1, err)
			}
			if !bytes.Equal(testPayload, m.Payload) {
				t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
			}
			checkHeaders(t, testOutgoingMd, m.Headers)
		}
		_, err = ss.Recv()
		checkError(t, err, codes.FailedPrecondition, testErrorMessages...)
	})

	t.Run("timeout", func(t *testing.T) {
		tctx, _ := context.WithTimeout(ctx, 100*time.Millisecond)

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		ss, err := cli.ServerStream(tctx, reqMsg)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		_, err = ss.Recv()
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		ss, err := cli.ServerStream(cctx, reqMsg)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		_, err = ss.Recv()
		checkError(t, err, codes.Canceled)
	})
}

func testHalfDuplexBidiStream(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(testOutgoingMd))
	reqMsg := &Message{
		Payload: testPayload,
		Count:   -1, // enables half-duplex mode in server
		Headers: testMdHeaders,
	}

	t.Run("success", func(t *testing.T) {
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}
		reqMsg.Headers = nil

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		reqMsg.Trailers = testMdTrailers
		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #3 failed: %v", err)
		}
		reqMsg.Trailers = nil

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		md, err := bidi.Header()
		if err != nil {
			t.Fatalf("failed to get header metadata: %v", err)
		}
		checkMdHeaders(t, testMdHeaders, md)

		m, err := bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message #1 failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload in message #1: expecting %v; got %v", testPayload, m.Payload)
		}
		checkHeaders(t, testOutgoingMd, m.Headers)

		m, err = bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message #2 failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload in message #2: expecting %v; got %v", testPayload, m.Payload)
		}

		m, err = bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message #3 failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload in message #3: expecting %v; got %v", testPayload, m.Payload)
		}

		_, err = bidi.Recv()
		if err != io.EOF {
			t.Fatalf("expected EOF; got %v", err)
		}

		md = bidi.Trailer()
		checkMdHeaders(t, testMdTrailers, md)
	})

	t.Run("failure", func(t *testing.T) {
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}

		reqMsg.Code = int32(codes.DataLoss)
		reqMsg.ErrorDetails = testErrorDetails
		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		m, err := bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.DataLoss, testErrorMessages...)
	})

	t.Run("timeout", func(t *testing.T) {
		tctx, _ := context.WithTimeout(ctx, 100*time.Millisecond)
		bidi, err := cli.BidiStream(tctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		bidi, err := cli.BidiStream(cctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.Canceled)
	})
}

func testFullDuplexBidiStream(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(testOutgoingMd))
	reqMsg := &Message{
		Payload:  testPayload,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	t.Run("success", func(t *testing.T) {
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		for i := 0; i < 3; i++ {
			err = bidi.Send(reqMsg)
			if err != nil {
				t.Fatalf("sending message #%d failed: %v", i+1, err)
			}

			if i == 0 {
				md, err := bidi.Header()
				if err != nil {
					t.Fatalf("failed to get header metadata: %v", err)
				}
				checkMdHeaders(t, testMdHeaders, md)
			}

			m, err := bidi.Recv()
			if err != nil {
				t.Fatalf("receiving message #%d failed: %v", i+1, err)
			}
			if !bytes.Equal(testPayload, m.Payload) {
				t.Fatalf("wrong payload in message #%d: expecting %v; got %v", i+1, testPayload, m.Payload)
			}
			checkHeaders(t, testOutgoingMd, m.Headers)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		if err != io.EOF {
			t.Fatalf("expected EOF; got %v", err)
		}

		md := bidi.Trailer()
		checkMdHeaders(t, testMdTrailers, md)
	})

	t.Run("failure", func(t *testing.T) {
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}

		m, err := bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
		}

		reqMsg.Code = int32(codes.DataLoss)
		reqMsg.ErrorDetails = testErrorDetails
		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.DataLoss, testErrorMessages...)
	})

	t.Run("timeout", func(t *testing.T) {
		tctx, _ := context.WithTimeout(ctx, 100*time.Millisecond)
		bidi, err := cli.BidiStream(tctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		bidi, err := cli.BidiStream(cctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		reqMsg.Code = int32(codes.OK)
		reqMsg.DelayMillis = 500

		err = bidi.Send(reqMsg)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.Canceled)
	})
}

func checkHeaders(t *testing.T, expected, actual map[string]string) {
	// we don't just do a strict equals check because the actual headers
	// echoed back could have extra headers that were added implicitly
	// by the GRPC-over-HTTP client (such as GRPC-Timeout, Content-Type, etc).
	for k, v := range expected {
		v2, ok := actual[k]
		if !ok || v2 != v {
			t.Fatalf("wrong headers echoed back: expecting header %s to be %q, instead was %q", k, v, v2)
		}
	}
}

func checkMdHeaders(t *testing.T, expected map[string]string, actual metadata.MD) {
	// we don't just do a strict equals check because the actual headers
	// echoed back could have extra headers that were added implicitly
	// by the GRPC-over-HTTP client (such as GRPC-Timeout, Content-Type, etc).
	for k, v := range expected {
		v2, ok := actual[k]
		if !ok || len(v2) != 1 || v2[0] != v {
			t.Fatalf("wrong headers echoed back: expecting header %s to be [%s], instead was %v", k, v, v2)
		}
	}
}

func checkError(t *testing.T, err error, expectedCode codes.Code, expectedDetails ...proto.Message) {
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("wrong type of error")
	}
	if st.Code() != expectedCode {
		t.Fatalf("wrong response code: %v != %v", st.Code(), expectedCode)
	}
	actualDetails := st.Details()
	if len(actualDetails) != len(expectedDetails) {
		t.Fatalf("wrong number of error details: %v != %v", len(actualDetails), len(expectedDetails))
	}
	for i, msg := range actualDetails {
		if !proto.Equal(msg.(proto.Message), expectedDetails[i]) {
			t.Fatalf("wrong error detail message at index %d: %v != %v", i, msg, expectedDetails[i])
		}
	}
}
