package httpgrpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/mem"

	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	grpcproto "google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/fullstorydev/grpchan/internal"
)

// ChannelOption is a function that can be used to configure a Channel.
type ChannelOption func(*channelOptions) error

type channelOptions struct {
	// TODO(kellegous): It would be ideal if this were refactored into a protocol abstraction that encapsulates the message-level codec and the framing strategy
	// into a single object. That would all us to have size-prefixed proto, json+see, connectRPC ... anything else.

	// codecName is the name the codec was looked up by. It selects the content type
	// of a request and, with the grpc-over-http protocol defined in this package,
	// also how streams are framed.
	//
	// We preserve this instead of using codec.Name() because this value is
	// already normalized and safe to compare, case-sensitive, with just ==.
	codecName string
	codec     encoding.CodecV2
}

func (o *channelOptions) setCodec(name string) error {
	codec, err := codecByName(name)
	if err != nil {
		return err
	}
	o.codecName, o.codec = name, codec
	return nil
}

func defaultChannelOptions() (channelOptions, error) {
	var opts channelOptions
	// Default to protobuf binary encoding.
	if err := opts.setCodec(grpcproto.Name); err != nil {
		return channelOptions{}, err
	}
	return opts, nil
}

// codecByName looks up a registered codec, reporting an unrecognized name as
// an error.
func codecByName(name string) (encoding.CodecV2, error) {
	codec := encoding.GetCodecV2(name)
	if codec == nil {
		return nil, fmt.Errorf("no codec registered for %q", name)
	}
	return codec, nil
}

// WithJSONEncoding configures the channel to use JSON encoding between the client and server.
// For unary calls, the request and response are JSON values. For streaming calls, the request is
// a series of JSON values and the response is SSE events containing JSON values.
func WithJSONEncoding(useJSONEncoding bool) ChannelOption {
	return func(o *channelOptions) error {
		name := grpcproto.Name
		if useJSONEncoding {
			name = jsonCodecName
		}
		// Resolved in both directions, so that a later WithJSONEncoding(false) undoes
		// an earlier WithJSONEncoding(true) rather than leaving JSON in place.
		return o.setCodec(name)
	}
}

// NewChannel creates a new Channel with the given base URL and transport, both of
// which are required. The ChannelOption functions can be used to configure the
// Channel. The error reports a channel that cannot be configured as asked.
func NewChannel(baseURL *url.URL, transport http.RoundTripper, opts ...ChannelOption) (*Channel, error) {
	if err := checkChannelParams(baseURL, transport); err != nil {
		return nil, err
	}
	chOpts, err := defaultChannelOptions()
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		if err := opt(&chOpts); err != nil {
			return nil, err
		}
	}
	return &Channel{
		BaseURL:   baseURL,
		Transport: transport,
		opts:      &chOpts,
	}, nil
}

func checkChannelParams(baseURL *url.URL, transport http.RoundTripper) error {
	if baseURL == nil {
		return errors.New("channel base URL is required")
	}
	if transport == nil {
		return errors.New("channel transport is required")
	}
	return nil
}

// Channel is used as a connection for GRPC requests issued over HTTP 1.1.
// Values should be created using the NewChannel constructor.
//
// For backwards compatibility, it is still allowed to construct the channel
// via a struct literal, as long as both Transport and BaseURL fields are set
// to non-nil values. Construction via struct literal produces a Channel with
// all default behavior; use of NewChannel is required to provide channel
// options.
//
// It implements version 1 of the GRPC-over-HTTP transport protocol defined
// in this package.
type Channel struct {
	Transport http.RoundTripper
	BaseURL   *url.URL
	// opts is nil when the channel was built as a struct literal, which is the
	// older and still supported form, and so carries all default behavior. A
	// non-nil value means NewChannel already validated the configuration.
	opts *channelOptions
}

var _ grpc.ClientConnInterface = (*Channel)(nil)

var grpcDetailsHeader = textproto.CanonicalMIMEHeaderKey("X-GRPC-Details")

// Invoke satisfies the grpchan.Channel interface and supports sending unary
// RPCs via the in-process channel.
func (ch *Channel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {
	chOpts, err := ch.channelOptions()
	if err != nil {
		return err
	}
	copts := internal.GetCallOptions(opts)

	reqUrl := *ch.BaseURL
	reqUrl.Path = path.Join(reqUrl.Path, methodName)
	reqUrlStr := reqUrl.String()
	ctx, err = internal.ApplyPerRPCCreds(ctx, copts, reqUrlStr, reqUrl.Scheme == "https")
	if err != nil {
		return err
	}

	codec := chOpts.codec
	h := getHeadersForClientUnaryRequest(ctx, chOpts)
	buf, err := codec.Marshal(req)
	if err != nil {
		return err
	}
	b := buf.Materialize()

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
		b, err = io.ReadAll(reply.Body)
		_ = reply.Body.Close()
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

	if stat := statFromResponse(reply, codec); stat.Code() != codes.OK {
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

	return codec.Unmarshal(mem.BufferSlice{mem.SliceBuffer(b)}, resp)
}

// NewStream satisfies the grpchan.Channel interface and supports sending
// streaming RPCs via the in-process channel.
func (ch *Channel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	chOpts, err := ch.channelOptions()
	if err != nil {
		return nil, err
	}
	copts := internal.GetCallOptions(opts)

	reqUrl := *ch.BaseURL
	reqUrl.Path = path.Join(reqUrl.Path, methodName)
	reqUrlStr := reqUrl.String()
	ctx, err = internal.ApplyPerRPCCreds(ctx, copts, reqUrlStr, reqUrl.Scheme == "https")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	h, newStreamWriter := getHeadersAndWriterForClientStreamingRequest(ctx, chOpts)

	// Intercept r.Close() so we can control the error sent across to the writer thread.
	r, w := io.Pipe()
	req, err := http.NewRequest("POST", reqUrlStr, io.NopCloser(r))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header = h

	// The details in X-GRPC-Details are encoded with the same codec as the body.
	cs := newClientStream(ctx, cancel, w, desc.ServerStreams, copts, ch.BaseURL, newStreamWriter(w), chOpts.codec)
	go cs.doHttpCall(ch.Transport, req, r)

	// ensure that context is cancelled, even if caller
	// fails to fully consume or cancel the stream
	ret := &clientStreamWrapper{cs}
	runtime.SetFinalizer(ret, func(*clientStreamWrapper) { cancel() })

	return ret, nil
}

func (ch *Channel) channelOptions() (channelOptions, error) {
	// These fields are exported and mutable, so we check them on every call.
	err := checkChannelParams(ch.BaseURL, ch.Transport)
	if err != nil {
		return channelOptions{}, err
	}
	if ch.opts != nil {
		// Configured and validated by NewChannel.
		return *ch.opts, nil
	}
	return defaultChannelOptions()
}

type clientStreamWrapper struct {
	grpc.ClientStream
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

	streamWriter streamWriter

	// detailsCodec unmarshals X-GRPC-Details when a streaming RPC fails before the body
	// uses the negotiated unary-style encoding (same as Channel.WithJSONEncoding).
	detailsCodec encoding.CodecV2

	// respStream is set to indicate whether client expects stream response; unary if false
	respStream bool

	// hd and hdErr are populated when ready is done
	ready sync.WaitGroup
	hdErr error
	hd    metadata.MD

	// rCh is used to deliver messages from doHttpCall goroutine
	// to callers of RecvMsg.
	// done must be set to true before it is closed
	rCh chan streamMsg

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

func newClientStream(
	ctx context.Context,
	cancel context.CancelFunc,
	w io.WriteCloser,
	recvStream bool,
	copts *internal.CallOptions,
	baseUrl *url.URL,
	streamWriter streamWriter,
	detailsCodec encoding.CodecV2,
) *clientStream {
	cs := &clientStream{
		ctx:          ctx,
		cancel:       cancel,
		copts:        copts,
		baseUrl:      baseUrl,
		streamWriter: streamWriter,
		detailsCodec: detailsCodec,
		w:            w,
		respStream:   recvStream,
		rCh:          make(chan streamMsg),
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

	cs.wErr = cs.streamWriter(m, false)
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
		err := msg.Decode(m)
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
func (cs *clientStream) doHttpCall(transport http.RoundTripper, req *http.Request, readPipe *io.PipeReader) {
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
		readPipe.CloseWithError(rErr)
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
	defer func() {
		_, _ = io.ReadAll(reply.Body)
		_ = reply.Body.Close()
	}()

	if len(cs.copts.Peer) > 0 {
		cs.copts.SetPeer(getPeer(cs.baseUrl, reply.TLS))
	}
	md, err := asMetadata(reply.Header)
	if err != nil {
		onReady(err, nil)
		return
	}

	onReady(nil, md)

	stat := statFromResponse(reply, cs.detailsCodec)
	if stat.Code() != codes.OK {
		statProto := stat.Proto()
		cs.tr.Code = statProto.Code
		cs.tr.Message = statProto.Message
		cs.tr.Details = statProto.Details
		return
	}

	contentType := reply.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	streamReader := getClientStreamReader(mediaType, reply.Body)

	if streamReader == nil {
		onReady(status.Error(codes.Internal, fmt.Sprintf("unsupported media type: %s", mediaType)), nil)
		return
	}

	counter := 0
	for {
		// TODO: enforce max send and receive size in call options

		counter++
		var msg streamMsg
		msg, rErr = streamReader()
		if rErr != nil {
			if rErr == io.EOF {
				rErr = io.ErrUnexpectedEOF
			}
			return
		}
		if msg.isTrailer {
			// final message is a trailer (need lock to write to cs.tr)
			cs.rMu.Lock()
			rMuHeld = true // defer above will unlock for us
			cs.rErr = msg.Decode(&cs.tr)
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

func statFromResponse(reply *http.Response, detailsCodec encoding.CodecV2) *status.Status {
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
		var details []*anypb.Any
		if detailHeaders := reply.Header[grpcDetailsHeader]; len(detailHeaders) > 0 {
			details = make([]*anypb.Any, 0, len(detailHeaders))
			for _, d := range detailHeaders {
				b, err := base64.RawURLEncoding.DecodeString(d)
				if err != nil {
					continue
				}
				msg := new(anypb.Any)
				if err := detailsCodec.Unmarshal(mem.BufferSlice{mem.SliceBuffer(b)}, msg); err != nil {
					continue
				}
				details = append(details, msg)
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
