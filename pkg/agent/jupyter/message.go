package jupyter

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// SignatureSchemeHMACSHA256 is the only signing scheme supported by the
	// Jupyter messaging specification and the direct bridge.
	SignatureSchemeHMACSHA256 = "hmac-sha256"

	defaultMaxFrameCount   = 64
	defaultMaxFrameBytes   = 16 * 1024 * 1024
	defaultMaxMessageBytes = 32 * 1024 * 1024
)

var delimiter = []byte("<IDS|MSG>")

// Limits bounds untrusted multipart traffic. Zero values select conservative
// defaults.
type Limits struct {
	MaxFrameCount   int
	MaxFrameBytes   int
	MaxMessageBytes int
}

func (l Limits) normalized() Limits {
	if l.MaxFrameCount <= 0 {
		l.MaxFrameCount = defaultMaxFrameCount
	}
	if l.MaxFrameBytes <= 0 {
		l.MaxFrameBytes = defaultMaxFrameBytes
	}
	if l.MaxMessageBytes <= 0 {
		l.MaxMessageBytes = defaultMaxMessageBytes
	}
	return l
}

// Message is a Jupyter wire message. JSON dictionaries remain raw so unknown
// fields and message types survive a bridge round trip unchanged.
type Message struct {
	RoutingPrefix [][]byte
	Signature     []byte
	Header        json.RawMessage
	ParentHeader  json.RawMessage
	Metadata      json.RawMessage
	Content       json.RawMessage
	Buffers       [][]byte
}

// Signer signs and verifies the four JSON frames in a Jupyter wire message.
type Signer struct {
	key []byte
}

// NewSigner constructs a signer for a validated Jupyter signature scheme. An
// empty key is valid and represents authentication-disabled Jupyter messages.
func NewSigner(key, signatureScheme string) (*Signer, error) {
	if signatureScheme != SignatureSchemeHMACSHA256 {
		return nil, fmt.Errorf("unsupported Jupyter signature scheme %q", signatureScheme)
	}
	return &Signer{key: []byte(key)}, nil
}

// Sign returns the lowercase hexadecimal HMAC for the four serialized JSON
// frames. It returns an empty signature when the connection key is empty.
func (s *Signer) Sign(parts ...[]byte) []byte {
	if s == nil || len(s.key) == 0 {
		return []byte{}
	}
	mac := hmac.New(sha256.New, s.key)
	for _, part := range parts {
		_, _ = mac.Write(part)
	}
	encoded := make([]byte, hex.EncodedLen(sha256.Size))
	hex.Encode(encoded, mac.Sum(nil))
	return encoded
}

// Verify checks a wire signature without timing-sensitive byte comparison.
func (s *Signer) Verify(signature []byte, parts ...[]byte) error {
	if s == nil {
		return errors.New("jupyter signer is required")
	}
	if len(s.key) == 0 {
		if len(signature) != 0 {
			return errors.New("unexpected signature for empty Jupyter key")
		}
		return nil
	}
	if len(signature) != hex.EncodedLen(sha256.Size) {
		return errors.New("invalid Jupyter signature length")
	}
	provided := make([]byte, sha256.Size)
	if _, err := hex.Decode(provided, signature); err != nil {
		return errors.New("invalid Jupyter signature encoding")
	}

	mac := hmac.New(sha256.New, s.key)
	for _, part := range parts {
		_, _ = mac.Write(part)
	}
	if subtle.ConstantTimeCompare(provided, mac.Sum(nil)) != 1 {
		return errors.New("invalid Jupyter message signature")
	}
	return nil
}

// ParseMultipart validates and decodes a Jupyter ZeroMQ multipart message. It
// searches for the delimiter because ROUTER and IOPub prefixes are variable.
func ParseMultipart(frames [][]byte, signer *Signer, limits Limits) (Message, error) {
	if err := validateMultipartLimits(frames, limits); err != nil {
		return Message{}, err
	}

	delimiterIndex := -1
	for i, frame := range frames {
		if bytes.Equal(frame, delimiter) {
			delimiterIndex = i
			break
		}
	}
	if delimiterIndex < 0 {
		return Message{}, errors.New("jupyter message delimiter not found")
	}
	if len(frames)-delimiterIndex-1 < 5 {
		return Message{}, errors.New("jupyter message is missing signed JSON frames")
	}

	signature := frames[delimiterIndex+1]
	header := frames[delimiterIndex+2]
	parentHeader := frames[delimiterIndex+3]
	metadata := frames[delimiterIndex+4]
	content := frames[delimiterIndex+5]
	// Authenticate exact bytes before attempting to parse attacker-controlled
	// JSON. Callers can therefore rely on ParseMultipart never exposing an
	// unauthenticated logical message.
	if err := signer.Verify(signature, header, parentHeader, metadata, content); err != nil {
		return Message{}, err
	}
	for name, raw := range map[string][]byte{
		"header":        header,
		"parent_header": parentHeader,
		"metadata":      metadata,
		"content":       content,
	} {
		if err := validateJSONObject(raw); err != nil {
			return Message{}, fmt.Errorf("invalid Jupyter %s: %w", name, err)
		}
	}

	return Message{
		RoutingPrefix: cloneFrames(frames[:delimiterIndex]),
		Signature:     bytes.Clone(signature),
		Header:        bytes.Clone(header),
		ParentHeader:  bytes.Clone(parentHeader),
		Metadata:      bytes.Clone(metadata),
		Content:       bytes.Clone(content),
		Buffers:       cloneFrames(frames[delimiterIndex+6:]),
	}, nil
}

// MarshalMultipart validates and serializes a Jupyter message into ZeroMQ
// multipart frames, recomputing the signature over the exact JSON bytes.
func MarshalMultipart(message Message, signer *Signer, limits Limits) ([][]byte, error) {
	for name, raw := range map[string][]byte{
		"header":        message.Header,
		"parent_header": message.ParentHeader,
		"metadata":      message.Metadata,
		"content":       message.Content,
	} {
		if err := validateJSONObject(raw); err != nil {
			return nil, fmt.Errorf("invalid Jupyter %s: %w", name, err)
		}
	}
	if signer == nil {
		return nil, errors.New("jupyter signer is required")
	}

	frames := make([][]byte, 0, len(message.RoutingPrefix)+6+len(message.Buffers))
	frames = append(frames, cloneFrames(message.RoutingPrefix)...)
	frames = append(frames,
		bytes.Clone(delimiter),
		signer.Sign(message.Header, message.ParentHeader, message.Metadata, message.Content),
		bytes.Clone(message.Header),
		bytes.Clone(message.ParentHeader),
		bytes.Clone(message.Metadata),
		bytes.Clone(message.Content),
	)
	frames = append(frames, cloneFrames(message.Buffers)...)
	if err := validateMultipartLimits(frames, limits); err != nil {
		return nil, err
	}
	return frames, nil
}

func validateJSONObject(raw []byte) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return errors.New("must be valid JSON")
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return errors.New("must be a JSON object")
	}
	return nil
}

func validateMultipartLimits(frames [][]byte, limits Limits) error {
	limits = limits.normalized()
	if len(frames) == 0 {
		return errors.New("jupyter multipart message is empty")
	}
	if len(frames) > limits.MaxFrameCount {
		return fmt.Errorf("jupyter multipart frame count %d exceeds limit %d", len(frames), limits.MaxFrameCount)
	}
	total := 0
	for _, frame := range frames {
		if len(frame) > limits.MaxFrameBytes {
			return fmt.Errorf("jupyter frame size %d exceeds limit %d", len(frame), limits.MaxFrameBytes)
		}
		if len(frame) > limits.MaxMessageBytes-total {
			return fmt.Errorf("jupyter multipart payload exceeds limit %d", limits.MaxMessageBytes)
		}
		total += len(frame)
	}
	return nil
}

func cloneFrames(frames [][]byte) [][]byte {
	if len(frames) == 0 {
		return nil
	}
	cloned := make([][]byte, len(frames))
	for i, frame := range frames {
		cloned[i] = bytes.Clone(frame)
	}
	return cloned
}
