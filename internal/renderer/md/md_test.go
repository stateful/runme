package md

// func TestRender_Basic(t *testing.T) {
// 	data := []byte(`This is a basic snippet with a shell command:

// ` + "```" + `sh
// $ echo "Hello, runme!"
// ` + "```" + `

// It can have an annotation with a name:

// ` + "```" + `sh {name=echo first= second=2}
// $ echo "Hello, runme!"
// ` + "```\n")
// 	source := document.NewSource(data)
// 	result, err := Render(source.Parse().Root(), data)
// 	require.NoError(t, err)
// 	assert.Equal(t, string(data), string(result))
// }
