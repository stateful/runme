---
runme:
  id: 01HF7BT3HD84GWTQB9008APC3C
  version: v3
---

# Examples

## Shell

This is a basic snippet with shell command:

```sh {"id":"01HF7BT3HD84GWTQB8ZCWPH803"}
$ echo "Hello, runme!"
```

With `{"name": "hello"}` you can annotate it and give it a nice name:

```sh {"id":"01HF7BT3HD84GWTQB8ZEBC4E7R","name":"echo"}
$ echo "Hello, runme!"
```

It can contain multiple lines too:

```sh {"id":"01HF7BT3HD84GWTQB8ZESW75T0"}
$ echo "1"
$ echo "2"
$ echo "3"
```

Also, the dollar sign is not needed:

```sh {"id":"01HF7BT3HD84GWTQB8ZGWMJJE6"}
echo "Hello, runme! Again!"
```

It works with `cd`, `pushd`, and similar because all lines are executed as a single script:

```sh {"id":"01HF7BT3HD84GWTQB8ZHV2AF26"}
temp_dir=$(mktemp -d -t "runme-XXXXXXX")
pushd $temp_dir
echo "hi!" > hi.txt
pwd
cat hi.txt
popd
pwd
```

Sometimes, shell scripts fail:

```sh {"id":"01HF7BT3HD84GWTQB8ZNRMG7MW"}
echo ok
exit 1
```

### Interactive Scripts

```sh {"id":"01HF7BT3HD84GWTQB8ZNTNW63E","name":"print-name"}
echo -n "Enter your name: "
read name
echo ""
echo "Hi, $name!"
```

## JavaScript

It can also execute a snippet of JavaScript code:

```js {"id":"01HF7BT3HD84GWTQB8ZPB8TH53","name":"hello-js"}
console.log("Hello World!");
```

## Go

It can also execute a snippet of Go code:

```go {"id":"01HF7BT3HD84GWTQB8ZQXG4049"}
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

```ini {"id":"01HF7BT3HD84GWTQB8ZR32T70Y"}
[database]
username = admin
password = admin
```

## Tags

```sh {"tag":"a","id":"01HF7BT3HD84GWTQB8ZT8D9SVW","name":"a"}
echo "Tag A"
```

```sh {"tag":"a,b","id":"01HF7BT3HD84GWTQB8ZVF8JVKF","name":"b"}
echo "Tag A,B"
```

```sh {"tag":"a,b,c","id":"01HF7BT3HD84GWTQB8ZY0GBA06","name":"c"}
echo "Tag A,B,C"
```
