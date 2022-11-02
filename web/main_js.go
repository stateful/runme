package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer"
	"github.com/stateful/runme/internal/runner"
)

// These are variables so that they can be set during the build time.
var (
	BuildDate    = "unknown"
	BuildVersion = "0.0.0"
	Commit       = "unknown"
)

func main() {
	runme := map[string]interface{}{
		"initialize":    js.FuncOf(initialize),
		"getCell":       js.FuncOf(getCell),
		"getCells":      js.FuncOf(getCells),
		"getSource":     js.FuncOf(getSource),
		"updateCell":    js.FuncOf(updateCell),
		"prepareScript": js.FuncOf(prepareScript),
	}
	js.Global().Set("Runme", js.ValueOf(runme))

	select {}
}

var (
	parsed     *document.ParsedSource
	sourceHash [16]byte
)

func assertParsed() js.Value {
	if parsed == nil {
		return toJSError(errors.New("call initialize() first"))
	}
	return js.Null()
}

func initialize(this js.Value, args []js.Value) any {
	source := args[0].String()

	parsed = document.NewSource([]byte(source)).Parse()
	sourceHash = md5.Sum([]byte(source))

	return nil
}

func getCell(this js.Value, args []js.Value) any {
	if val := assertParsed(); !val.IsNull() {
		return val
	}

	idx := args[0].Int()
	blocks := parsed.Blocks()

	if idx >= len(blocks) {
		return toJSError(errors.New("block index out of range"))
	}

	data, err := json.Marshal(blocks[idx])
	if err != nil {
		return toJSError(err)
	}

	result := make(map[string]interface{})
	if err := json.Unmarshal(data, &result); err != nil {
		return toJSError(err)
	}
	return result
}

func getCells(this js.Value, args []js.Value) any {
	if val := assertParsed(); !val.IsNull() {
		return val
	}

	var b bytes.Buffer
	if err := renderer.ToJSON(parsed, &b); err != nil {
		return toJSError(err)
	}

	result := make(map[string]interface{})
	if err := json.Unmarshal(b.Bytes(), &result); err != nil {
		return toJSError(err)
	}
	return result
}

func getSource(this js.Value, args []js.Value) any {
	if val := assertParsed(); !val.IsNull() {
		return val
	}
	return string(parsed.Source())
}

func updateCell(this js.Value, args []js.Value) any {
	if val := assertParsed(); !val.IsNull() {
		return val
	}

	idx := args[0].Int()
	newSource := args[1].String()

	updater := document.NewUpdater(parsed)
	if err := updater.UpdateBlock(idx, newSource); err != nil {
		return toJSError(err)
	}

	parsed = updater.Parsed()
	sourceHash = md5.Sum(parsed.Source())

	return nil
}

func prepareScript(this js.Value, args []js.Value) interface{} {
	lines := args[0]
	len := lines.Length()
	scriptLines := make([]string, 0, len)
	for i := 0; i < len; i++ {
		scriptLines = append(scriptLines, lines.Index(i).String())
	}
	return runner.PrepareScript(scriptLines)
}

func toJSError(err error) js.Value {
	if err == nil {
		return js.Null()
	}
	return js.Global().Get("Error").New(err.Error())
}
