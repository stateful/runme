package renderer

import (
	"encoding/json"
	"io"
)

func RenderJSON(v interface{}, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
