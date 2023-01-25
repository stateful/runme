# Daemon/kernel functionality

Install system dependencies:

```sh
$ brew bundle --no-lock
```

Let's build the project first and include working directory into `$PATH`:

```sh
$ cd ../..
$ make
$ export CWD=$(cd ../.. && pwd | tr -d '\n')
$ export PATH="$CWD:$PATH"
```

## Exercise GRPC interface

Bring up the server. It's gRPC based:

```sh { background=true }
$ ../../runme server --address /tmp/runme.sock
```

Issue a simple call to the deserialize API, first set markdown input data:

```sh
export mddata="# Ohai this is my cool headline"
```

Then issue RPC call and display the result:

```sh { closeTerminalOnSuccess=false }
$ data="$(echo $mddata | openssl base64 | tr -d '\n')"
$ cd ../.. && grpcurl \
    -protoset <(buf build -o -) \
    -d "{\"source\": \"$data\"}" \
    -plaintext \
    -unix /tmp/runme.sock \
    runme.parser.v1.ParserService/Deserialize
```
