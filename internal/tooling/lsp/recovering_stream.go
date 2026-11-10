package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"go.lsp.dev/jsonrpc2"
)

const maxFrameBytes = 256 << 20

// frameIdle bounds the wait for the rest of a body once it has started
// arriving: a Content-Length larger than the bytes the client actually sends
// is otherwise indistinguishable from a client that never finishes, and
// holding the connection open would answer the frame only after a hang-up.
const frameIdle = 500 * time.Millisecond

var (
	errBadHeader   = errors.New("header block without a usable Content-Length")
	errShortBody   = errors.New("body ended before its Content-Length")
	errReadTimeout = errors.New("read timed out")
)

// recoveringStream answers a malformed frame with an error and resumes at the next Content-Length header.
type recoveringStream struct {
	jsonrpc2.Stream
	in        *byteStream
	resyncing bool
}

func newRecoveringStream(rwc io.ReadWriteCloser) jsonrpc2.Stream {
	return &recoveringStream{Stream: jsonrpc2.NewStream(rwc), in: newByteStream(rwc)}
}

// byteStream reads one source in a goroutine, so a body that stalls can be
// abandoned without losing the bytes that arrive after it.
type byteStream struct {
	chunks chan readChunk
	buf    []byte
	err    error
}

type readChunk struct {
	data []byte
	err  error
}

func newByteStream(r io.Reader) *byteStream {
	b := &byteStream{chunks: make(chan readChunk, 16)}
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				b.chunks <- readChunk{data: append([]byte(nil), buf[:n]...)}
			}
			if err != nil {
				b.chunks <- readChunk{err: err}
				close(b.chunks)
				return
			}
		}
	}()
	return b
}

// fill waits for another chunk; timeout 0 waits forever.
func (b *byteStream) fill(timeout time.Duration) error {
	if b.err != nil {
		return b.err
	}
	var expired <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		expired = timer.C
	}
	select {
	case c, ok := <-b.chunks:
		if !ok {
			b.err = io.EOF
			return b.err
		}
		b.buf = append(b.buf, c.data...)
		if c.err != nil {
			b.err = c.err
		}
		return b.err
	case <-expired:
		return errReadTimeout
	}
}

// readLine returns one newline-terminated line, newline included.
func (b *byteStream) readLine() ([]byte, error) {
	for {
		if i := bytes.IndexByte(b.buf, '\n'); i >= 0 {
			line := append([]byte(nil), b.buf[:i+1]...)
			b.buf = b.buf[i+1:]
			return line, nil
		}
		if err := b.fill(0); err != nil {
			return nil, err
		}
	}
}

// readN returns n bytes, or errShortBody when the body starts but then idles.
func (b *byteStream) readN(n int) ([]byte, error) {
	out := make([]byte, 0, min(n, 1<<20))
	for len(out) < n {
		if len(b.buf) > 0 {
			take := min(len(b.buf), n-len(out))
			out = append(out, b.buf[:take]...)
			b.buf = b.buf[take:]
			continue
		}
		timeout := time.Duration(0)
		if len(out) > 0 {
			timeout = frameIdle
		}
		if err := b.fill(timeout); err != nil {
			switch {
			case err == errReadTimeout && len(out) > 0:
				return out, errShortBody
			case err == io.EOF:
				return out, io.ErrUnexpectedEOF
			default:
				return out, err
			}
		}
	}
	return out, nil
}

func (s *recoveringStream) Read(ctx context.Context) (jsonrpc2.Message, int64, error) {
	body, n, err := s.ReadFrame(ctx)
	if err != nil {
		return nil, 0, err
	}
	msg, err := jsonrpc2.DecodeMessage(body)
	return msg, n, err
}

// ReadFrame is implemented so the library keeps its batch path.
func (s *recoveringStream) ReadFrame(ctx context.Context) ([]byte, int64, error) {
	for {
		body, n, err := s.readFrame(ctx)
		switch {
		case err == io.EOF:
			return nil, 0, io.EOF
		case err == io.ErrUnexpectedEOF:
			s.reject(ctx, jsonrpc2.ErrParse)
			return nil, 0, io.EOF
		case err == errBadHeader:
			if !s.resyncing {
				s.reject(ctx, jsonrpc2.ErrParse)
			}
			s.resyncing = true
			continue
		case err == errShortBody:
			s.reject(ctx, jsonrpc2.ErrParse)
			s.resyncing = true
			continue
		case err != nil:
			return nil, 0, err
		}
		body, e := screen(body)
		if e != nil {
			s.reject(ctx, e)
			continue
		}
		s.resyncing = false
		return body, n, nil
	}
}

func (s *recoveringStream) WriteFrame(ctx context.Context, data []byte) (int64, error) {
	if w, ok := s.Stream.(interface {
		WriteFrame(context.Context, []byte) (int64, error)
	}); ok {
		return w.WriteFrame(ctx, data)
	}
	return 0, errors.New("lsp: stream cannot write raw frames")
}

// screen returns the body to dispatch, or the error to answer with instead; bad batch members are replaced by an empty object, which the library answers with -32600.
func screen(body []byte) ([]byte, *jsonrpc2.Error) {
	if !json.Valid(body) {
		return nil, jsonrpc2.ErrParse
	}
	if t := bytes.TrimSpace(body); len(t) > 0 && t[0] == '[' {
		var members []json.RawMessage
		if json.Unmarshal(t, &members) != nil {
			return nil, jsonrpc2.ErrInvalidRequest
		}
		for i, m := range members {
			if !isObject(m) || nullID(m) {
				members[i] = json.RawMessage("{}")
			}
		}
		out, err := json.Marshal(members)
		if err != nil {
			return nil, jsonrpc2.ErrInvalidRequest
		}
		return out, nil
	}
	if nullID(body) {
		return nil, jsonrpc2.ErrInvalidRequest
	}
	if _, err := jsonrpc2.DecodeMessage(body); err != nil {
		return nil, jsonrpc2.ErrInvalidRequest
	}
	return body, nil
}

func isObject(m json.RawMessage) bool {
	t := bytes.TrimSpace(m)
	return len(t) > 0 && t[0] == '{'
}

// nullID: a request whose id is null is not a notification, and LSP ids are integers or strings.
func nullID(m []byte) bool {
	var f struct {
		ID     json.RawMessage `json:"id"`
		Method json.RawMessage `json:"method"`
	}
	return json.Unmarshal(m, &f) == nil && f.Method != nil && string(f.ID) == "null"
}

func (s *recoveringStream) reject(ctx context.Context, e *jsonrpc2.Error) {
	s.Stream.Write(ctx, jsonrpc2.NewResponse(jsonrpc2.ID{}, nil, e))
}

func (s *recoveringStream) readFrame(ctx context.Context) ([]byte, int64, error) {
	length, seen, junk := 0, false, false
	var consumed int64
	for {
		line, err := s.in.readLine()
		consumed += int64(len(line))
		if err != nil {
			if len(line) == 0 && !seen && !junk && err == io.EOF {
				return nil, 0, io.EOF
			}
			return nil, 0, io.ErrUnexpectedEOF
		}
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(trimmed) == 0 {
			if seen {
				break
			}
			if junk {
				return nil, 0, errBadHeader
			}
			continue
		}
		if n, ok := contentLength(trimmed); ok {
			length, seen = n, n > 0 && n <= maxFrameBytes
			junk = junk || !seen
		} else {
			junk = true
		}
	}
	body, err := s.in.readN(length)
	if err != nil {
		return nil, 0, err
	}
	return body, consumed + int64(length), nil
}

// contentLength finds the header anywhere in the line so that bytes left over from a mis-sized frame do not hide the next header.
func contentLength(line []byte) (int, bool) {
	i := bytes.Index(bytes.ToLower(line), []byte("content-length:"))
	if i < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(string(bytes.TrimSpace(line[i+len("content-length:"):])))
	if err != nil {
		return 0, true
	}
	return n, true
}
