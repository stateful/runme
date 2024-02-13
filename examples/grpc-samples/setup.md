---
runme:
  id: 01HF7BT3HBDTRGQAQMH51RDQEC
  version: v3
---

# GRPC Setup

Install system dependencies:

```sh {"id":"01HF7BT3HBDTRGQAQMGTJN1KP3"}
$ brew bundle --no-lock
```

## Exercise GRPC interface

Issue a simple call to the deserialize API, first set markdown input data:

```sh {"id":"01HF7BT3HBDTRGQAQMH0ZYTWA9"}
export MD="# Ohai this is my cool headline"
```

Then issue RPC call and display the result:

```sh {"closeTerminalOnSuccess":"false","id":"01HF7BT3HBDTRGQAQMH2K85BHG","terminalRows":"15"}
$ data="$(echo $MD | openssl base64 | tr -d '\n')"
$ cd ../.. && grpcurl \
    -cacert /tmp/runme/tls/cert.pem \
    -cert /tmp/runme/tls/cert.pem \
    -key /tmp/runme/tls/key.pem \
    -protoset <(buf build -o -) \
    -d "{\"source\": \"$data\"}" \
    127.0.0.1:9999 runme.parser.v1.ParserService/Deserialize
```
