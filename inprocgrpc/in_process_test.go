package inprocgrpc_test

import (
	"bytes"
	"context"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestInProcessChannel(t *testing.T) {
	svr := &grpchantesting.TestServer{}

	var cc inprocgrpc.Channel
	grpchantesting.RegisterHandlerTestService(&cc, svr)

	before := runtime.NumGoroutine()

	grpchantesting.RunChannelTestCases(t, &cc, true)

	// check for goroutine leaks
	deadline := time.Now().Add(time.Second * 5)
	after := 0
	for deadline.After(time.Now()) {
		after = runtime.NumGoroutine()
		if after <= before {
			// number of goroutines returned to previous level: no leak!
			return
		}
		time.Sleep(time.Millisecond * 50)
	}
	t.Errorf("%d goroutines leaked", after-before)
}

func TestUseDynamicMessage(t *testing.T) {
	// This uses dynamic messages for request and response and
	// ensures the in-process channel works correctly that way.

	svr := &grpchantesting.TestServer{}

	var cc inprocgrpc.Channel
	grpchantesting.RegisterHandlerTestService(&cc, svr)
	stub := grpcdynamic.NewStub(&cc)

	fd, err := desc.LoadFileDescriptor("grpchantesting/test.proto")
	if err != nil {
		t.Fatalf("failed to load descriptor for test.proto: %v", err)
	}
	md := fd.FindMessage("grpchantesting.Message")
	if md == nil {
		t.Fatalf("could not find descriptor for grpchantesting.Message")
	}
	sd := fd.FindService("grpchantesting.TestService")
	if sd == nil {
		t.Fatalf("could not find descriptor for grpchantesting.TestService")
	}
	mtd := sd.FindMethodByName("Unary")
	if mtd == nil {
		t.Fatalf("could not find descriptor for grpchantesting.TestService/Unary")
	}

	testPayload := []byte{100, 90, 80, 70, 60, 50, 40, 30, 20, 10, 0}
	testOutgoingMd := map[string][]byte{
		"foo": []byte("bar"),
	}
	testMdHeaders := map[string][]byte{
		"foo1": []byte("bar2"),
	}
	testMdTrailers := map[string][]byte{
		"foo3": []byte("bar4"),
	}

	ctx := metadata.NewOutgoingContext(context.Background(), grpchantesting.MetadataNew(testOutgoingMd))
	req := dynamic.NewMessage(md)
	req.SetFieldByName("payload", testPayload)
	req.SetFieldByName("headers", testMdHeaders)
	req.SetFieldByName("trailers", testMdTrailers)

	var hdr, tlr metadata.MD
	rsp, err := stub.InvokeRpc(ctx, mtd, req, grpc.Header(&hdr), grpc.Trailer(&tlr))
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}
	msg := rsp.(*dynamic.Message)

	payload := msg.GetFieldByName("payload")
	if !bytes.Equal(testPayload, payload.([]byte)) {
		t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, payload)
	}
	reqHeaders := map[string][]byte{}
	for k, v := range msg.GetFieldByName("headers").(map[interface{}]interface{}) {
		reqHeaders[k.(string)] = v.([]byte)
	}
	if !reflect.DeepEqual(testOutgoingMd, reqHeaders) {
		t.Fatalf("wrong request headers echoed back: expecting %v; got %v", testOutgoingMd, reqHeaders)
	}

	actualHdrs := map[string][]byte{}
	for k, v := range hdr {
		if len(v) > 1 {
			t.Fatalf("too many values for response header %q", k)
		}
		actualHdrs[k] = []byte(v[0])
	}
	if !reflect.DeepEqual(testMdHeaders, actualHdrs) {
		t.Fatalf("wrong response headers echoed back: expecting %v; got %v", testMdHeaders, actualHdrs)
	}

	actualTlrs := map[string][]byte{}
	for k, v := range tlr {
		if len(v) > 1 {
			t.Fatalf("too many values for response trailer %q", k)
		}
		actualTlrs[k] = []byte(v[0])
	}
	if !reflect.DeepEqual(testMdTrailers, actualTlrs) {
		t.Fatalf("wrong response trailers echoed back: expecting %v; got %v", testMdTrailers, actualTlrs)
	}
}
