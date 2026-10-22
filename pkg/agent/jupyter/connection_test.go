package jupyter

import (
	"strings"
	"testing"
)

const validConnectionJSON = `{
  "transport": "tcp",
  "ip": "127.0.0.1",
  "shell_port": 51001,
  "iopub_port": 51002,
  "stdin_port": 51003,
  "control_port": 51004,
  "hb_port": 51005,
  "key": "secret",
  "signature_scheme": "hmac-sha256"
}`

func TestParseConnectionInfo(t *testing.T) {
	info, err := ParseConnectionInfo([]byte(validConnectionJSON))
	if err != nil {
		t.Fatalf("ParseConnectionInfo() error = %v", err)
	}
	got, err := info.Endpoint(ChannelShell)
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if want := "tcp://127.0.0.1:51001"; got != want {
		t.Fatalf("Endpoint() = %q, want %q", got, want)
	}
}

func TestParseConnectionInfoRejectsInvalidFiles(t *testing.T) {
	tests := map[string]string{
		"malformed JSON":        `{`,
		"unsupported transport": strings.Replace(validConnectionJSON, `"tcp"`, `"ipc"`, 1),
		"non-loopback IP":       strings.Replace(validConnectionJSON, `"127.0.0.1"`, `"192.0.2.10"`, 1),
		"invalid IP":            strings.Replace(validConnectionJSON, `"127.0.0.1"`, `"localhost"`, 1),
		"bad signature scheme":  strings.Replace(validConnectionJSON, `"hmac-sha256"`, `"hmac-sha1"`, 1),
		"invalid port":          strings.Replace(validConnectionJSON, `51001`, `0`, 1),
		"duplicate port":        strings.Replace(validConnectionJSON, `51002`, `51001`, 1),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseConnectionInfo([]byte(raw)); err == nil {
				t.Fatal("ParseConnectionInfo() unexpectedly succeeded")
			}
		})
	}
}

func FuzzParseConnectionInfo(f *testing.F) {
	f.Add([]byte(validConnectionJSON))
	f.Add([]byte(`{}`))
	f.Add([]byte{0xff, 0x00, '{'})
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ParseConnectionInfo(raw)
	})
}
