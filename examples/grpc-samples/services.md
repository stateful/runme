---
runme:
  id: 01HG9CJ12JXPWFTA90C2HT8ZX3
  version: v2.2
---

## ProjectService

Flags match the launcher arguments in `.vscode/launch.json` for `Debug Server`. Be sure to complete [setup.md](setup.md).

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
    -d "{\"directory\":{\"path\":\".\"}}" \
    127.0.0.1:9999 runme.project.v1.ProjectService/Load
```

## RunnerService

List all runner services.

```sh {"id":"01HNGQNYYWKP635FT8GHE67476","promptEnv":"false"}
export VERSION="v1" # there's also v2alpha1
$ grpcurl \
    -cacert /tmp/runme/tls/cert.pem \
    -cert /tmp/runme/tls/cert.pem \
    -key /tmp/runme/tls/key.pem \
    127.0.0.1:9999 list runme.runner.$VERSION.RunnerService
```

Resolve variable inside cells:

```sh {"id":"01HNGQS6TV8YKQAKE0ZD7TZREH","promptEnv":"false"}
$ grpcurl \
    -cacert /tmp/runme/tls/cert.pem \
    -cert /tmp/runme/tls/cert.pem \
    -key /tmp/runme/tls/key.pem \
    -d "{\"script\":\"export NAME=Noname\", \"env\":[\"NAME=Sebastian\"]}" \
    127.0.0.1:9999 runme.runner.v1.RunnerService/ResolveEnv
```
