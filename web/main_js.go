package main

import (
	"bytes"
	"encoding/json"
	"syscall/js"

	"github.com/stateful/rdme/internal/parser"
)

// These are variables so that they can be set during the build time.
var (
	BuildDate    = "unknown"
	BuildVersion = "0.0.0"
	Commit       = "unknown"
)

func main() {
	js.Global().Set("GetSnippets", js.FuncOf(GetSnippets))
	js.Global().Set("GetDocument", js.FuncOf(GetDocument))

	select {}
}

func GetSnippets(this js.Value, args []js.Value) interface{} {
	readme := args[0].String()

	p := parser.New([]byte(readme))
	snippets := p.Snippets()

	var result []interface{}

	for _, s := range snippets {
		var lines []interface{}
		for _, line := range s.Lines() {
			lines = append(lines, line)
		}
		entry := map[string]interface{}{
			"name":        s.GetName(),
			"description": s.GetDescription(),
			"content":     s.GetContent(),
			"executable":  s.Executable(),
			"lines":       lines,
		}
		result = append(result, entry)
	}

	return result
}

func GetDocument(this js.Value, args []js.Value) interface{} {
	readme := args[0].String()
	p := parser.New([]byte(readme))

	var b bytes.Buffer
	p.Render(&b)

	dynamic := make(map[string]interface{})
	json.Unmarshal(b.Bytes(), &dynamic)

	return dynamic
}
