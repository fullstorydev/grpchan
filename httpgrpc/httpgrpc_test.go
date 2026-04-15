package httpgrpc_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/httpgrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestGrpcOverHttp(t *testing.T) {
	svr := &grpchantesting.TestServer{}
	reg := grpchan.HandlerMap{}
	grpchantesting.RegisterTestServiceServer(reg, svr)

	var mux http.ServeMux
	httpgrpc.HandleServices(mux.HandleFunc, "/", reg, nil, nil)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed it listen on socket: %v", err)
	}
	httpServer := http.Server{Handler: &mux}
	go httpServer.Serve(l)
	defer httpServer.Close()

	// now setup client stub
	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", l.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	cc := httpgrpc.Channel{
		Transport: http.DefaultTransport,
		BaseURL:   u,
	}

	grpchantesting.RunChannelTestCases(t, &cc, false)

	t.Run("empty-trailer", func(t *testing.T) {
		// test RPC w/ streaming response where trailer message is empty
		// (e.g. no trailer metadata and code == 0 [OK])
		cli := grpchantesting.NewTestServiceClient(&cc)
		str, err := cli.ServerStream(context.Background(), &grpchantesting.Message{})
		if err != nil {
			t.Fatalf("failed to initiate server stream: %v", err)
		}
		// if there is an issue with trailer message, it will appear to be
		// a regular message and err would be nil
		_, err = str.Recv()
		if err != io.EOF {
			t.Fatalf("server stream should not have returned any messages")
		}
	})
}

// This test is nearly identical to TestGrpcOverHttp, except that it uses
// *httpgrpc.Server instead of httpgrpc.HandleServices.
func TestServer(t *testing.T) {
	errFunc := func(reqCtx context.Context, st *status.Status, response http.ResponseWriter) {
	}

	svc := &grpchantesting.TestServer{}
	svr := httpgrpc.NewServer(httpgrpc.WithBasePath("/foo/"), httpgrpc.ErrorRenderer(errFunc))
	grpchantesting.RegisterTestServiceServer(svr, svc)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed it listen on socket: %v", err)
	}
	httpServer := http.Server{Handler: svr}
	go httpServer.Serve(l)
	defer httpServer.Close()

	// now setup client stub
	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d/foo/", l.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	cc := httpgrpc.Channel{
		Transport: http.DefaultTransport,
		BaseURL:   u,
	}

	grpchantesting.RunChannelTestCases(t, &cc, false)

	t.Run("empty-trailer", func(t *testing.T) {
		// test RPC w/ streaming response where trailer message is empty
		// (e.g. no trailer metadata and code == 0 [OK])
		cli := grpchantesting.NewTestServiceClient(&cc)
		str, err := cli.ServerStream(context.Background(), &grpchantesting.Message{})
		if err != nil {
			t.Fatalf("failed to initiate server stream: %v", err)
		}
		// if there is an issue with trailer message, it will appear to be
		// a regular message and err would be nil
		_, err = str.Recv()
		if err != io.EOF {
			t.Fatalf("server stream should not have returned any messages")
		}
	})
}

func TestJSONSSEServer(t *testing.T) {
	errFunc := func(reqCtx context.Context, st *status.Status, response http.ResponseWriter) {
	}

	svc := &grpchantesting.TestServer{}
	svr := httpgrpc.NewServer(httpgrpc.WithBasePath("/foo/"), httpgrpc.ErrorRenderer(errFunc))
	grpchantesting.RegisterTestServiceServer(svr, svc)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed it listen on socket: %v", err)
	}
	httpServer := http.Server{Handler: svr}
	go httpServer.Serve(l)
	defer httpServer.Close()

	// now setup client stub
	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d/foo/", l.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	cc := httpgrpc.NewChannel(u, http.DefaultTransport, httpgrpc.WithJSONEncoding(true))

	grpchantesting.RunChannelTestCases(t, cc, false)

	t.Run("empty-trailer", func(t *testing.T) {
		// test RPC w/ streaming response where trailer message is empty
		// (e.g. no trailer metadata and code == 0 [OK])
		cli := grpchantesting.NewTestServiceClient(cc)
		str, err := cli.ServerStream(context.Background(), &grpchantesting.Message{})
		if err != nil {
			t.Fatalf("failed to initiate server stream: %v", err)
		}
		// if there is an issue with trailer message, it will appear to be
		// a regular message and err would be nil
		_, err = str.Recv()
		if err != io.EOF {
			t.Fatalf("server stream should not have returned any messages")
		}
	})
}

// TestUnaryXGrpcDetailsWireCodec asserts that X-GRPC-Details header payloads use
// the same encoding as the unary request body (protobuf vs JSON), so the
// client recovers google.rpc.Status details correctly for both modes.
func TestUnaryXGrpcDetailsWireCodec(t *testing.T) {
	detailMsg := &structpb.ListValue{
		Values: []*structpb.Value{
			{Kind: &structpb.Value_StringValue{StringValue: "x-grpc-details-wire"}},
		},
	}
	wantAny := new(anypb.Any)
	if err := anypb.MarshalFrom(wantAny, detailMsg, proto.MarshalOptions{}); err != nil {
		t.Fatalf("marshal detail any: %v", err)
	}

	svc := &grpchantesting.TestServer{}
	reg := grpchan.HandlerMap{}
	grpchantesting.RegisterTestServiceServer(reg, svc)

	mux := http.NewServeMux()
	httpgrpc.HandleServices(mux.HandleFunc, "/", reg, nil, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	u, err := url.Parse(fmt.Sprintf("http://%s", ln.Addr().String()))
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	mkReq := func() *grpchantesting.Message {
		return &grpchantesting.Message{
			Code:         int32(codes.FailedPrecondition),
			ErrorDetails: []*anypb.Any{proto.Clone(wantAny).(*anypb.Any)},
		}
	}

	t.Run("protobuf", func(t *testing.T) {
		cc := &httpgrpc.Channel{Transport: http.DefaultTransport, BaseURL: u}
		cli := grpchantesting.NewTestServiceClient(cc)
		_, err := cli.Unary(context.Background(), mkReq())
		assertUnaryErrorHasDetail(t, err, codes.FailedPrecondition, detailMsg)
	})

	t.Run("json", func(t *testing.T) {
		cc := httpgrpc.NewChannel(u, http.DefaultTransport, httpgrpc.WithJSONEncoding(true))
		cli := grpchantesting.NewTestServiceClient(cc)
		_, err := cli.Unary(context.Background(), mkReq())
		assertUnaryErrorHasDetail(t, err, codes.FailedPrecondition, detailMsg)
	})
}

func assertUnaryErrorHasDetail(t *testing.T, err error, wantCode codes.Code, wantDetail proto.Message) {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != wantCode {
		t.Fatalf("status code: got %v want %v", st.Code(), wantCode)
	}
	details := st.Details()
	if len(details) != 1 {
		t.Fatalf("status details: got %d want 1 (%v)", len(details), details)
	}
	if !proto.Equal(details[0].(proto.Message), wantDetail) {
		t.Fatalf("status detail mismatch:\ngot  %v\nwant %v", details[0], wantDetail)
	}
}
