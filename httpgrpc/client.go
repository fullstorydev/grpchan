package httpgrpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/fullstorydev/grpchan/internal"
)

// Channel is used as a connection for GRPC requests issued over HTTP 1.1. The
// server endpoint is configured using the BaseURL field, and the Transport can
// also be configured. Both of those fields must be specified.
//
// It implements version 1 of the GRPC-over-HTTP transport protocol.
type Channel struct {
	Transport http.RoundTripper
	BaseURL   *url.URL
}

var _ grpc.ClientConnInterface = (*Channel)(nil)

var grpcDetailsHeader = textproto.CanonicalMIMEHeaderKey("X-GRPC-Details")

// Invoke satisfies the grpchan.Channel interface and supports sending unary
// RPCs via the in-process channel.
func (ch *Channel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {
	copts := internal.GetCallOptions(opts)

	reqUrl := *ch.BaseURL
	reqUrl.Path = path.Join(reqUrl.Path, methodName)
	reqUrlStr := reqUrl.String()
	ctx, err := internal.ApplyPerRPCCreds(ctx, copts, reqUrlStr, reqUrl.Scheme == "https")
	if err != nil {
		return err
	}
	h := headersFromContext(ctx)
	h.Set("Content-Type", UnaryRpcContentType_V1)

	b, err := proto.Marshal(req.(proto.Message))
	if err != nil {
		return err
	}

	// TODO: enforce max send and receive size in call options

	r, err := http.NewRequest("POST", reqUrlStr, bytes.NewReader(b))
	if err != nil {
		return err
	}
	r.Header = h
	reply, err := ch.Transport.RoundTrip(r.WithContext(ctx))
	if err != nil {
		return statusFromContextError(err)
	}

	// we fire up a goroutine to read the response so that we can properly
	// respect any context deadline (e.g. don't want to be blocked, reading
	// from socket, long past requested timeout).
	respCh := make(chan struct{})
	go func() {
		defer close(respCh)
		b, err = ioutil.ReadAll(reply.Body)
		reply.Body.Close()
	}()

	if len(copts.Peer) > 0 {
		copts.SetPeer(getPeer(ch.BaseURL, r.TLS))
	}

	// gather headers and trailers
	if len(copts.Headers) > 0 || len(copts.Trailers) > 0 {
		if err := setMetadata(reply.Header, copts); err != nil {
			return err
		}
	}

	if stat := statFromResponse(reply); stat.Code() != codes.OK {
		return stat.Err()
	}

	select {
	case <-ctx.Done():
		return statusFromContextError(ctx.Err())
	case <-respCh:
	}
	if err != nil {
		return err
	}
	return proto.Unmarshal(b, resp.(proto.Message))
}

// NewStream satisfies the grpchan.Channel interface and supports sending
// streaming RPCs via the in-process channel.
func (ch *Channel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	copts := internal.GetCallOptions(opts)

	reqUrl := *ch.BaseURL
	reqUrl.Path = path.Join(reqUrl.Path, methodName)
	reqUrlStr := reqUrl.String()
	ctx, err := internal.ApplyPerRPCCreds(ctx, copts, reqUrlStr, reqUrl.Scheme == "https")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	h := headersFromContext(ctx)
	h.Set("Content-Type", StreamRpcContentType_V1)

	r, w := io.Pipe()
	req, err := http.NewRequest("POST", reqUrlStr, r)
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header = h

	cs := newClientStream(ctx, cancel, w, desc.ServerStreams, copts, ch.BaseURL)
	// ensure that context is cancelled, even if caller
	// fails to fully consume or cancel the stream
	runtime.SetFinalizer(cs, func(*clientStream) { cancel() })

	go cs.doHttpCall(ch.Transport, req)

	return cs, nil
}

func getPeer(baseUrl *url.URL, tls *tls.ConnectionState) *peer.Peer {
	hostPort := baseUrl.Host
	if !strings.Contains(hostPort, ":") {
		if baseUrl.Scheme == "https" {
			hostPort = hostPort + ":443"
		} else if baseUrl.Scheme == "http" {
			hostPort = hostPort + ":80"
		}
	}
	pr := peer.Peer{Addr: strAddr(hostPort)}
	if tls != nil {
		pr.AuthInfo = credentials.TLSInfo{State: *tls}
	}
	return &pr
}

func setMetadata(h http.Header, copts *internal.CallOptions) error {
	hdr, err := asMetadata(h)
	if err != nil {
		return err
	}
	tlr := metadata.MD{}

	const trailerPrefix = "x-grpc-trailer-"

	for k, v := range hdr {
		if strings.HasPrefix(strings.ToLower(k), trailerPrefix) {
			trailerName := k[len(trailerPrefix):]
			if trailerName != "" {
				tlr[trailerName] = v
				delete(hdr, k)
			}
		}
	}

	copts.SetHeaders(hdr)
	copts.SetTrailers(tlr)
	return nil
}

// clientStream implements a client stream over HTTP 1.1. A goroutine sets up the
// RPC by initiating an HTTP 1.1 request, reading the response, and decoding that
// response stream into messages which are fed to this stream via the rCh field.
// Sending messages is handled synchronously, writing to a pipe that feeds the
// HTTP 1.1 request body.
type clientStream struct {
	ctx     context.Context
	cancel  context.CancelFunc
	copts   *internal.CallOptions
	baseUrl *url.URL

	// respStream is set to indicate whether client expects stream response; unary if false
	respStream bool

	// hd and hdErr are populated when ready is done
	ready sync.WaitGroup
	hdErr error
	hd    metadata.MD

	// rCh is used to deliver messages from doHttpCall goroutine
	// to callers of RecvMsg.
	// done must be set to true before it is closed
	rCh chan []byte

	// rMu protects done, rErr, and tr
	rMu  sync.RWMutex
	done bool
	rErr error
	tr   HttpTrailer

	// wMu protects w and wErr
	wMu  sync.Mutex
	w    io.WriteCloser
	wErr error
}

func newClientStream(ctx context.Context, cancel context.CancelFunc, w io.WriteCloser, recvStream bool, copts *internal.CallOptions, baseUrl *url.URL) *clientStream {
	cs := &clientStream{
		ctx:        ctx,
		cancel:     cancel,
		copts:      copts,
		baseUrl:    baseUrl,
		w:          w,
		respStream: recvStream,
		rCh:        make(chan []byte),
	}
	cs.ready.Add(1)
	return cs
}

func (cs *clientStream) Header() (metadata.MD, error) {
	cs.ready.Wait()
	return cs.hd, cs.hdErr
}

func (cs *clientStream) Trailer() metadata.MD {
	// only safe to read trailers after stream has completed
	cs.rMu.RLock()
	defer cs.rMu.RUnlock()
	if cs.done {
		return metadataFromProto(cs.tr.Metadata)
	}
	return nil
}

func metadataFromProto(trailers map[string]*TrailerValues) metadata.MD {
	md := metadata.MD{}
	for k, vs := range trailers {
		md[k] = vs.Values
	}
	return md
}

func (cs *clientStream) CloseSend() error {
	cs.wMu.Lock()
	defer cs.wMu.Unlock()
	return cs.w.Close()
}

func (cs *clientStream) Context() context.Context {
	return cs.ctx
}

func (cs *clientStream) readErrorIfDone() (bool, error) {
	cs.rMu.RLock()
	defer cs.rMu.RUnlock()
	if !cs.done {
		return false, nil
	}
	if cs.rErr != nil {
		return true, cs.rErr
	}
	if cs.tr.Code == int32(codes.OK) {
		return true, io.EOF
	}
	statProto := spb.Status{
		Code:    cs.tr.Code,
		Message: cs.tr.Message,
		Details: cs.tr.Details,
	}
	return true, status.FromProto(&statProto).Err()
}

func (cs *clientStream) SendMsg(m interface{}) error {
	// GRPC streams return EOF error for attempts to send on closed stream
	if done, _ := cs.readErrorIfDone(); done {
		return io.EOF
	}

	cs.wMu.Lock()
	defer cs.wMu.Unlock()
	if cs.wErr != nil {
		// earlier write error means stream is effectively closed
		return io.EOF
	}

	cs.wErr = writeProtoMessage(cs.w, m.(proto.Message), false)
	return cs.wErr
}

func (cs *clientStream) RecvMsg(m interface{}) error {
	if done, err := cs.readErrorIfDone(); done {
		return err
	}

	select {
	case <-cs.ctx.Done():
		return statusFromContextError(cs.ctx.Err())
	case msg, ok := <-cs.rCh:
		if !ok {
			done, err := cs.readErrorIfDone()
			if !done {
				// sanity check: this shouldn't be possible
				panic("cs.rCh was closed but cs.done == false!")
			}
			return err
		}
		err := proto.Unmarshal(msg, m.(proto.Message))
		if err != nil {
			return status.Error(codes.Internal, fmt.Sprintf("server sent invalid message: %v", err))
		}
		if !cs.respStream {
			// We need to query the channel for a second message. If there *is* a
			// second message, the server tried to send too many, and that's an
			// error. And if there isn't a second message, we still need to see the
			// channel close (e.g. end-of-stream) so we know that tr is set (so that
			// it's available for a subsequent call to Trailer)
			select {
			case <-cs.ctx.Done():
				return statusFromContextError(cs.ctx.Err())
			case _, ok := <-cs.rCh:
				if ok {
					// server tried to send >1 message!
					cs.rMu.Lock()
					defer cs.rMu.Unlock()
					if cs.rErr == nil {
						cs.rErr = status.Error(codes.Internal, "method should return 1 response message but server sent >1")
						cs.done = true
						// we won't be reading from the channel anymore, so we must
						// cancel the context so that doHttpCall doesn't hang trying
						// to write to channel
						cs.cancel()
					}
					return cs.rErr
				}
				// if server sent a failure after the single message, the failure takes precedence
				done, err := cs.readErrorIfDone()
				if !done {
					// sanity check: this shouldn't be possible
					panic("cs.rCh was closed but cs.done == false!")
				}
				if err != io.EOF {
					return err
				}
			}
		}
		return nil
	}
}

// doHttpCall performs the HTTP round trip and then reads the reply body,
// sending delimited messages to the clientStream via a channel.
func (cs *clientStream) doHttpCall(transport http.RoundTripper, req *http.Request) {
	// On completion, we must fill in cs.tr or cs.rErr and then close channel,
	// which signals to client code that we've reached end-of-stream.

	var rErr error
	rMuHeld := false

	defer func() {
		if !rMuHeld {
			cs.rMu.Lock()
		}
		defer cs.rMu.Unlock()

		if rErr != nil && cs.rErr == nil {
			cs.rErr = rErr
		}
		cs.done = true
		close(cs.rCh)
	}()

	onReady := func(err error, headers metadata.MD) {
		cs.hdErr = err
		cs.hd = headers
		if len(headers) > 0 && len(cs.copts.Headers) > 0 {
			cs.copts.SetHeaders(headers)
		}
		rErr = err
		cs.ready.Done()
	}

	reply, err := transport.RoundTrip(req.WithContext(cs.ctx))
	if err != nil {
		onReady(statusFromContextError(err), nil)
		return
	}
	defer reply.Body.Close()

	if len(cs.copts.Peer) > 0 {
		cs.copts.SetPeer(getPeer(cs.baseUrl, reply.TLS))
	}
	md, err := asMetadata(reply.Header)
	if err != nil {
		onReady(err, nil)
		return
	}

	onReady(nil, md)

	stat := statFromResponse(reply)
	if stat.Code() != codes.OK {
		statProto := stat.Proto()
		cs.tr.Code = statProto.Code
		cs.tr.Message = statProto.Message
		cs.tr.Details = statProto.Details
		return
	}

	counter := 0
	for {
		// TODO: enforce max send and receive size in call options

		counter++
		var sz int32
		sz, rErr = readSizePreface(reply.Body)
		if rErr != nil {
			return
		}
		if sz < 0 {
			// final message is a trailer (need lock to write to cs.tr)
			cs.rMu.Lock()
			rMuHeld = true // defer above will unlock for us
			cs.rErr = readProtoMessage(reply.Body, int32(-sz), &cs.tr)
			if cs.rErr != nil {
				if cs.rErr == io.EOF {
					cs.rErr = io.ErrUnexpectedEOF
				}
			}
			if len(cs.tr.Metadata) > 0 && len(cs.copts.Trailers) > 0 {
				cs.copts.SetTrailers(metadataFromProto(cs.tr.Metadata))
			}
			return
		}
		msg := make([]byte, sz)
		_, rErr = io.ReadAtLeast(reply.Body, msg, int(sz))
		if rErr != nil {
			if rErr == io.EOF {
				rErr = io.ErrUnexpectedEOF
			}
			return
		}

		select {
		case <-cs.ctx.Done():
			// operation timed out or was cancelled before we could
			// successfully send this message to client code
			rErr = statusFromContextError(cs.ctx.Err())
			return
		case cs.rCh <- msg:
		}
	}
}

// statusFromContextError translates the given error, returned by a call to
// context.Context.Err(), into a suitable GRPC error. If the given error is
// not a context error (e.g. neither deadline exceeded nor canceled) then it
// is returned as is.
func statusFromContextError(err error) error {
	if err == context.DeadlineExceeded {
		return status.Error(codes.DeadlineExceeded, err.Error())
	} else if err == context.Canceled {
		return status.Error(codes.Canceled, err.Error())
	}
	return err
}

// headersFromContext returns HTTP request headers to send to the remote host
// based on the specified context. GRPC clients store outgoing metadata into the
// context, which is translated into headers. Also, a context deadline will be
// propagated to the server via GRPC timeout metadata.
func headersFromContext(ctx context.Context) http.Header {
	h := http.Header{}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		toHeaders(md, h, "")
	}
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		millis := int64(timeout / time.Millisecond)
		if millis <= 0 {
			millis = 1
		}
		h.Set("GRPC-Timeout", fmt.Sprintf("%dm", millis))
	}
	return h
}

func statFromResponse(reply *http.Response) *status.Status {
	code := codeFromHttpStatus(reply.StatusCode)
	msg := reply.Status
	codeStrs := strings.SplitN(reply.Header.Get("X-GRPC-Status"), ":", 2)
	if len(codeStrs) > 0 && codeStrs[0] != "" {
		if c, err := strconv.ParseInt(codeStrs[0], 10, 32); err == nil {
			code = codes.Code(c)
		}
		if len(codeStrs) > 1 {
			msg = codeStrs[1]
		}
	}
	if code != codes.OK {
		var details []*any.Any
		if detailHeaders := reply.Header[grpcDetailsHeader]; len(detailHeaders) > 0 {
			details = make([]*any.Any, 0, len(detailHeaders))
			for _, d := range detailHeaders {
				b, err := base64.RawURLEncoding.DecodeString(d)
				if err != nil {
					continue
				}
				var msg any.Any
				if err := proto.Unmarshal(b, &msg); err != nil {
					continue
				}
				details = append(details, &msg)
			}
		}
		if len(details) > 0 {
			statProto := spb.Status{
				Code:    int32(code),
				Message: msg,
				Details: details,
			}
			return status.FromProto(&statProto)
		}
		return status.New(code, msg)
	}
	return nil
}
