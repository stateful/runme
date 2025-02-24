---
runme:
  id: 01HG9CJ12JXPWFTA90C2HT8ZX3
  version: v3
---

## gRPC Services

```sh {"id":"01JMWX6ZJ7GCJH5V4J399583G7","promptEnv":"never","terminalRows":"1"}
export RUNME_SERVER_ADDR="127.0.0.1:9999"
export RUNME_SESSION_STRATEGY="recent"
export RUNME_TLS_DIR="/tmp/runme/tls"
```

## ProjectService

Flags match the launcher arguments in `.vscode/launch.json` for `Debug Server`. Be sure to complete [setup.md](setup.md).

List available operations:

```sh {"id":"01HG9EB92X51P42CG6CGH00FT3","promptEnv":"never"}
$ grpcurl \
    -cacert $RUNME_TLS_DIR/cert.pem \
    -cert $RUNME_TLS_DIR/cert.pem \
    -key $RUNME_TLS_DIR/key.pem $RUNME_SERVER_ADDR list runme.project.v1.ProjectService
```

Load file project:

```sh {"id":"01HG9EB92X51P42CG6CK41HRRV","terminalRows":"28"}
$ grpcurl \
    -cacert $RUNME_TLS_DIR/cert.pem \
    -cert $RUNME_TLS_DIR/cert.pem \
    -key $RUNME_TLS_DIR/key.pem \
    -d "{\"file\":{\"path\":\"./examples/README.md\"}}" $RUNME_SERVER_ADDR runme.project.v1.ProjectService/Load
```

Load directory project:

```sh {"id":"01HG9EB92X51P42CG6CP8Y07F1","terminalRows":"28"}
$ grpcurl \
    -cacert $RUNME_TLS_DIR/cert.pem \
    -cert $RUNME_TLS_DIR/cert.pem \
    -key $RUNME_TLS_DIR/key.pem \
    -d "{\"directory\":{\"path\":\".\"}}" $RUNME_SERVER_ADDR runme.project.v1.ProjectService/Load
```

## RunnerService

List all runner services.

```sh {"id":"01HNGQNYYWKP635FT8GHE67476","promptEnv":"false"}
export VERSION="v1" # there's also v2alpha1
$ grpcurl \
    -cacert $RUNME_TLS_DIR/cert.pem \
    -cert $RUNME_TLS_DIR/cert.pem \
    -key $RUNME_TLS_DIR/key.pem $RUNME_SERVER_ADDR list runme.runner.$VERSION.RunnerService
```

List sessions:

```sh {"id":"01JMWPZKRDMPE6NRK2EX35VXNF","terminalRows":"16"}
$ grpcurl \
    -cacert $RUNME_TLS_DIR/cert.pem \
    -cert $RUNME_TLS_DIR/cert.pem \
    -key $RUNME_TLS_DIR/key.pem $RUNME_SERVER_ADDR \
    runme.runner.v1.RunnerService/ListSessions | jq -r ".sessions[].id"
```

Resolve variables inside cell:

```sh {"id":"01HNGQS6TV8YKQAKE0ZD7TZREH","name":"resolve-vars","promptEnv":"false","terminalRows":"48"}
$ cat resolve-program.json | jq .
$ echo "👆 request and 👇 response"
$ grpcurl \
    -cacert $RUNME_TLS_DIR/cert.pem \
    -cert $RUNME_TLS_DIR/cert.pem \
    -key $RUNME_TLS_DIR/key.pem \
    -d @ $RUNME_SERVER_ADDR runme.runner.v1.RunnerService/ResolveProgram < resolve-program.json | jq .
```

### Complex script

```javascript {"id":"01HNQWVXY92G9KC9VYB17EMNR4","interactive":"false"}
const fs = require('node:fs')

const bytes = fs.readFileSync('./deploy-helpers.sh')
const payload = { script: bytes.toString('utf-8') }
const serialized = JSON.stringify(payload)

fs.writeFileSync('complex-script.json', serialized)
console.log(serialized)
```

```sh {"id":"01HNQVQB1H16B9QNV49BR0EJYY","promptEnv":"false","terminalRows":"15"}
$ grpcurl \
    -cacert $RUNME_TLS_DIR/cert.pem \
    -cert $RUNME_TLS_DIR/cert.pem \
    -key $RUNME_TLS_DIR/key.pem \
    -d @ $RUNME_SERVER_ADDR runme.runner.v1.RunnerService/ResolveProgram < complex-script.json
```
