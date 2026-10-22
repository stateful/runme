package jupyter

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestChannelJSONRoundTripPreservesProtocolObjects(t *testing.T) {
	payload := []byte(`{
  "channel":"shell",
  "header":{"msg_id":"m1","msg_type":"custom_request","extension":{"future":true}},
  "parent_header":{},
  "metadata":{"unknown":[1,2,3]},
  "content":{"custom":{"nested":"value"}},
  "buffers":[],
  "future_top_level":"ignored by Jupyter Session.send"
}`)
	channel, message, err := ParseChannelJSON(payload, Limits{})
	if err != nil {
		t.Fatalf("ParseChannelJSON() error = %v", err)
	}
	if channel != ChannelShell {
		t.Fatalf("channel = %q, want shell", channel)
	}

	encoded, err := MarshalChannelJSON(channel, message, Limits{})
	if err != nil {
		t.Fatalf("MarshalChannelJSON() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got["msg_id"] != "m1" || got["msg_type"] != "custom_request" {
		t.Fatalf("top-level compatibility fields = %v/%v", got["msg_id"], got["msg_type"])
	}
	var original map[string]json.RawMessage
	if err := json.Unmarshal(payload, &original); err != nil {
		t.Fatal(err)
	}
	var roundTrip map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"header", "parent_header", "metadata", "content"} {
		var wantValue, gotValue any
		if err := json.Unmarshal(original[field], &wantValue); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(roundTrip[field], &gotValue); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Fatalf("%s = %#v, want %#v", field, gotValue, wantValue)
		}
	}
}

func TestChannelJSONRejectsUnsupportedTraffic(t *testing.T) {
	base := `"header":{},"parent_header":{},"metadata":{},"content":{}`
	tests := map[string]string{
		"iopub target":    `{"channel":"iopub",` + base + `}`,
		"unknown channel": `{"channel":"comm",` + base + `}`,
		"binary buffer":   `{"channel":"shell",` + base + `,"buffers":["AA=="]}`,
		"missing header":  `{"channel":"shell","parent_header":{},"metadata":{},"content":{}}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseChannelJSON([]byte(payload), Limits{}); err == nil {
				t.Fatal("ParseChannelJSON() unexpectedly succeeded")
			}
		})
	}

	_, err := MarshalChannelJSON(ChannelIOPub, Message{
		Header:       []byte(`{"msg_id":"m","msg_type":"comm_msg"}`),
		ParentHeader: []byte(`{}`),
		Metadata:     []byte(`{}`),
		Content:      []byte(`{}`),
		Buffers:      [][]byte{{1, 2, 3}},
	}, Limits{})
	if !errors.Is(err, ErrBinaryBuffersUnsupported) {
		t.Fatalf("MarshalChannelJSON() error = %v, want ErrBinaryBuffersUnsupported", err)
	}
}
