# rdme

Execute code snippets directly from Markdown files, defaulting to local `README.md`.

rdme extracts all code snippets defined in code blocks and allows to explore and execute them.

You can execute commands from a different directory using a `--chdir` flag.
To select a different file than `README.md`, use `--filename`.

## Installation

The easiest way is to use Homebrew:

```sh
$ brew install stateful/tap/rdme
```

Alternatively, check out [rdme's releases](https://github.com/stateful/rdme/releases) and select
a binary for your operating system.

If you have Go developer tools installed, you can install it with `go install`:

```sh
$ go install github.com/stateful/rdme@latest
```

## Contributing

TBD

## LICENCE

APACHE LICENSE, VERSION 2.0
