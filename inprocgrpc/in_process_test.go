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
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestInProcessChannel(t *testing.T) {
	svr := &grpchantesting.TestServer{}

	var cc inprocgrpc.Channel
	grpchantesting.RegisterTestServiceServer(&cc, svr)

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
	grpchantesting.RegisterTestServiceServer(&cc, svr)

	md := grpchantesting.File_test_proto.Messages().ByName("Message")
	if md == nil {
		t.Fatalf("could not find descriptor for grpchantesting.Message")
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
	req := dynamicpb.NewMessage(md)
	req.Set(md.Fields().ByName("payload"), protoreflect.ValueOfBytes(testPayload))
	setBytesMap(req, "headers", testMdHeaders)
	setBytesMap(req, "trailers", testMdTrailers)

	var hdr, tlr metadata.MD
	rsp := dynamicpb.NewMessage(md)
	err := cc.Invoke(ctx, "/grpchantesting.TestService/Unary", req, rsp, grpc.Header(&hdr), grpc.Trailer(&tlr))
	if err != nil {
		t.Fatalf("RPC failed: %v", err)
	}

	payload := rsp.Get(md.Fields().ByName("payload")).Bytes()
	if !bytes.Equal(testPayload, payload) {
		t.Fatalf("wrong payload returned: expecting %v; got %v", testPayload, payload)
	}
	reqHeaders := map[string][]byte{}
	rsp.Get(md.Fields().ByName("headers")).Map().Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		reqHeaders[k.String()] = v.Bytes()
		return true
	})
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

func setBytesMap(m *dynamicpb.Message, field string, values map[string][]byte) {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(field))
	mp := m.Mutable(fd).Map()
	for k, v := range values {
		mp.Set(protoreflect.ValueOfString(k).MapKey(), protoreflect.ValueOfBytes(v))
	}
}

// legacyMessage is a message that does not implement the current Protobuf Go
// API. It resembles the dynamic messages from github.com/jhump/protoreflect v1,
// which can be used with an in-process channel as long as the other side of
// the copy is a message with a compatible wire format.
type legacyMessage struct {
	wire []byte
}

func (m *legacyMessage) Reset()         { m.wire = nil }
func (m *legacyMessage) String() string { return "legacy" }
func (m *legacyMessage) ProtoMessage()  {}

func (m *legacyMessage) Marshal() ([]byte, error) { return m.wire, nil }

func (m *legacyMessage) UnmarshalMerge(b []byte) error {
	m.wire = append(m.wire, b...)
	return nil
}

func TestUseLegacyMessage(t *testing.T) {
	svr := &grpchantesting.TestServer{}

	var cc inprocgrpc.Channel
	grpchantesting.RegisterTestServiceServer(&cc, svr)

	payload := []byte{1, 2, 3}
	wire, err := proto.Marshal(&grpchantesting.Message{Payload: payload})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := &legacyMessage{wire: wire}
	rsp := &legacyMessage{}
	if err := cc.Invoke(context.Background(), "/grpchantesting.TestService/Unary", req, rsp); err != nil {
		t.Fatalf("RPC failed: %v", err)
	}

	var actual grpchantesting.Message
	if err := proto.Unmarshal(rsp.wire, &actual); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !bytes.Equal(payload, actual.Payload) {
		t.Fatalf("wrong payload returned: expecting %v; got %v", payload, actual.Payload)
	}
}
