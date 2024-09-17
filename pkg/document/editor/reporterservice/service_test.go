package reporterservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/wrapperspb"

	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	reporterv1alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/reporter/v1alpha1"
)

func TestTransform(t *testing.T) {
	logger := zaptest.NewLogger(t)
	server := NewReporterServiceServer(logger)

	req := &reporterv1alpha1.TransformRequest{
		Notebook: &parserv1.Notebook{
			Metadata: map[string]string{
				"key1": "value1",
			},
			Frontmatter: &parserv1.Frontmatter{
				Shell:       "shell",
				Cwd:         "/path/to/cwd",
				SkipPrompts: false,
				Runme: &parserv1.FrontmatterRunme{
					Id:      "runme-id",
					Version: "runme-version",
					Session: &parserv1.RunmeSession{
						Id: "session-id",
						Document: &parserv1.RunmeSessionDocument{
							RelativePath: "relative-path",
						},
					},
				},
				Category:     "category",
				Tag:          "tag",
				TerminalRows: "10",
			},
			Cells: []*parserv1.Cell{
				{
					Kind:       1,
					Value:      "print('hello world')",
					LanguageId: "python",
					Metadata:   map[string]string{"key1": "value1"},
					TextRange: &parserv1.TextRange{
						Start: uint32(1),
						End:   uint32(2),
					},
					ExecutionSummary: &parserv1.CellExecutionSummary{
						ExecutionOrder: wrapperspb.UInt32(1),
						Success:        &wrapperspb.BoolValue{Value: true},
						Timing: &parserv1.ExecutionSummaryTiming{
							StartTime: &wrapperspb.Int64Value{
								Value: 1630454400,
							},
							EndTime: &wrapperspb.Int64Value{
								Value: 1630454400,
							},
						},
					},
					Outputs: []*parserv1.CellOutput{
						{
							ProcessInfo: &parserv1.CellOutputProcessInfo{
								Pid: &wrapperspb.Int64Value{
									Value: 123,
								},
								ExitReason: &parserv1.ProcessInfoExitReason{
									Type: "success",
									Code: &wrapperspb.UInt32Value{
										Value: 0,
									},
								},
							},
							Metadata: map[string]string{"key1": "value1"},
							Items: []*parserv1.CellOutputItem{
								{
									Mime: "application/vnd.code.notebook.stdout",
									Data: []byte("output-data-1"),
								},
								{
									Mime: "application/vnd.code.notebook.stderr",
									Data: []byte("output-data-2"),
								},
							},
						},
					},
				},
			},
		},
		Extension: &reporterv1alpha1.TransformRequestExtension{
			AutoSave:       boolPtr(true),
			Repository:     stringPtr("repo-url"),
			Branch:         stringPtr("main"),
			Commit:         stringPtr("commit-id"),
			FilePath:       stringPtr("/path/to/file"),
			FileContent:    []byte("file-content"),
			PlainOutput:    []byte("plain-output"),
			MaskedOutput:   []byte("masked-output"),
			MacAddress:     stringPtr("00:00:00:00:00:00"),
			Hostname:       stringPtr("hostname"),
			Platform:       stringPtr("platform"),
			Release:        stringPtr("release"),
			Arch:           stringPtr("arch"),
			Vendor:         stringPtr("vendor"),
			Shell:          stringPtr("shell"),
			VsAppHost:      stringPtr("host"),
			VsAppName:      stringPtr("VSCode"),
			VsAppSessionId: stringPtr("session-id"),
			VsMachineId:    stringPtr("machine-id"),
			VsMetadata: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	resp, err := server.Transform(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, req.Notebook.Metadata, resp.Notebook.Metadata)
	assert.Equal(t, req.Notebook.Frontmatter, resp.Notebook.Frontmatter)
	assert.Equal(t, len(req.Notebook.Cells), len(resp.Notebook.Cells))

	// It should only include "application/vnd.code.notebook.stdout" mime type items
	assert.Equal(t, 1, len(resp.Notebook.Cells[0].Outputs[0].Items))
	assert.Equal(t, "application/vnd.code.notebook.stdout", resp.Notebook.Cells[0].Outputs[0].Items[0].Mime)

	assert.Equal(t, req.Extension.AutoSave, &resp.Extension.AutoSave)
	assert.Equal(t, req.Extension.Repository, &resp.Extension.Git.Repository)
	assert.Equal(t, req.Extension.Branch, &resp.Extension.Git.Branch)
	assert.Equal(t, req.Extension.Commit, &resp.Extension.Git.Commit)
	assert.Equal(t, req.Extension.FilePath, &resp.Extension.File.Path)
	assert.Equal(t, req.Extension.FileContent, resp.Extension.File.Content)
	assert.Equal(t, req.Extension.PlainOutput, resp.Extension.Session.PlainOutput)
	assert.Equal(t, req.Extension.MaskedOutput, resp.Extension.Session.MaskedOutput)
	assert.Equal(t, req.Extension.MacAddress, &resp.Extension.Device.MacAddress)
	assert.Equal(t, req.Extension.Hostname, &resp.Extension.Device.Hostname)
	assert.Equal(t, req.Extension.Platform, &resp.Extension.Device.Platform)
	assert.Equal(t, req.Extension.Release, &resp.Extension.Device.Release)
	assert.Equal(t, req.Extension.Arch, &resp.Extension.Device.Arch)
	assert.Equal(t, req.Extension.Vendor, &resp.Extension.Device.Vendor)
	assert.Equal(t, req.Extension.Shell, &resp.Extension.Device.Shell)
	assert.Equal(t, req.Extension.VsAppHost, &resp.Extension.Device.VsAppHost)
	assert.Equal(t, req.Extension.VsAppName, &resp.Extension.Device.VsAppName)
	assert.Equal(t, req.Extension.VsAppSessionId, &resp.Extension.Device.VsAppSessionId)
	assert.Equal(t, req.Extension.VsMachineId, &resp.Extension.Device.VsMachineId)
	assert.Equal(t, req.Extension.VsMetadata, resp.Extension.Device.VsMetadata)
}

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}
