package editor

import (
	"bytes"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
)

const FrontmatterKey = "runme.dev/frontmatter"

func Deserialize(data []byte) (*Notebook, error) {
	sections, err := document.ParseSections(data)
	if err != nil {
		return nil, err
	}

	// Deserialize content to cells.
	doc := document.New(sections.Content, cmark.Render)
	node, _, err := doc.Parse()
	if err != nil {
		return nil, err
	}

	notebook := &Notebook{
		Cells: toCells(node, data),
	}

	// If Front Matter exists, store it in Notebook's metadata.
	if len(sections.FrontMatter) > 0 {
		notebook.Metadata = map[string]string{
			FrontmatterKey: string(sections.FrontMatter),
		}
	}

	return notebook, nil
}

func Serialize(notebook *Notebook) ([]byte, error) {
	var result []byte

	if intro, ok := notebook.Metadata[FrontmatterKey]; ok {
		intro := []byte(intro)
		lb := detectLineBreak(intro)
		result = append(
			intro,
			append(lb, lb...)...,
		)
	}

	result = append(result, serializeCells(notebook.Cells)...)

	return result, nil
}

func detectLineBreak(source []byte) []byte {
	crlfCount := bytes.Count(source, []byte{'\r', '\n'})
	lfCount := bytes.Count(source, []byte{'\n'})
	if crlfCount == lfCount {
		return []byte{'\r', '\n'}
	}
	return []byte{'\n'}
}
