package document

import (
	"testing/fstest"
)

var testREADME = []byte(`
## Shell

This is a basic snippet with a shell command:

` + "```" + `sh
$ echo "Hello, rdme!"
` + "```" + `

It can have an annotation with a name:

` + "```" + `sh {name=echo}
$ echo "Hello, rdme!"
` + "```" + `

It can contain multiple lines too:

` + "```" + `sh
$ echo "1"
$ echo "2"
$ echo "3"
` + "```" + `

Also, the dollar sign is not needed:

` + "```" + `sh
echo "Hello, rdme! Again!"
` + "```" + `

## Go

It can also execute a snippet of Go code:

` + "```" + `go
package main

import (
"fmt"
)

func main() {
	fmt.Println("Hello from Go, rdme!")
}
` + "```")

var testFS = fstest.MapFS{
	"README.md": &fstest.MapFile{
		Data: testREADME,
	},
}
