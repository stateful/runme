package editor

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"

	"github.com/stateful/runme/internal/document/constants"
)

const FrontmatterKey = "frontmatter"

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
		Cells:         toCells(node, data),
		contentOffset: sections.ContentOffset,
	}

	finalLinesBreaks := document.CountFinalLineBreaks(data, document.DetectLineBreak(data))
	notebook.Metadata = map[string]string{
		PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey): fmt.Sprint(finalLinesBreaks),
	}

	// If Front Matter exists, store it in Notebook's metadata.
	if len(sections.FrontMatter) > 0 {
		notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)] = string(sections.FrontMatter)
	}

	return notebook, nil
}

func DeserializeV2(data []byte, requireIdentity bool) (*Notebook, error) {
	notebook, err := Deserialize(data)
	if err != nil {
		return nil, err
	}

	rawfm := notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)]

	f, info := document.ParseFrontmatterWithIdentity(rawfm, requireIdentity)
	notebook.parsedFrontmatter = &f
	notebook.frontmatterParseInfo = &info

	if rawfm := info.GetRaw(); rawfm != "" {
		notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)] = rawfm
	}

	return notebook, nil
}

func Serialize(notebook *Notebook) ([]byte, error) {
	var result []byte

	if intro, ok := notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)]; ok {
		intro := []byte(intro)
		lb := document.DetectLineBreak(intro)
		result = append(
			intro,
			bytes.Repeat(lb, 2)...,
		)
	}

	result = append(result, serializeCells(notebook.Cells)...)

	if lineBreaks, ok := notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)]; ok {
		desired, err := strconv.ParseInt(lineBreaks, 10, 32)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		lb := document.DetectLineBreak(result)
		actual := document.CountFinalLineBreaks(result, lb)
		delta := int(desired) - actual

		if delta < 0 {
			end := len(result) + delta*len(lb)
			result = result[0:max(0, end)]
		} else {
			result = append(result, bytes.Repeat(lb, delta)...)
		}
	}

	return result, nil
}
