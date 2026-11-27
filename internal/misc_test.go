package internal

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCopyMessage(t *testing.T) {
	src := timestamppb.New(timestamppb.Now().AsTime())
	dst := &timestamppb.Timestamp{Seconds: 1, Nanos: 2}
	if err := CopyMessage(dst, src); err != nil {
		t.Fatalf("CopyMessage failed: %v", err)
	}
	if !proto.Equal(src, dst) {
		t.Fatalf("copy is not equal: %v != %v", dst, src)
	}
	if dst == src {
		t.Fatal("copy should be a different message")
	}
}

func TestCopyMessage_DynamicAndGenerated(t *testing.T) {
	md := (&timestamppb.Timestamp{}).ProtoReflect().Descriptor()
	generated := &timestamppb.Timestamp{Seconds: 123, Nanos: 456}

	dynamic := dynamicpb.NewMessage(md)
	if err := CopyMessage(dynamic, generated); err != nil {
		t.Fatalf("copy to dynamic message failed: %v", err)
	}
	if got := dynamic.Get(md.Fields().ByName("seconds")).Int(); got != 123 {
		t.Fatalf("wrong seconds in dynamic message: %d", got)
	}

	roundTrip := &timestamppb.Timestamp{}
	if err := CopyMessage(roundTrip, dynamic); err != nil {
		t.Fatalf("copy from dynamic message failed: %v", err)
	}
	if !proto.Equal(generated, roundTrip) {
		t.Fatalf("round trip is not equal: %v != %v", roundTrip, generated)
	}
}

func TestCopyMessage_Errors(t *testing.T) {
	testCases := map[string]struct {
		out, in any
		errText string
	}{
		"type mismatch":        {&durationpb.Duration{}, &timestamppb.Timestamp{}, "type mismatch"},
		"in is not a message":  {&timestamppb.Timestamp{}, "foo", "not a proto.Message"},
		"out is not a message": {"foo", &timestamppb.Timestamp{}, "not a proto.Message"},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := CopyMessage(tc.out, tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.errText) {
				t.Fatalf("expected error containing %q, got %v", tc.errText, err)
			}
		})
	}
}

func TestCopyMessage_NilSource(t *testing.T) {
	var nilTimestamp *timestamppb.Timestamp
	dst := &timestamppb.Timestamp{Seconds: 1}
	if err := CopyMessage(dst, nilTimestamp); err != nil {
		t.Fatalf("CopyMessage failed: %v", err)
	}
	if !proto.Equal(dst, &timestamppb.Timestamp{}) {
		t.Fatalf("expected an empty message, got %v", dst)
	}
}

func TestCloneMessage(t *testing.T) {
	src := &timestamppb.Timestamp{Seconds: 5}
	clone, err := CloneMessage(src)
	if err != nil {
		t.Fatalf("CloneMessage failed: %v", err)
	}
	got, ok := clone.(*timestamppb.Timestamp)
	if !ok || got == src || !proto.Equal(src, got) {
		t.Fatalf("bad clone: %T %v", clone, clone)
	}
	if _, err := CloneMessage("foo"); err == nil {
		t.Fatal("expected an error for a value that is not a message")
	}
}
