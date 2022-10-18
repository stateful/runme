# Examples

## Shell

This is a basic snippet with shell command:

```sh
$ echo "Hello, runme!"
```

With `{name=hello}` you can annotate it and give it a nice name:

```sh {name=echo}
$ echo "Hello, runme!"
```

It can contain multiple lines too:

```sh
$ echo "1"
$ echo "2"
$ echo "3"
```

Also, the dollar sign is not needed:

```sh
echo "Hello, runme! Again!"
```

It works with `cd`, `pushd`, and similar because all lines are executed as a single script:

```sh
temp_dir=$(mktemp -d -t "runme-")
pushd $temp_dir
echo "hi!" > hi.txt
pwd
cat hi.txt
popd
pwd
```

## Go

It can also execute a snippet of Go code:

```go
package main

import (
    "fmt"
)

func main() {
    fmt.Println("Hello from Go, runme!")
}
```

## Unknown snippets

Snippets without provided type are ignored.

To still display unknown snippets, provide `--allow-unknown` to the `list` command.

```
[database]
username = admin
password = admin
```
