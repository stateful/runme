package editor

import (
	"bytes"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/document/constants"
	"github.com/stateful/runme/v3/pkg/document/identity"
)

const (
	FrontmatterKey = "frontmatter"
	DocumentID     = "id"
)

type Options struct {
	IdentityResolver *identity.IdentityResolver
	LoggerInstance   *zap.Logger
}

func (o Options) Logger() *zap.Logger {
	if o.LoggerInstance == nil {
		o.LoggerInstance = zap.NewNop()
	}
	return o.LoggerInstance
}

func Deserialize(data []byte, opts Options) (*Notebook, error) {
	// Deserialize content to cells.
	doc := document.New(data, opts.IdentityResolver)
	node, err := doc.Root()
	if err != nil {
		return nil, err
	}

	frontmatter, fmErr := doc.FrontmatterWithError()
	// non-fatal error
	if fmErr != nil {
		opts.Logger().Warn("failed to parse frontmatter", zap.Error(fmErr))
	}

	notebook := &Notebook{
		Cells:       toCells(doc, node, doc.Content()),
		Frontmatter: frontmatter,
		Metadata: map[string]string{
			PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey): strconv.Itoa(doc.TrailingLineBreaksCount()),
		},
	}

	// Additionally, put raw frontmatter in notebook's metadata, no matter invalid or valid
	// TODO(adamb): handle the error.
	if raw, err := frontmatter.Marshal(opts.IdentityResolver.DocumentEnabled()); err == nil && len(raw) > 0 {
		notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)] = string(raw)
	}
	// if parsing frontmatter failed put unparsed frontmatter in notebook's metadata to avoid earsing it with "default frontmatter"
	if raw := doc.FrontmatterRaw(); fmErr != nil && len(raw) > 0 {
		notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)] = string(raw)
	}

	// Store internal ephemeral document ID if the document lifecycle ID is disabled.
	if !opts.IdentityResolver.DocumentEnabled() {
		notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, DocumentID)] = opts.IdentityResolver.EphemeralDocumentID()
	}

	return notebook, nil
}

func Serialize(notebook *Notebook, outputMetadata *document.RunmeMetadata, opts Options) ([]byte, error) {
	var result []byte
	var err error
	var frontmatter *document.Frontmatter

	// Serialize parsed frontmatter.
	intro, ok := notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, FrontmatterKey)]
	if ok {
		raw := []byte(intro)

		frontmatter, err = document.ParseFrontmatter(raw)
		// non-fatal error
		if err != nil {
			opts.Logger().Warn("failed to parse frontmatter", zap.Error(err))
		}
	}

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

	var raw []byte
	// retain raw frontmatter even if parsing failed due to invalidity
	if len(intro) > 0 {
		raw = []byte(intro)
	}

	if frontmatter != nil {
		// if the deserializer didn't add the ID first, it means it's not required
		requireIdentity := !frontmatter.Runme.IsEmpty() && frontmatter.Runme.ID != ""
		raw, err = frontmatter.Marshal(requireIdentity)
		if err != nil {
			return nil, err
		}
	}

	if len(raw) > 0 {
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
