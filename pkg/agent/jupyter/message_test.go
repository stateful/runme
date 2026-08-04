package jupyter

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMarshalMultipartGoldenAndRoundTrip(t *testing.T) {
	signer, err := NewSigner("secret", SignatureSchemeHMACSHA256)
	if err != nil {
		t.Fatal(err)
	}
	message := Message{
		RoutingPrefix: [][]byte{[]byte("client-a"), []byte("route-b")},
		Header:        []byte(`{"msg_id":"abc","msg_type":"execute_request","session":"s","username":"runme","version":"5.3"}`),
		ParentHeader:  []byte(`{}`),
		Metadata:      []byte(`{"x":1}`),
		Content:       []byte(`{"code":"print(1)","silent":false}`),
		Buffers:       [][]byte{{0x00, 0x01, 0xfe, 0xff}},
	}

	frames, err := MarshalMultipart(message, signer, Limits{})
	if err != nil {
		t.Fatalf("MarshalMultipart() error = %v", err)
	}
	want := [][]byte{
		[]byte("client-a"),
		[]byte("route-b"),
		[]byte("<IDS|MSG>"),
		[]byte("032afa1cd3332a9794de5989bf088fbe4ca39b6ca5d28c4d9e61c5271e6fda3c"),
		message.Header,
		message.ParentHeader,
		message.Metadata,
		message.Content,
		{0x00, 0x01, 0xfe, 0xff},
	}
	if !reflect.DeepEqual(frames, want) {
		t.Fatalf("MarshalMultipart() frames differ\n got: %q\nwant: %q", frames, want)
	}

	parsed, err := ParseMultipart(frames, signer, Limits{})
	if err != nil {
		t.Fatalf("ParseMultipart() error = %v", err)
	}
	if !reflect.DeepEqual(parsed.RoutingPrefix, message.RoutingPrefix) {
		t.Fatalf("routing prefix = %q, want %q", parsed.RoutingPrefix, message.RoutingPrefix)
	}
	if !reflect.DeepEqual(parsed.Buffers, message.Buffers) {
		t.Fatalf("buffers = %v, want %v", parsed.Buffers, message.Buffers)
	}
}

func TestParseMultipartRejectsInvalidTraffic(t *testing.T) {
	signer, err := NewSigner("secret", SignatureSchemeHMACSHA256)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := MarshalMultipart(Message{
		Header:       []byte(`{"msg_id":"abc"}`),
		ParentHeader: []byte(`{}`),
		Metadata:     []byte(`{}`),
		Content:      []byte(`{"code":"1"}`),
	}, signer, Limits{})
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		frames [][]byte
		limits Limits
	}{
		"missing delimiter": {frames: valid[1:]},
		"missing content":   {frames: valid[:len(valid)-1]},
		"bad signature": {
			frames: func() [][]byte {
				frames := cloneFrames(valid)
				frames[1] = bytes.Repeat([]byte("0"), 64)
				return frames
			}(),
		},
		"malformed header": {
			frames: func() [][]byte {
				frames := cloneFrames(valid)
				frames[2] = []byte(`[]`)
				frames[1] = signer.Sign(frames[2], frames[3], frames[4], frames[5])
				return frames
			}(),
		},
		"too many frames": {
			frames: valid,
			limits: Limits{MaxFrameCount: len(valid) - 1},
		},
		"oversized frame": {
			frames: valid,
			limits: Limits{MaxFrameBytes: 4},
		},
		"oversized message": {
			frames: valid,
			limits: Limits{MaxFrameBytes: 1024, MaxMessageBytes: 8},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseMultipart(test.frames, signer, test.limits); err == nil {
				t.Fatal("ParseMultipart() unexpectedly succeeded")
			}
		})
	}
}

func TestEmptyKeyRequiresEmptySignature(t *testing.T) {
	signer, err := NewSigner("", SignatureSchemeHMACSHA256)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := MarshalMultipart(Message{
		Header:       []byte(`{}`),
		ParentHeader: []byte(`{}`),
		Metadata:     []byte(`{}`),
		Content:      []byte(`{}`),
	}, signer, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(frames[1]) != 0 {
		t.Fatalf("signature = %q, want empty", frames[1])
	}
	frames[1] = []byte("unexpected")
	if _, err := ParseMultipart(frames, signer, Limits{}); err == nil {
		t.Fatal("ParseMultipart() accepted a signature with an empty key")
	}
}

func FuzzParseMultipart(f *testing.F) {
	f.Add([]byte("<IDS|MSG>\x00{}{}{}{}"), uint8(0))
	f.Add([]byte{0xff, 0x00, 0x01}, uint8(3))
	f.Fuzz(func(t *testing.T, raw []byte, splitSeed uint8) {
		signer, err := NewSigner("fuzz-key", SignatureSchemeHMACSHA256)
		if err != nil {
			t.Fatal(err)
		}
		frameCount := int(splitSeed%16) + 1
		frames := make([][]byte, 0, frameCount)
		for i := 0; i < frameCount; i++ {
			start := len(raw) * i / frameCount
			end := len(raw) * (i + 1) / frameCount
			frames = append(frames, raw[start:end])
		}
		_, _ = ParseMultipart(frames, signer, Limits{
			MaxFrameCount:   32,
			MaxFrameBytes:   64 * 1024,
			MaxMessageBytes: 128 * 1024,
		})
	})
}
