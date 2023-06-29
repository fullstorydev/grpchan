package grpchantesting

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/loov/hrtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RunChannelBenchmarkCases runs numerous test cases to exercise the behavior of the
// given channel. The server side of the channel needs to have a *TestServer (in
// this package) registered to provide the implementation of fsgrpc.TestService
// (proto in this package). If the channel does not support full-duplex
// communication, it must provide at least half-duplex support for bidirectional
// streams.
//
// The test cases will be defined as child tests by invoking t.Run on the given
// *testing.T.
func RunChannelBenchmarkCases(b *testing.B, ch grpc.ClientConnInterface, supportsFullDuplex bool) {
	cli := NewTestServiceClient(ch)

	// b.Run("unary_latency_stats", func(b *testing.B) { BenchmarkUnaryLatency(b, cli) })

	b.Run("unary_latency_histogram", func(b *testing.B) { BenchmarkHistogramUnaryLatency(b, cli) })
}

func BenchmarkHistogramUnaryLatency(b *testing.B, cli TestServiceClient) {
	// bench := hrtesting.NewBenchmark(b)
	bench := hrtime.NewBenchmark(100000)
	// defer bench.Report()

	ctx := metadata.NewOutgoingContext(context.Background(), MetadataNew(testOutgoingMd))

	reqPrototype := Message{
		Payload:  testPayload,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	var hdr, tlr metadata.MD
	req := proto.Clone(&reqPrototype).(*Message)
	for bench.Next() {
		rsp, err := cli.Unary(ctx, req, grpc.Header(&hdr), grpc.Trailer(&tlr))
		// b.StopTimer()
		if err != nil {
			b.Fatalf("RPC failed: %v", err)
		}
		if !bytes.Equal(testPayload, rsp.Payload) {
			b.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, rsp.Payload)
		}
		checkRequestHeadersBench(b, testOutgoingMd, rsp.Headers)

		checkMetadataBench(b, testMdHeaders, hdr, "header")
		checkMetadataBench(b, testMdTrailers, tlr, "trailer")

	}
	fmt.Println(bench.Histogram(10))
}

func BenchmarkUnaryLatency(b *testing.B, cli TestServiceClient) {
	ctx := metadata.NewOutgoingContext(context.Background(), MetadataNew(testOutgoingMd))
	reqPrototype := Message{
		Payload:  testPayload,
		Headers:  testMdHeaders,
		Trailers: testMdTrailers,
	}

	var hdr, tlr metadata.MD
	req := proto.Clone(&reqPrototype).(*Message)
	// b.Run("success", func(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// b.StartTimer()
		rsp, err := cli.Unary(ctx, req, grpc.Header(&hdr), grpc.Trailer(&tlr))
		// b.StopTimer()
		if err != nil {
			b.Fatalf("RPC failed: %v", err)
		}
		if !bytes.Equal(testPayload, rsp.Payload) {
			b.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, rsp.Payload)
		}
		checkRequestHeadersBench(b, testOutgoingMd, rsp.Headers)

		checkMetadataBench(b, testMdHeaders, hdr, "header")
		checkMetadataBench(b, testMdTrailers, tlr, "trailer")

	}
	// })
}

func checkRequestHeadersBench(b *testing.B, expected, actual map[string][]byte) {
	// we don't just do a strict equals check because the actual headers
	// echoed back could have extra headers that were added implicitly
	// by the GRPC-over-HTTP client (such as GRPC-Timeout, Content-Type, etc).
	for k, v := range expected {
		v2, ok := actual[k]
		if !ok || !bytes.Equal(v2, v) {
			b.Fatalf("wrong headers echoed back: expecting header %s to be %q, instead was %q", k, v, v2)
		}
	}
}

func checkResponseMetadataBench(b *testing.B, cs grpc.ClientStream, hdrs map[string][]byte, tlrs map[string][]byte) {
	checkResponseHeadersBench(b, cs, hdrs)
	checkResponseTrailersBench(b, cs, tlrs)
}

func checkResponseHeadersBench(b *testing.B, cs grpc.ClientStream, md map[string][]byte) {
	h, err := cs.Header()
	if err != nil {
		b.Fatalf("failed to get header metadata: %v", err)
	}
	checkMetadataBench(b, md, h, "header")
}

func checkResponseTrailersBench(b *testing.B, cs grpc.ClientStream, md map[string][]byte) {
	checkMetadataBench(b, md, cs.Trailer(), "trailer")
}

func checkMetadataBench(b *testing.B, expected map[string][]byte, actual metadata.MD, name string) {
	// we don't just do a strict equals check because the actual headers
	// echoed back could have extra headers that were added implicitly
	// by the GRPC-over-HTTP client (such as GRPC-Timeout, Content-Type, etc).
	for k, v := range expected {
		v2, ok := actual[k]
		if !ok || len(v2) != 1 || !bytes.Equal([]byte(v2[0]), v) {
			b.Fatalf("wrong %ss echoed back: expecting %s %s to be [%s], instead was %v", name, name, k, v, v2)
		}
	}
}

func checkErrorBench(b *testing.B, err error, expectedCode codes.Code, expectedDetails ...proto.Message) {
	st, ok := status.FromError(err)
	if !ok {
		b.Fatalf("wrong type of error: %v", err)
	}
	if st.Code() != expectedCode {
		b.Fatalf("wrong response code: %v != %v", st.Code(), expectedCode)
	}
	actualDetails := st.Details()
	if len(actualDetails) != len(expectedDetails) {
		b.Fatalf("wrong number of error details: %v != %v", len(actualDetails), len(expectedDetails))
	}
	for i, msg := range actualDetails {
		if !proto.Equal(msg.(proto.Message), expectedDetails[i]) {
			b.Fatalf("wrong error detail message at index %d: %v != %v", i, msg, expectedDetails[i])
		}
	}
}
