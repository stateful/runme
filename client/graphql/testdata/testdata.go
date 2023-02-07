package testdata

import "embed"

//go:embed *.json
var Contents embed.FS
