---
runme:
  id: 01HG9CJ12JXPWFTA90C2HT8ZX3
  version: v0.0
---

## gRPC runme.project.v1.ProjectService

Flags matche the launcher arguments in .vscode/launch.json.

List available operations:

```sh {"id":"01HG9EB92X51P42CG6CGH00FT3"}
$ grpcurl \
    -cacert /tmp/runme/tls/cert.pem \
    -cert /tmp/runme/tls/cert.pem \
    -key /tmp/runme/tls/key.pem \
    127.0.0.1:9999 list runme.project.v1.ProjectService
```

Load file project:

```sh {"id":"01HG9EB92X51P42CG6CK41HRRV","terminalRows":"28"}
$ grpcurl \
    -cacert /tmp/runme/tls/cert.pem \
    -cert /tmp/runme/tls/cert.pem \
    -key /tmp/runme/tls/key.pem \
    -d "{\"file\":{\"path\":\"./examples/README.md\"}}" \
    127.0.0.1:9999 runme.project.v1.ProjectService/Load
```

Load directory project:

```sh {"id":"01HG9EB92X51P42CG6CP8Y07F1","terminalRows":"28"}
$ grpcurl \
    -cacert /tmp/runme/tls/cert.pem \
    -cert /tmp/runme/tls/cert.pem \
    -key /tmp/runme/tls/key.pem \
    -d "{\"directory\":{\"path\":\".\",\"respect_gitignore\":true}}" \
    127.0.0.1:9999 runme.project.v1.ProjectService/Load
```
