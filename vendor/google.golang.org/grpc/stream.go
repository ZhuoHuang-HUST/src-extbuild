/*
 *
 * Copyright 2014, Google Inc.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *     * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *     * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

package grpc

import (
	"bytes"
	"errors"
	"io"
	"math"
	"sync"
	"time"
   
    "fmt"
    "os"
    "log"

	"golang.org/x/net/context"
	"golang.org/x/net/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/transport"
)

// StreamHandler defines the handler called by gRPC server to complete the
// execution of a streaming RPC.
type StreamHandler func(srv interface{}, stream ServerStream) error

// StreamDesc represents a streaming RPC service's method specification.
type StreamDesc struct {
	StreamName string
	Handler    StreamHandler

	// At least one of these is true.
	ServerStreams bool
	ClientStreams bool
}

// Stream defines the common interface a client or server stream has to satisfy.
type Stream interface {
	// Context returns the context for this stream.
	Context() context.Context
	// SendMsg blocks until it sends m, the stream is done or the stream
	// breaks.
	// On error, it aborts the stream and returns an RPC status on client
	// side. On server side, it simply returns the error to the caller.
	// SendMsg is called by generated code. Also Users can call SendMsg
	// directly when it is really needed in their use cases.
	SendMsg(m interface{}) error
	// RecvMsg blocks until it receives a message or the stream is
	// done. On client side, it returns io.EOF when the stream is done. On
	// any other error, it aborts the stream and returns an RPC status. On
	// server side, it simply returns the error to the caller.
	RecvMsg(m interface{}) error
}

// ClientStream defines the interface a client stream has to satisfy.
type ClientStream interface {
	// Header returns the header metadata received from the server if there
	// is any. It blocks if the metadata is not ready to read.
	Header() (metadata.MD, error)
	// Trailer returns the trailer metadata from the server, if there is any.
	// It must only be called after stream.CloseAndRecv has returned, or
	// stream.Recv has returned a non-nil error (including io.EOF).
	Trailer() metadata.MD
	// CloseSend closes the send direction of the stream. It closes the stream
	// when non-nil error is met.
	CloseSend() error
	Stream
}

// NewClientStream creates a new Stream for the client side. This is called
// by generated code.
func NewClientStream(ctx context.Context, desc *StreamDesc, cc *ClientConn, method string, opts ...CallOption) (ClientStream, error) {
	if cc.dopts.streamInt != nil {
		return cc.dopts.streamInt(ctx, desc, cc, method, newClientStream, opts...)
	}
	return newClientStream(ctx, desc, cc, method, opts...)
}

func newClientStream(ctx context.Context, desc *StreamDesc, cc *ClientConn, method string, opts ...CallOption) (_ ClientStream, err error) {
	var (
		t   transport.ClientTransport
		s   *transport.Stream
		put func()
	)
	c := defaultCallInfo
	for _, o := range opts {
		if err := o.before(&c); err != nil {
			return nil, toRPCErr(err)
		}
	}
	callHdr := &transport.CallHdr{
		Host:   cc.authority,
		Method: method,
		Flush:  desc.ServerStreams && desc.ClientStreams,
	}
	if cc.dopts.cp != nil {
		callHdr.SendCompress = cc.dopts.cp.Type()
	}
	var trInfo traceInfo
	if EnableTracing {
		trInfo.tr = trace.New("grpc.Sent."+methodFamily(method), method)
		trInfo.firstLine.client = true
		if deadline, ok := ctx.Deadline(); ok {
			trInfo.firstLine.deadline = deadline.Sub(time.Now())
		}
		trInfo.tr.LazyLog(&trInfo.firstLine, false)
		ctx = trace.NewContext(ctx, trInfo.tr)
		defer func() {
			if err != nil {
				// Need to call tr.finish() if error is returned.
				// Because tr will not be returned to caller.
				trInfo.tr.LazyPrintf("RPC: [%v]", err)
				trInfo.tr.SetError()
				trInfo.tr.Finish()
			}
		}()
	}
	gopts := BalancerGetOptions{
		BlockingWait: !c.failFast,
	}
	for {
		t, put, err = cc.getTransport(ctx, gopts)
		if err != nil {
			// TODO(zhaoq): Probably revisit the error handling.
			if _, ok := err.(*rpcError); ok {
				return nil, err
			}
			if err == errConnClosing || err == errConnUnavailable {
				if c.failFast {
					return nil, Errorf(codes.Unavailable, "%v", err)
				}
				continue
			}
			// All the other errors are treated as Internal errors.
			return nil, Errorf(codes.Internal, "%v", err)
		}

        //fmt.Println("vendor/google.golang.org/grpc/stream.go  newClientStream()")
        //fmt.Println("vendor/google.golang.org/grpc/stream.go  newClientStream() NewStream")
		s, err = t.NewStream(ctx, callHdr)
		if err != nil {
			if put != nil {
				put()
				put = nil
			}
			if _, ok := err.(transport.ConnectionError); ok || err == transport.ErrStreamDrain {
				if c.failFast {
					return nil, toRPCErr(err)
				}
				continue
			}
			return nil, toRPCErr(err)
		}
		break
	}
	cs := &clientStream{
		opts:  opts,
		c:     c,
		desc:  desc,
		codec: cc.dopts.codec,
		cp:    cc.dopts.cp,
		dc:    cc.dopts.dc,

		put: put,
		t:   t,
		s:   s,
		p:   &parser{r: s},

		tracing: EnableTracing,
		trInfo:  trInfo,
	}
	if cc.dopts.cp != nil {
		cs.cbuf = new(bytes.Buffer)
	}
	// Listen on ctx.Done() to detect cancellation and s.Done() to detect normal termination
	// when there is no pending I/O operations on this stream.
	go func() {
		select {
		case <-t.Error():
			// Incur transport error, simply exit.
		case <-s.Done():
			// TODO: The trace of the RPC is terminated here when there is no pending
			// I/O, which is probably not the optimal solution.
			if s.StatusCode() == codes.OK {
				cs.finish(nil)
			} else {
				cs.finish(Errorf(s.StatusCode(), "%s", s.StatusDesc()))
			}
			cs.closeTransportStream(nil)
		case <-s.GoAway():
			cs.finish(errConnDrain)
			cs.closeTransportStream(errConnDrain)
		case <-s.Context().Done():
			err := s.Context().Err()
			cs.finish(err)
			cs.closeTransportStream(transport.ContextErr(err))
		}
	}()
	return cs, nil
}

// clientStream implements a client side Stream.
type clientStream struct {
	opts  []CallOption
	c     callInfo
	t     transport.ClientTransport
	s     *transport.Stream
	p     *parser
	desc  *StreamDesc
	codec Codec
	cp    Compressor
	cbuf  *bytes.Buffer
	dc    Decompressor

	tracing bool // set to EnableTracing when the clientStream is created.

	mu     sync.Mutex
	put    func()
	closed bool
	// trInfo.tr is set when the clientStream is created (if EnableTracing is true),
	// and is set to nil when the clientStream's finish method is called.
	trInfo traceInfo
}

func (cs *clientStream) Context() context.Context {
	return cs.s.Context()
}

func (cs *clientStream) Header() (metadata.MD, error) {
	m, err := cs.s.Header()
	if err != nil {
		if _, ok := err.(transport.ConnectionError); !ok {
			cs.closeTransportStream(err)
		}
	}
	return m, err
}

func (cs *clientStream) Trailer() metadata.MD {
	return cs.s.Trailer()
}

func (cs *clientStream) SendMsg(m interface{}) (err error) {
	if cs.tracing {
		cs.mu.Lock()
		if cs.trInfo.tr != nil {
			cs.trInfo.tr.LazyLog(&payload{sent: true, msg: m}, true)
		}
		cs.mu.Unlock()
	}
	defer func() {
		if err != nil {
			cs.finish(err)
		}
		if err == nil {
			return
		}
		if err == io.EOF {
			// Specialize the process for server streaming. SendMesg is only called
			// once when creating the stream object. io.EOF needs to be skipped when
			// the rpc is early finished (before the stream object is created.).
			// TODO: It is probably better to move this into the generated code.
			if !cs.desc.ClientStreams && cs.desc.ServerStreams {
				err = nil
			}
			return
		}
		if _, ok := err.(transport.ConnectionError); !ok {
			cs.closeTransportStream(err)
		}
		err = toRPCErr(err)
	}()
	out, err := encode(cs.codec, m, cs.cp, cs.cbuf)
	defer func() {
		if cs.cbuf != nil {
			cs.cbuf.Reset()
		}
	}()
	if err != nil {
		return Errorf(codes.Internal, "grpc: %v", err)
	}
	return cs.t.Write(cs.s, out, &transport.Options{Last: false})
}

func (cs *clientStream) RecvMsg(m interface{}) (err error) {
    //fmt.Println("vendor/google/grpc/stream.go  RecvMsg() ")
	err = recv(cs.p, cs.codec, cs.s, cs.dc, m, math.MaxInt32)
	defer func() {
		// err != nil indicates the termination of the stream.
		if err != nil {
			cs.finish(err)
		}
	}()
	if err == nil {
		if cs.tracing {
			cs.mu.Lock()
			if cs.trInfo.tr != nil {
				cs.trInfo.tr.LazyLog(&payload{sent: false, msg: m}, true)
			}
			cs.mu.Unlock()
		}
		if !cs.desc.ClientStreams || cs.desc.ServerStreams {
			return
		}
		// Special handling for client streaming rpc.
		err = recv(cs.p, cs.codec, cs.s, cs.dc, m, math.MaxInt32)
		cs.closeTransportStream(err)
		if err == nil {
			return toRPCErr(errors.New("grpc: client streaming protocol violation: get <nil>, want <EOF>"))
		}
		if err == io.EOF {
			if cs.s.StatusCode() == codes.OK {
				cs.finish(err)
				return nil
			}
			return Errorf(cs.s.StatusCode(), "%s", cs.s.StatusDesc())
		}
		return toRPCErr(err)
	}
	if _, ok := err.(transport.ConnectionError); !ok {
		cs.closeTransportStream(err)
	}
	if err == io.EOF {
		if cs.s.StatusCode() == codes.OK {
			// Returns io.EOF to indicate the end of the stream.
			return
		}
		return Errorf(cs.s.StatusCode(), "%s", cs.s.StatusDesc())
	}
	return toRPCErr(err)
}

func (cs *clientStream) CloseSend() (err error) {
	err = cs.t.Write(cs.s, nil, &transport.Options{Last: true})
	defer func() {
		if err != nil {
			cs.finish(err)
		}
	}()
	if err == nil || err == io.EOF {
		return nil
	}
	if _, ok := err.(transport.ConnectionError); !ok {
		cs.closeTransportStream(err)
	}
	err = toRPCErr(err)
	return
}

func (cs *clientStream) closeTransportStream(err error) {
	cs.mu.Lock()
	if cs.closed {
		cs.mu.Unlock()
		return
	}
	cs.closed = true
	cs.mu.Unlock()
	cs.t.CloseStream(cs.s, err)
}

func (cs *clientStream) finish(err error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, o := range cs.opts {
		o.after(&cs.c)
	}
	if cs.put != nil {
		cs.put()
		cs.put = nil
	}
	if !cs.tracing {
		return
	}
	if cs.trInfo.tr != nil {
		if err == nil || err == io.EOF {
			cs.trInfo.tr.LazyPrintf("RPC: [OK]")
		} else {
			cs.trInfo.tr.LazyPrintf("RPC: [%v]", err)
			cs.trInfo.tr.SetError()
		}
		cs.trInfo.tr.Finish()
		cs.trInfo.tr = nil
	}
}

// ServerStream defines the interface a server stream has to satisfy.
type ServerStream interface {
	// SendHeader sends the header metadata. It should not be called
	// after SendProto. It fails if called multiple times or if
	// called after SendProto.
	SendHeader(metadata.MD) error
	// SetTrailer sets the trailer metadata which will be sent with the RPC status.
	// When called more than once, all the provided metadata will be merged.
	SetTrailer(metadata.MD)
	Stream
}

// serverStream implements a server side Stream.
type serverStream struct {
	t          transport.ServerTransport
	s          *transport.Stream
	p          *parser
	codec      Codec
	cp         Compressor
	dc         Decompressor
	cbuf       *bytes.Buffer
	maxMsgSize int
	statusCode codes.Code
	statusDesc string
	trInfo     *traceInfo

	mu sync.Mutex // protects trInfo.tr after the service handler runs.
}

func (ss *serverStream) Context() context.Context {
	return ss.s.Context()
}

func (ss *serverStream) SendHeader(md metadata.MD) error {
    fmt.Println("vendor/google/grpc/stream.go  SendHeader()")
    logPrintStream("SendHeader()")
	return ss.t.WriteHeader(ss.s, md)
}

func (ss *serverStream) SetTrailer(md metadata.MD) {
	if md.Len() == 0 {
		return
	}
	ss.s.SetTrailer(md)
	return
}

func (ss *serverStream) SendMsg(m interface{}) (err error) {
    fmt.Println("vendor/google/grpc/stream.go  SendMsg()")
    logPrintStream("SendMsg()")
	defer func() {
		if ss.trInfo != nil {
			ss.mu.Lock()
			if ss.trInfo.tr != nil {
				if err == nil {
					ss.trInfo.tr.LazyLog(&payload{sent: true, msg: m}, true)
				} else {
					ss.trInfo.tr.LazyLog(&fmtStringer{"%v", []interface{}{err}}, true)
					ss.trInfo.tr.SetError()
				}
			}
			ss.mu.Unlock()
		}
	}()
	out, err := encode(ss.codec, m, ss.cp, ss.cbuf)
	defer func() {
		if ss.cbuf != nil {
			ss.cbuf.Reset()
		}
	}()
	if err != nil {
		err = Errorf(codes.Internal, "grpc: %v", err)
		return err
	}
	if err := ss.t.Write(ss.s, out, &transport.Options{Last: false}); err != nil {
		return toRPCErr(err)
	}
	return nil
}

func (ss *serverStream) RecvMsg(m interface{}) (err error) {
    logPrintStream("RecvMsg()")
	defer func() {
		if ss.trInfo != nil {
			ss.mu.Lock()
			if ss.trInfo.tr != nil {
				if err == nil {
					ss.trInfo.tr.LazyLog(&payload{sent: false, msg: m}, true)
				} else if err != io.EOF {
					ss.trInfo.tr.LazyLog(&fmtStringer{"%v", []interface{}{err}}, true)
					ss.trInfo.tr.SetError()
				}
			}
			ss.mu.Unlock()
		}
	}()
	if err := recv(ss.p, ss.codec, ss.s, ss.dc, m, ss.maxMsgSize); err != nil {
		if err == io.EOF {
			return err
		}
		if err == io.ErrUnexpectedEOF {
			err = Errorf(codes.Internal, io.ErrUnexpectedEOF.Error())
		}
		return toRPCErr(err)
	}
	return nil
}



func logPrintStream(errStr string) {
    logFile, logError := os.Open("/home/vagrant/logStream.md")
    if logError != nil {
        logFile, _ = os.Create("/home/vagrant/logStream.md")
    }
    defer logFile.Close()

    debugLog := log.New(logFile, "[Debug]", log.Llongfile)
    debugLog.Println(errStr)
}
