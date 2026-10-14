# Runme Agent

The `pkg/agent` package contains the Go server used by the Runme web
application. It provides runner and parser services, optional static web
application hosting, authentication, and Jupyter proxying. WebMCP is the
supported integration path for browser automation.

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
- The Jupyter proxy manages Jupyter servers and forwards kernel channel
  WebSockets.
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

## Browser JWT authentication and Google workload tokens

The optional Google token broker exposes `POST /v1/credentials/google:mint`.
It uses workload ADC (GKE metadata or an operator-managed `external_account`
configuration) to call Google IAM Credentials for one fixed service account.
User ADC and service-account JSON private keys are rejected. The browser supplies
only `scopes` and optional `lifetimeSeconds`; the server enforces the scope
allowlist and a 60–3,600 second lifetime bound.

Example runner configuration (HTTP loopback; configure TLS separately when needed):

```yaml
assistantServer:
  bindAddress: 127.0.0.1
  port: 9988
  runnerService: true
  parserService: true
  corsOrigins: ["*"]
  oidc:
    static:
      issuer: urn:runme:browser
      audience: urn:runme:runner
      maxTokenLifetimeSeconds: 300
      clockSkewSeconds: 30
      keys:
        - kid: browser-auth-v1
          algorithm: ES256
          publicKeyFile: /run/runme/server/browser-auth.pub.pem
          subject: runme
          principal: runme
  googleTokenBroker:
    enabled: true
    source: workload_identity
    tokenMode: iam_credentials
    serviceAccountEmail: runme@project.iam.gserviceaccount.com
    allowedScopes: ["https://www.googleapis.com/auth/drive"]
    defaultLifetimeSeconds: 900
    maxLifetimeSeconds: 3600
iamPolicy:
  bindings:
    - role: role/tokenCreator
      members: [{kind: user, name: runme}]
    - role: role/runner.user
      members: [{kind: user, name: runme}]
    - role: role/parser.user
      members: [{kind: user, name: runme}]
```

Configure exactly one of `oidc.static`, `oidc.google`, or `oidc.generic`.
Static mode skips OIDC discovery and login routes. It verifies ES256 signatures,
issuer, audience, subject, issued-at, not-before, expiry, and maximum lifetime.
The IAM principal comes from the pinned-key configuration, never a caller's
email or role claim. A single browser JWT works across authorized services;
each endpoint checks its own role. `role/tokenCreator` alone does not permit
execution. It is distinct from Google's service-account token-creation roles.

```http
POST /v1/credentials/google:mint
Origin: https://web.runme.dev
Authorization: Bearer <runner-jwt>
Content-Type: application/json

{"scopes":["https://www.googleapis.com/auth/drive"],"lifetimeSeconds":900}
```

The JSON response contains `accessToken`, `tokenType: "Bearer"`, `expiresAt`
(the actual Google expiry), `serviceAccountEmail`, and normalized `scopes`.
Mint responses use `Cache-Control: no-store`. Broker errors contain a stable
error code and request ID, never Google response bodies or credentials.
Authentication and IAM failures use the standard protected-route errors.

The endpoint uses the same authentication and CORS middleware as parser and
Jupyter routes: bearer headers, authorization query parameters, and session
cookies are verified, then IAM must grant `role/tokenCreator`. Browser clients
should use the Authorization header. `corsOrigins: ["*"]` allows all sites,
including HTTP development sites; an explicit origin list can still be used.
Requests without an Origin header are also authenticated normally. CORS is not
an authorization check and successful preflight never grants permission to mint.
The broker retains its loopback bind/Host restrictions, explicit auth/IAM
requirement, request-field validation, and scope allowlist. Authenticated request
bursts are limited, and concurrent mints for the same scope/lifetime are coalesced.

Enable IAM Credentials and the target Google API, and grant the workload
`iam.serviceAccounts.getAccessToken` on the configured GSA. When metadata already
returns that GSA's token, the second mint is self-impersonation and needs an
explicit grant on itself. For Drive, grant the GSA access to the intended Shared
drive; token creation does not grant file access. No infrastructure grants are
created by this server. The public verification key can be mounted via CSI or a
ConfigMap; keep the private signing key with the provisioned browser. Restart
the runner when rotating its configured public keys.

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
