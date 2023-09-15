# Release

## Upgrading go mods

Periodically dependencies need to be upgraded. For minor versions with test deps:

```sh
$ go get -t -u ./...
```

For major versions deps need to be upgraded one at a time:

```sh
$ gomajor list
```

## Protos

To release protobuf definitions:

```sh { name=buf-token }
export BUF_TOKEN=Your buf token
```

```sh { name=release-buf }
make proto/publish
```
