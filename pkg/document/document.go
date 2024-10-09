package document

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/stateful/runme/v3/internal/renderer/cmark"
	"github.com/stateful/runme/v3/pkg/document/constants"
)

var DefaultAttributeParser = newFailoverAttributeParser(
	[]attributeParser{
		&jsonParser{},
		&babikMLParser{},
	},
	&jsonParser{},
)

var defaultRenderer = cmark.Render

type Document struct {
	source           []byte
	identityResolver identityResolver
	nameResolver     *nameResolver
	parser           parser.Parser
	renderer         renderer

	onceParse               sync.Once
	parseErr                error
	onceSplitSource         sync.Once
	splitSourceErr          error
	rootASTNode             ast.Node
	rootNode                *Node
	frontmatterRaw          []byte
	content                 []byte // raw data after frontmatter
	contentOffset           int
	trailingLineBreaksCount int

	onceParseFrontmatter sync.Once
	parseFrontmatterErr  error
	frontmatter          *Frontmatter
}

func New(source []byte, identityResolver identityResolver) *Document {
	return &Document{
		source:           source,
		identityResolver: identityResolver,
		nameResolver: &nameResolver{
			namesCounter: map[string]int{},
			cache:        map[interface{}]string{},
		},
		parser:               goldmark.DefaultParser(),
		renderer:             defaultRenderer,
		onceParse:            sync.Once{},
		onceSplitSource:      sync.Once{},
		onceParseFrontmatter: sync.Once{},
		contentOffset:        -1,
	}
}

func (d *Document) Content() []byte {
	return d.content
}

// ContentOffset returns the position of source from which
// the actual content starts. If a value <0 is returned,
// it means that the source is not parsed yet.
//
// Frontmatter is not a part of the content.
func (d *Document) ContentOffset() int {
	return d.contentOffset
}

func (d *Document) Parse() error {
	return d.splitAndParse()
}

func (d *Document) Root() (*Node, error) {
	if err := d.splitAndParse(); err != nil {
		return nil, err
	}
	return d.rootNode, nil
}

func (d *Document) RootAST() (ast.Node, error) {
	if err := d.splitAndParse(); err != nil {
		return nil, err
	}
	return d.rootASTNode, nil
}

func (d *Document) TrailingLineBreaksCount() int {
	return d.trailingLineBreaksCount
}

func (d *Document) splitAndParse() error {
	d.splitSource()

	if err := d.splitSourceErr; err != nil {
		return err
	}

	d.parse()

	if d.parseErr != nil {
		return d.parseErr
	}

	return nil
}

func (d *Document) FrontmatterRaw() []byte {
	return d.frontmatterRaw
}

// splitSource splits source into FrontMatter and content.
// TODO(adamb): replace it with an extension to goldmark.
// Example: https://github.com/abhinav/goldmark-frontmatter
func (d *Document) splitSource() {
	d.onceSplitSource.Do(func() {
		l := &itemParser{input: d.source}

		runItemParser(l, parseInit)

		for _, item := range l.items {
			switch item.Type() {
			case parsedItemFrontMatter:
				d.frontmatterRaw = item.Value(d.source)
			case parsedItemContent:
				d.content = item.Value(d.source)
				d.contentOffset = item.start
			case parsedItemError:
				if errors.Is(item.err, errParseRawFrontmatter) {
					d.parseFrontmatterErr = item.err
				} else {
					d.splitSourceErr = item.err
				}
				return
			}
		}
	})
}

func (d *Document) parse() {
	d.onceParse.Do(func() {
		d.rootASTNode = d.parser.Parse(text.NewReader(d.content))

		node := &Node{}
		if err := d.buildBlocksTree(d.rootASTNode, node); err != nil {
			d.parseErr = err
			return
		}

		d.rootNode = node

		d.trailingLineBreaksCount = countTrailingLineBreaks(d.source, detectLineBreak(d.source))
		// Retain trailing new lines. This information must be stored in
		// ast.Node's attributes because it's later used by internal/renderer/cmark.Render,
		// which does not use anything else than ast.Node.
		d.rootASTNode.SetAttributeString(constants.FinalLineBreaksKey, d.trailingLineBreaksCount)
	})
}

func (d *Document) buildBlocksTree(parent ast.Node, node *Node) error {
	for astNode := parent.FirstChild(); astNode != nil; astNode = astNode.NextSibling() {
		switch astNode.Kind() {
		case ast.KindCodeBlock, ast.KindFencedCodeBlock:
			block, err := newCodeBlock(
				d,
				astNode,
				d.identityResolver,
				d.nameResolver,
				d.content,
				d.renderer,
			)
			if err != nil {
				return errors.WithStack(err)
			}
			node.add(block)
		case ast.KindBlockquote, ast.KindList, ast.KindListItem:
			block, err := newInnerBlock(astNode, d.content, d.renderer)
			if err != nil {
				return errors.WithStack(err)
			}
			nNode := node.add(block)
			if err := d.buildBlocksTree(astNode, nNode); err != nil {
				return err
			}
		default:
			block, err := newMarkdownBlock(astNode, d.content, d.renderer)
			if err != nil {
				return errors.WithStack(err)
			}
			node.add(block)
		}
	}
	return nil
}

type nameResolver struct {
	namesCounter map[string]int
	cache        map[interface{}]string
}

func (r *nameResolver) Get(obj interface{}, name string) string {
	if v, ok := r.cache[obj]; ok {
		return v
	}
	var result string
	r.namesCounter[name]++
	if r.namesCounter[name] == 1 {
		result = name
	} else {
		result = fmt.Sprintf("%s-%d", name, r.namesCounter[name])
	}
	r.cache[obj] = result
	return result
}

func CountTrailingLineBreaks(source []byte, lineBreak []byte) int {
	return countTrailingLineBreaks(source, lineBreak)
}

func countTrailingLineBreaks(source []byte, lineBreak []byte) int {
	i := len(source) - len(lineBreak)
	numBreaks := 0

	for i >= 0 && bytes.Equal(source[i:i+len(lineBreak)], lineBreak) {
		i -= len(lineBreak)
		numBreaks++
	}

	return numBreaks
}

func DetectLineBreak(source []byte) []byte {
	return detectLineBreak(source)
}

func detectLineBreak(source []byte) []byte {
	crlfCount := bytes.Count(source, []byte{'\r', '\n'})
	lfCount := bytes.Count(source, []byte{'\n'})
	if crlfCount == lfCount {
		return []byte{'\r', '\n'}
	}
	return []byte{'\n'}
}
