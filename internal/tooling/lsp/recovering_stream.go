package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"go.lsp.dev/jsonrpc2"
)

const maxFrameBytes = 256 << 20

var errBadHeader = errors.New("header block without a usable Content-Length")

// recoveringStream answers a malformed frame with an error and resumes at the next Content-Length header.
type recoveringStream struct {
	jsonrpc2.Stream
	in        *bufio.Reader
	resyncing bool
}

func newRecoveringStream(rwc io.ReadWriteCloser) jsonrpc2.Stream {
	return &recoveringStream{Stream: jsonrpc2.NewStream(rwc), in: bufio.NewReader(rwc)}
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
		line, err := s.in.ReadBytes('\n')
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
	body := make([]byte, 0, min(length, 1<<20))
	buf := bytes.NewBuffer(body)
	if _, err := io.CopyN(buf, s.in, int64(length)); err != nil {
		return nil, 0, io.ErrUnexpectedEOF
	}
	body = buf.Bytes()
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
