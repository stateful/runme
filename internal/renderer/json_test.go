//go:build !windows

package renderer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bradleyjkemp/cupaloy/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

var testCases = []string{
	"happy",
	"simple",
	"linesless",
	"singleblock",
	"doublecode",
	"nocodeblock",
	"equalvshash",
}

func TestParser_Renderer(t *testing.T) {
	snapshotter := cupaloy.New(cupaloy.SnapshotSubdirectory("testdata/.snapshots"))
	for _, testName := range testCases {
		t.Run(testName, func(t *testing.T) {
			source, _ := os.ReadFile(filepath.Join("testdata", testName+".md"))
			mdp := goldmark.DefaultParser()
			rootNode := mdp.Parse(text.NewReader(source))

			var b bytes.Buffer
			mdr := goldmark.New(goldmark.WithRenderer(NewJSON(source, rootNode)))
			mdr.Renderer().Render(&b, source, rootNode)

			snapshotter.SnapshotT(t, b.String())
		})
	}
}
