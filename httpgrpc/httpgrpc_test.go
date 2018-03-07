package httpgrpc_test

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/httpgrpc"
)

func TestGrpcOverHttp(t *testing.T) {
	svr := &grpchantesting.TestServer{}
	reg := grpchan.HandlerMap{}
	grpchantesting.RegisterTestServiceHandler(reg, svr)

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
}
