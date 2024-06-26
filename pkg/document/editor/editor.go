package editor

import (
	"bytes"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/document/constants"
	"github.com/stateful/runme/v3/pkg/document/identity"
)

const (
	FrontmatterKey = "frontmatter"
	DocumentID     = "id"
)

func Deserialize(data []byte, identityResolver *identity.IdentityResolver) (*Notebook, error) {
	// Deserialize content to cells.
	doc := document.New(data, identityResolver)
	node, err := doc.Root()
	if err != nil {
		return nil, err
	}

	frontmatter, err := doc.Frontmatter()
	if err != nil {
		return nil, err
	}

	notebook := &Notebook{
		Cells:       toCells(doc, node, doc.Content()),
		Frontmatter: frontmatter,
		Metadata: map[string]string{
			PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey): strconv.Itoa(doc.TrailingLineBreaksCount()),
		},
	}

	var docID bytes.Buffer
	raw, err := frontmatter.Marshal(identityResolver.DocumentEnabled(), &docID)
	// Additionally, put raw frontmatter in notebook's metadata.
	// TODO(adamb): handle the error.
	if err == nil && len(raw) > 0 {
		notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)] = string(raw)
	}
	// Store document ID in metadata to bridge state where frontmatter with identity wasn't saved yet.
	if err == nil && docID.Len() > 0 {
		notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, DocumentID)] = docID.String()
	}
	// Include new frontmatter in notebook to bridge state where a new file/doc was created but not serialized yet.
	if notebook.Frontmatter == nil {
		if fm, err := document.ParseFrontmatter(raw); err == nil {
			notebook.Frontmatter = fm
		}
	}

	return notebook, nil
}

func Serialize(notebook *Notebook, outputMetadata *document.RunmeMetadata) ([]byte, error) {
	var result []byte
	var err error
	var frontmatter *document.Frontmatter

	// Serialize frontmatter.
	if intro, ok := notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)]; ok {
		raw := []byte(intro)

		frontmatter, err = document.ParseFrontmatter(raw)
		if err != nil {
			return nil, err
		}
	}

	var raw []byte
	if outputMetadata != nil && outputMetadata.Session.GetID() != "" {
		if frontmatter == nil {
			frontmatter = document.NewYAMLFrontmatter()
		}
		if frontmatter.Runme == nil {
			frontmatter.Runme = &document.RunmeMetadata{}
		}
		frontmatter.Runme.Session = outputMetadata.Session
		frontmatter.Runme.Session.Updated = prettyTime(time.Now())
		frontmatter.Runme.Document = outputMetadata.Document
	}

	if frontmatter != nil {
		// if the deserializer didn't add the ID first, it means it's not required
		requireIdentity := !frontmatter.Runme.IsEmpty() && frontmatter.Runme.ID != ""
		var internalDocID bytes.Buffer
		raw, err = frontmatter.Marshal(requireIdentity, &internalDocID)
		if err != nil {
			return nil, err
		}

		lb := document.DetectLineBreak(raw)
		result = append(
			raw,
			bytes.Repeat(lb, 2)...,
		)
	}

	// Serialize cells.
	result = append(result, serializeCells(notebook.Cells)...)

	// Add trailing line breaks.
	if lineBreaks, ok := notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)]; ok {
		desired, err := strconv.Atoi(lineBreaks)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		lb := document.DetectLineBreak(result)
		actual := document.CountTrailingLineBreaks(result, lb)
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
