package parser

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bradleyjkemp/cupaloy/v2"
)

var testCases = []string{"happy", "simple"}

func TestParser_Renderer(t *testing.T) {
	snapshotter := cupaloy.New(cupaloy.SnapshotSubdirectory("testdata/.snapshots"))
	for _, testName := range testCases {
		t.Run(testName, func(t *testing.T) {
			source, _ := os.ReadFile(filepath.Join("testdata", testName+".md"))
			p := New(source)
			var b bytes.Buffer
			p.Render(&b)
			snapshotter.SnapshotT(t, b.String())
		})
	}
}
