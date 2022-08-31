package main

import (
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

	select {}
}

func GetSnippets(this js.Value, args []js.Value) interface{} {
	readme := args[0].String()

	p := parser.New([]byte(readme))
	snippets := p.Snippets()

	var result []interface{}

	for _, s := range snippets {
		entry := map[string]interface{}{
			"name":        s.Name(),
			"description": s.Description(),
			"content":     s.Content(),
			"executable":  s.Executable(),
			"lines":       s.Lines(),
		}
		result = append(result, entry)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return err.Error()
	}

	return string(data)
}
