# Release

## Protos

To release protobuf definitions:

``` sh { name=buf-token }
export BUF_TOKEN=Your buf token
```

``` sh { name=release-buf }
cd ./internal/api && buf push
```
