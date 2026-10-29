package lsp

import (
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type protocolCodec struct{}

func (protocolCodec) Marshal(value any) ([]byte, error) {
	switch value.(type) {
	case jsonrpc2.RawMessage, *jsonrpc2.RawMessage, json.RawMessage, *json.RawMessage:
		return (jsonrpc2.JSONCodec{}).Marshal(value)
	}
	return protocol.Marshal(value)
}

func (protocolCodec) Unmarshal(data []byte, value any) error {
	switch value.(type) {
	case *jsonrpc2.RawMessage, *json.RawMessage:
		return (jsonrpc2.JSONCodec{}).Unmarshal(data, value)
	}
	return protocol.Unmarshal(data, value)
}
