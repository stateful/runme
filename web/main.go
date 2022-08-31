package main

import (
	// go:build (js)
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

	<-make(chan bool)
}

func GetSnippets(this js.Value, args []js.Value) interface{} {
	readme := args[0].String()
	p := parser.New([]byte(readme))
	snippets := p.Snippets()
	var jsJson []interface{}

	for _, s := range snippets {
		var lines []interface{}
		for _, line := range s.Lines() {
			lines = append(lines, line)
		}
		entry := map[string]interface{}{
			"name":        s.Name(),
			"description": s.Description(),
			"content":     s.Content(),
			"executable":  s.Executable(),
			"lines":       lines,
		}
		jsJson = append(jsJson, entry)
	}

	return jsJson
}
