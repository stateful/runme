# Runme Agent

The `pkg/agent` package contains the Go server used by the Runme web
application. It provides runner and parser services, optional static web
application hosting, authentication, and direct Jupyter kernel management.
WebMCP is the supported integration path for browser automation.

Despite its historical package name, this is not an AI-only server. Runme Web
depends on it for normal notebook and cell execution.

## Server responsibilities

The server provides the runtime services required by Runme Web:

- `/ws` is the bidirectional WebSocket transport used to execute cells and
  stream terminal input and output. It is available when
  `assistantServer.runnerService` is enabled.
- The Runner service manages execution sessions and related runner operations.
- The Parser service parses and serializes notebook content when
  `assistantServer.parserService` is enabled.
- The Jupyter kernel API starts client-specified kernels, manages their
  lifecycle, and bridges browser channel WebSockets directly to kernel ZeroMQ
  sockets. Callers need `RunnerUserRole`; no separate Jupyter Server is used.
- Optional OIDC authentication, authorization, and telemetry are provided by
  the same HTTP server.
- A web application can optionally be served from the directory configured by
  `assistantServer.staticAssets`. The directory must contain an `index.html`;
  client-side routes fall back to that file.

These services remain supported. The legacy AI messages, ChatKit, and
app-server bridge endpoints have been removed independently of the execution
server.

## Quickstart

Create a minimal configuration file at `~/.runme-agent/config.yaml`:

```yaml
apiVersion: ""
kind: ""
logging:
  level: debug
  sinks:
    - path: stderr
assistantServer:
  port: 8080
  runnerService: true
  parserService: true
  # Optional: serve a web application from this directory.
  # staticAssets: /absolute/path/to/webapp/dist
  corsOrigins:
    - "http://localhost:5173"
```

Start the server:

```bash {"name":"serve"}
runme agent serve
```

Web applications can be hosted independently or served by this server. For
independent hosting, configure the runner endpoint to use this server's
WebSocket endpoint, for example `ws://localhost:8080/ws`. To serve an
application from the agent server, set `assistantServer.staticAssets` to its
build directory. Static serving is enabled only when that directory contains a
readable `index.html`.

## Deprecated configuration compatibility

The `openai`, `cloudAssistant`, `webApp`, `assistantServer.agentService`, and
`assistantServer.webAppURL` keys are accepted for a transition release so
existing configuration files continue to load. The server ignores these keys
and logs a deprecation notice when they are present. Remove them from personal
and deployment configuration files before a later release removes the
compatibility fields.

Published legacy agent protobuf schemas and generated clients remain available
during the same transition, but the server no longer registers their services.
Consumers should stop calling those endpoints before the schemas are removed
in a later breaking release.

## Local tracing

Configure an OTLP endpoint in `config.yaml`:

```yaml
telemetry:
  otlpHTTPEndpoint: localhost:4318
```

Start Jaeger locally:

```sh {"name":"jaeger"}
docker run --rm --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 5778:5778 \
  -p 9411:9411 \
  jaegertracing/jaeger:2.6.0
```
