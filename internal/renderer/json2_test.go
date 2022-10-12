package renderer

// func TestRenderer_Render(t *testing.T) {
// 	source := document.NewSource([]byte(`
// ### Header

// Some paragraph

// > **Warning!**

// ` + "```" + `sh { name=install }
// brew bundle
// ` + "```" + `
// 	`))

// 	source := doc.NewSource([]byte(`
// > Warning!
// > **Warning!**
// `))

// 	parsed := source.Parse()
// 	mdr := goldmark.New(goldmark.WithRenderer(NewRenderer()))
// 	buf := new(bytes.Buffer)
// 	require.NoError(t, mdr.Renderer().Render(buf, parsed.Source(), parsed.Root()))
// 	require.Equal(t, `{"document":[{"markdown":"### Header"},{"markdown":"Some Paragraph"},{"markdown":"> Warning! **Warning!**"}]}`, buf.String())
// }
