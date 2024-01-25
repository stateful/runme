---
runme:
  id: 01HF7BT3HBDTRGQAQMH51RDQEC
  version: v2.2
---

# Daemon/kernel functionality

Install system dependencies:

```sh {"id":"01HF7BT3HBDTRGQAQMGTJN1KP3"}
$ brew bundle --no-lock
```

Let's build the project first and include working directory into `$PATH`:

```sh {"id":"01HF7BT3HBDTRGQAQMGVG57QBE"}
$ cd ../..
$ make
$ export CWD=$(cd ../.. && pwd | tr -d '\n')
$ export PATH="$CWD:$PATH"
```

## Exercise GRPC interface

Bring up the server. It's gRPC based:

```sh {"background":"true","id":"01HF7BT3HBDTRGQAQMGXCKJCAB"}
$ ../../runme server --address /tmp/runme.sock
```

Issue a simple call to the deserialize API, first set markdown input data:

```sh {"id":"01HF7BT3HBDTRGQAQMH0ZYTWA9"}
export mddata="# Ohai this is my cool headline"
```

Then issue RPC call and display the result:

```sh {"closeTerminalOnSuccess":"false","id":"01HF7BT3HBDTRGQAQMH2K85BHG"}
$ data="$(echo $mddata | openssl base64 | tr -d '\n')"
$ cd ../.. && grpcurl \
    -protoset <(buf build -o -) \
    -d "{\"source\": \"$data\"}" \
    -plaintext \
    -unix /tmp/runme.sock \
    runme.parser.v1.ParserService/Deserialize
```
