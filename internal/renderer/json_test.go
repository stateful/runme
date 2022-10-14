//go:build !windows

package renderer

import (
	"bytes"
	"os"
	"testing"

	"github.com/bradleyjkemp/cupaloy/v2"
	"github.com/stateful/runme/internal/document"
	"github.com/stretchr/testify/require"
)

var testCases = []string{
	"happy",
	"simple",
	"linesless",
	"singleblock",
	"doublecode",
	"nocodeblock",
	"equalvshash",
	"symbols",
	"singlebslash",
}

func TestParser_Renderer(t *testing.T) {
	snapshotter := cupaloy.New(cupaloy.SnapshotSubdirectory("testdata/.snapshots"))
	for _, testName := range testCases {
		t.Run(testName, func(t *testing.T) {
			s, err := document.NewSourceFromFile(os.DirFS("./testdata"), testName+".md")
			require.NoError(t, err)

			parsed := s.Parse()

			var b bytes.Buffer
			err = RenderToJSON(&b, parsed.Source(), parsed.Root())
			require.NoError(t, err)
			snapshotter.SnapshotT(t, b.String())
		})
	}
}
