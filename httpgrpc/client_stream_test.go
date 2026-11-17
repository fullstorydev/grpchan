package httpgrpc

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/fullstorydev/grpchan/internal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }

func (nopWriteCloser) Close() error { return nil }

// TestRecvMsgReportsWhyStreamEnded checks that a RecvMsg that is already
// waiting when SendMsg ends the stream reports SendMsg's error, rather than
// the cancellation that SendMsg uses to tear the stream down.
func TestRecvMsgReportsWhyStreamEnded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		writeMsg := func(any, bool) error {
			return marshalError{err: errors.New("cannot marshal")}
		}
		cs := newClientStream(ctx, cancel, nopWriteCloser{}, true, &internal.CallOptions{}, nil, writeMsg, nil)

		recvErr := make(chan error, 1)
		go func() {
			recvErr <- cs.RecvMsg(nil)
		}()
		// Wait for RecvMsg to block, waiting for a message.
		synctest.Wait()

		sendErr := cs.SendMsg(nil)
		if status.Code(sendErr) != codes.Internal {
			t.Fatalf("SendMsg: got %v, want an Internal error", sendErr)
		}
		if err := <-recvErr; err == nil || err.Error() != sendErr.Error() {
			t.Fatalf("RecvMsg: got %v, want the error from SendMsg: %v", err, sendErr)
		}
	})
}
