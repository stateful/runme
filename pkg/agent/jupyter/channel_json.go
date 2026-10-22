package jupyter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrBinaryBuffersUnsupported is returned explicitly instead of silently
// dropping Jupyter comm/widget buffers that the current browser client cannot
// consume over its legacy JSON WebSocket protocol.
var ErrBinaryBuffersUnsupported = errors.New("jupyter binary buffers require the binary WebSocket subprotocol")

// ParseChannelJSON converts the existing browser-facing Jupyter JSON shape to
// a wire message while preserving all fields inside the four protocol objects.
func ParseChannelJSON(payload []byte, limits Limits) (Channel, Message, error) {
	limits = limits.normalized()
	if len(payload) == 0 {
		return "", Message{}, errors.New("empty Jupyter WebSocket message")
	}
	if len(payload) > limits.MaxMessageBytes {
		return "", Message{}, fmt.Errorf("jupyter WebSocket payload exceeds limit %d", limits.MaxMessageBytes)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		return "", Message{}, fmt.Errorf("parse Jupyter WebSocket JSON: %w", err)
	}
	if object == nil {
		return "", Message{}, errors.New("jupyter WebSocket message must be an object")
	}

	var channel Channel
	if err := json.Unmarshal(object["channel"], &channel); err != nil || channel == "" {
		return "", Message{}, errors.New("jupyter WebSocket message requires a channel")
	}
	switch channel {
	case ChannelShell, ChannelControl, ChannelStdin:
	case ChannelIOPub:
		return "", Message{}, errors.New("browser messages cannot target Jupyter IOPub")
	default:
		return "", Message{}, fmt.Errorf("unsupported browser Jupyter channel %q", channel)
	}

	if rawBuffers, ok := object["buffers"]; ok && len(bytes.TrimSpace(rawBuffers)) > 0 && !bytes.Equal(bytes.TrimSpace(rawBuffers), []byte("null")) {
		var buffers []json.RawMessage
		if err := json.Unmarshal(rawBuffers, &buffers); err != nil {
			return "", Message{}, errors.New("invalid Jupyter WebSocket buffers")
		}
		if len(buffers) > 0 {
			return "", Message{}, ErrBinaryBuffersUnsupported
		}
	}

	message := Message{
		Header:       bytes.Clone(object["header"]),
		ParentHeader: bytes.Clone(object["parent_header"]),
		Metadata:     bytes.Clone(object["metadata"]),
		Content:      bytes.Clone(object["content"]),
	}
	for name, raw := range map[string][]byte{
		"header":        message.Header,
		"parent_header": message.ParentHeader,
		"metadata":      message.Metadata,
		"content":       message.Content,
	} {
		if err := validateJSONObject(raw); err != nil {
			return "", Message{}, fmt.Errorf("invalid browser Jupyter %s: %w", name, err)
		}
	}
	return channel, message, nil
}

// MarshalChannelJSON converts an authenticated wire message to the legacy
// Jupyter Server JSON WebSocket shape already consumed by Runme Web.
func MarshalChannelJSON(channel Channel, message Message, limits Limits) ([]byte, error) {
	if channel != ChannelShell && channel != ChannelControl && channel != ChannelStdin && channel != ChannelIOPub {
		return nil, fmt.Errorf("unsupported kernel Jupyter channel %q", channel)
	}
	if len(message.Buffers) > 0 {
		return nil, ErrBinaryBuffersUnsupported
	}
	for name, raw := range map[string][]byte{
		"header":        message.Header,
		"parent_header": message.ParentHeader,
		"metadata":      message.Metadata,
		"content":       message.Content,
	} {
		if err := validateJSONObject(raw); err != nil {
			return nil, fmt.Errorf("invalid kernel Jupyter %s: %w", name, err)
		}
	}
	var header struct {
		MessageID   string `json:"msg_id"`
		MessageType string `json:"msg_type"`
	}
	if err := json.Unmarshal(message.Header, &header); err != nil {
		return nil, fmt.Errorf("parse kernel Jupyter header: %w", err)
	}
	object := map[string]any{
		"channel":       channel,
		"header":        json.RawMessage(message.Header),
		"parent_header": json.RawMessage(message.ParentHeader),
		"metadata":      json.RawMessage(message.Metadata),
		"content":       json.RawMessage(message.Content),
		"buffers":       []any{},
		"msg_id":        header.MessageID,
		"msg_type":      header.MessageType,
	}
	payload, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("marshal Jupyter WebSocket JSON: %w", err)
	}
	limits = limits.normalized()
	if len(payload) > limits.MaxMessageBytes {
		return nil, fmt.Errorf("jupyter WebSocket payload exceeds limit %d", limits.MaxMessageBytes)
	}
	return payload, nil
}
