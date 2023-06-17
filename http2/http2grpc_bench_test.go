package http2grpc_test

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// func TestGrpcOverHttp(t *testing.T) {
// 	svr := &grpchantesting.TestServer{}
// 	reg := grpchan.HandlerMap{}
// 	grpchantesting.RegisterTestServiceServer(reg, svr)

// 	var mux http.ServeMux
// 	httpgrpc.HandleServices(mux.HandleFunc, "/", reg, nil, nil)

// 	l, err := net.Listen("tcp", "127.0.0.1:0")
// 	if err != nil {
// 		t.Fatalf("failed it listen on socket: %v", err)
// 	}
// 	httpServer := http.Server{Handler: &mux}
// 	go httpServer.Serve(l)
// 	defer httpServer.Close()

// 	// now setup client stub
// 	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", l.Addr().(*net.TCPAddr).Port))
// 	if err != nil {
// 		t.Fatalf("failed to parse base URL: %v", err)
// 	}
// 	cc := httpgrpc.Channel{
// 		Transport: http.DefaultTransport,
// 		BaseURL:   u,
// 	}

// 	grpchantesting.RunChannelTestCases(t, &cc, false)

// 	t.Run("empty-trailer", func(t *testing.T) {
// 		// test RPC w/ streaming response where trailer message is empty
// 		// (e.g. no trailer metadata and code == 0 [OK])
// 		cli := grpchantesting.NewTestServiceClient(&cc)
// 		str, err := cli.ServerStream(context.Background(), &grpchantesting.Message{})
// 		if err != nil {
// 			t.Fatalf("failed to initiate server stream: %v", err)
// 		}
// 		// if there is an issue with trailer message, it will appear to be
// 		// a regular message and err would be nil
// 		_, err = str.Recv()
// 		if err != io.EOF {
// 			t.Fatalf("server stream should not have returned any messages")
// 		}
// 	})
// }

// This test is nearly identical to TestGrpcOverHttp, except that it uses
// *httpgrpc.Server instead of httpgrpc.HandleServices.
func BenchmarkServer(b *testing.B) {
	// errFunc := func(reqCtx context.Context, st *status.Status, response http.ResponseWriter) {

	// }
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)
	ctx := context.Background()

	svc := &grpchantesting.TestServer{}
	svr := grpc.NewServer()
	// svr := httpgrpc.NewServer(httpgrpc.WithBasePath("/foo/"), httpgrpc.ErrorRenderer(errFunc))
	grpchantesting.RegisterTestServiceServer(svr, svc)

	go svr.Serve(lis)

	conn, err := grpc.DialContext(ctx, "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	grpchantesting.RunChannelBenchmarkCases(b, conn, false)

	defer svr.Stop()

}
