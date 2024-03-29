package grpchantesting

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
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
func RunChannelTestCases(t *testing.T, ch grpc.ClientConnInterface, supportsFullDuplex bool) {
	cli := NewTestServiceClient(ch)
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

	testOutgoingMd = map[string][]byte{
		"foo":        []byte("bar"),
		"baz":        []byte("bedazzle"),
		"pickle-bin": testPayload,
	}

	testMdHeaders = map[string][]byte{
		"foo1":        []byte("bar4"),
		"baz2":        []byte("bedazzle5"),
		"pickle3-bin": testPayload,
	}

	testMdTrailers = map[string][]byte{
		"4foo4":        []byte("7bar7"),
		"5baz5":        []byte("8bedazzle8"),
		"6pickle6-bin": testPayload,
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
	testErrorDetails []*anypb.Any
)

func init() {
	for _, msg := range testErrorMessages {
		var a anypb.Any
		if err := anypb.MarshalFrom(&a, msg, proto.MarshalOptions{}); err != nil {
			panic(err)
		} else {
			testErrorDetails = append(testErrorDetails, &a)
		}
	}
}

func MetadataNew(m map[string][]byte) metadata.MD {
	md := metadata.MD{}
	for k, val := range m {
		key := strings.ToLower(k)
		md[key] = append(md[key], string(val))
	}
	return md
}

func testUnary(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), MetadataNew(testOutgoingMd))
	reqPrototype := Message{
		Payload:  testPayload,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}
	t.Run("success", func(t *testing.T) {
		var hdr, tlr metadata.MD
		req := proto.Clone(&reqPrototype).(*Message)
		rsp, err := cli.Unary(ctx, req, grpc.Header(&hdr), grpc.Trailer(&tlr))
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}
		if !bytes.Equal(testPayload, rsp.Payload) {
			t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, rsp.Payload)
		}
		checkRequestHeaders(t, testOutgoingMd, rsp.Headers)

		checkMetadata(t, testMdHeaders, hdr, "header")
		checkMetadata(t, testMdTrailers, tlr, "trailer")
	})

	t.Run("failure", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		req.Code = int32(codes.AlreadyExists)
		req.ErrorDetails = testErrorDetails
		_, err := cli.Unary(ctx, req)
		checkError(t, err, codes.AlreadyExists, testErrorMessages...)
	})

	t.Run("timeout", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		req.DelayMillis = 500
		tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		_, err := cli.Unary(tctx, req)
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		req.DelayMillis = 500
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		_, err := cli.Unary(cctx, req)
		checkError(t, err, codes.Canceled)
	})
}

func testClientStream(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), MetadataNew(testOutgoingMd))
	reqPrototype := Message{
		Payload:  testPayload,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	t.Run("success", func(t *testing.T) {
		cs, err := cli.ClientStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}
		req := proto.Clone(&reqPrototype).(*Message)
		err = cs.Send(req)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}

		err = cs.Send(req)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		err = cs.Send(req)
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
		checkRequestHeaders(t, testOutgoingMd, m.Headers)

		checkResponseMetadata(t, cs, testMdHeaders, testMdTrailers)
	})

	t.Run("failure", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		cs, err := cli.ClientStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		req.Code = int32(codes.ResourceExhausted)
		req.ErrorDetails = testErrorDetails
		err = cs.Send(req)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		_, err = cs.CloseAndRecv()
		checkError(t, err, codes.ResourceExhausted, testErrorMessages...)

		checkResponseMetadata(t, cs, testMdHeaders, testMdTrailers)
	})

	t.Run("timeout", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		cs, err := cli.ClientStream(tctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		req.Code = int32(codes.OK)
		req.DelayMillis = 500

		err = cs.Send(req)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		_, err = cs.CloseAndRecv()
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		cs, err := cli.ClientStream(cctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		req.Code = int32(codes.OK)
		req.DelayMillis = 500

		err = cs.Send(req)
		if err != nil {
			t.Fatalf("sending message failed: %v", err)
		}

		_, err = cs.CloseAndRecv()
		checkError(t, err, codes.Canceled)
	})
}

func testServerStream(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), MetadataNew(testOutgoingMd))
	reqPrototype := Message{
		Payload:  testPayload,
		Count:    5,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	t.Run("success", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		ss, err := cli.ServerStream(ctx, req)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		checkResponseHeaders(t, ss, testMdHeaders)

		for i := 0; i < 5; i++ {
			m, err := ss.Recv()
			if err != nil {
				t.Fatalf("receiving message #%d failed: %v", i+1, err)
			}
			if !bytes.Equal(testPayload, m.Payload) {
				t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
			}
			checkRequestHeaders(t, testOutgoingMd, m.Headers)
		}
		_, err = ss.Recv()
		if err != io.EOF {
			t.Fatalf("expected EOF; got %v", err)
		}

		checkResponseTrailers(t, ss, testMdTrailers)
	})

	t.Run("failure", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		req.Count = 2
		req.Code = int32(codes.FailedPrecondition)
		req.ErrorDetails = testErrorDetails
		ss, err := cli.ServerStream(ctx, req)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		checkResponseHeaders(t, ss, testMdHeaders)

		for i := 0; i < 2; i++ {
			m, err := ss.Recv()
			if err != nil {
				t.Fatalf("receiving message #%d failed: %v", i+1, err)
			}
			if !bytes.Equal(testPayload, m.Payload) {
				t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
			}
			checkRequestHeaders(t, testOutgoingMd, m.Headers)
		}
		_, err = ss.Recv()
		checkError(t, err, codes.FailedPrecondition, testErrorMessages...)

		checkResponseTrailers(t, ss, testMdTrailers)
	})

	t.Run("timeout", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		req.Code = int32(codes.OK)
		req.DelayMillis = 500

		ss, err := cli.ServerStream(tctx, req)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		_, err = ss.Recv()
		checkError(t, err, codes.DeadlineExceeded)
	})

	t.Run("canceled", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		req.Code = int32(codes.OK)
		req.DelayMillis = 500

		ss, err := cli.ServerStream(cctx, req)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		_, err = ss.Recv()
		checkError(t, err, codes.Canceled)
	})
}

func testHalfDuplexBidiStream(t *testing.T, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), MetadataNew(testOutgoingMd))
	reqPrototype := Message{
		Payload:  testPayload,
		Count:    -1, // enables half-duplex mode in server
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	t.Run("success", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		err = bidi.Send(req)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}
		req.Headers = nil

		err = bidi.Send(req)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		req.Trailers = testMdTrailers
		err = bidi.Send(req)
		if err != nil {
			t.Fatalf("sending message #3 failed: %v", err)
		}
		req.Trailers = nil

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		checkResponseHeaders(t, bidi, testMdHeaders)

		m, err := bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message #1 failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload in message #1: expecting %v; got %v", testPayload, m.Payload)
		}
		checkRequestHeaders(t, testOutgoingMd, m.Headers)

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

		checkResponseTrailers(t, bidi, testMdTrailers)
	})

	t.Run("failure", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		err = bidi.Send(req)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}

		req.Code = int32(codes.DataLoss)
		req.ErrorDetails = testErrorDetails
		err = bidi.Send(req)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		checkResponseHeaders(t, bidi, testMdHeaders)

		m, err := bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.DataLoss, testErrorMessages...)

		checkResponseTrailers(t, bidi, testMdTrailers)
	})

	t.Run("timeout", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		bidi, err := cli.BidiStream(tctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		req.Code = int32(codes.OK)
		req.DelayMillis = 500

		err = bidi.Send(req)
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
		req := proto.Clone(&reqPrototype).(*Message)
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		bidi, err := cli.BidiStream(cctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		req.Code = int32(codes.OK)
		req.DelayMillis = 500

		err = bidi.Send(req)
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
	ctx := metadata.NewOutgoingContext(context.Background(), MetadataNew(testOutgoingMd))
	reqPrototype := Message{
		Payload:  testPayload,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	t.Run("success", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		for i := 0; i < 3; i++ {
			err = bidi.Send(req)
			if err != nil {
				t.Fatalf("sending message #%d failed: %v", i+1, err)
			}

			if i == 0 {
				checkResponseHeaders(t, bidi, testMdHeaders)
			}

			m, err := bidi.Recv()
			if err != nil {
				t.Fatalf("receiving message #%d failed: %v", i+1, err)
			}
			if !bytes.Equal(testPayload, m.Payload) {
				t.Fatalf("wrong payload in message #%d: expecting %v; got %v", i+1, testPayload, m.Payload)
			}
			checkRequestHeaders(t, testOutgoingMd, m.Headers)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		if err != io.EOF {
			t.Fatalf("expected EOF; got %v", err)
		}

		checkResponseTrailers(t, bidi, testMdTrailers)
	})

	t.Run("failure", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		bidi, err := cli.BidiStream(ctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		err = bidi.Send(req)
		if err != nil {
			t.Fatalf("sending message #1 failed: %v", err)
		}

		checkResponseHeaders(t, bidi, testMdHeaders)

		m, err := bidi.Recv()
		if err != nil {
			t.Fatalf("receiving message failed: %v", err)
		}
		if !bytes.Equal(testPayload, m.Payload) {
			t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, m.Payload)
		}

		req.Code = int32(codes.DataLoss)
		req.ErrorDetails = testErrorDetails
		err = bidi.Send(req)
		if err != nil {
			t.Fatalf("sending message #2 failed: %v", err)
		}

		err = bidi.CloseSend()
		if err != nil {
			t.Fatalf("closing send-side of RPC failed: %v", err)
		}

		_, err = bidi.Recv()
		checkError(t, err, codes.DataLoss, testErrorMessages...)

		checkResponseTrailers(t, bidi, testMdTrailers)
	})

	t.Run("timeout", func(t *testing.T) {
		req := proto.Clone(&reqPrototype).(*Message)
		tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		bidi, err := cli.BidiStream(tctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		req.DelayMillis = 500
		err = bidi.Send(req)
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
		req := proto.Clone(&reqPrototype).(*Message)
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(100*time.Millisecond, cancel)

		bidi, err := cli.BidiStream(cctx)
		if err != nil {
			t.Fatalf("RPC failed: %v", err)
		}

		req.DelayMillis = 500
		err = bidi.Send(req)
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

func checkRequestHeaders(t *testing.T, expected, actual map[string][]byte) {
	t.Helper()
	// we don't just do a strict equals check because the actual headers
	// echoed back could have extra headers that were added implicitly
	// by the GRPC-over-HTTP client (such as GRPC-Timeout, Content-Type, etc).
	for k, v := range expected {
		v2, ok := actual[k]
		if !ok || !bytes.Equal(v2, v) {
			t.Fatalf("wrong headers echoed back: expecting header %s to be %q, instead was %q", k, v, v2)
		}
	}
}

func checkResponseMetadata(t *testing.T, cs grpc.ClientStream, hdrs map[string][]byte, tlrs map[string][]byte) {
	t.Helper()
	checkResponseHeaders(t, cs, hdrs)
	checkResponseTrailers(t, cs, tlrs)
}

func checkResponseHeaders(t *testing.T, cs grpc.ClientStream, md map[string][]byte) {
	t.Helper()
	h, err := cs.Header()
	if err != nil {
		t.Fatalf("failed to get header metadata: %v", err)
	}
	checkMetadata(t, md, h, "header")
}

func checkResponseTrailers(t *testing.T, cs grpc.ClientStream, md map[string][]byte) {
	t.Helper()
	checkMetadata(t, md, cs.Trailer(), "trailer")
}

func checkMetadata(t *testing.T, expected map[string][]byte, actual metadata.MD, name string) {
	t.Helper()

	// we don't just do a strict equals check because the actual headers
	// echoed back could have extra headers that were added implicitly
	// by the GRPC-over-HTTP client (such as GRPC-Timeout, Content-Type, etc).
	for k, v := range expected {
		v2, ok := actual[k]
		if !ok || len(v2) != 1 || !bytes.Equal([]byte(v2[0]), v) {
			t.Fatalf("wrong %ss echoed back: expecting %s %s to be [%s], instead was %v", name, name, k, v, v2)
		}
	}
}

func checkError(t *testing.T, err error, expectedCode codes.Code, expectedDetails ...proto.Message) {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("wrong type of error: %v", err)
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
