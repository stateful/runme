+++
[runme]
id = '01HRA297WC2HJP7X48FM3DR1V0'
version = 'v3'
+++

# The Owl Store 🦉

### ...aka smart ENV store, is a better solution to specify, validate, and resolve a user's or workload's environment. Because environment variables aren't just for workloads they are for humans, too.

![Owl Store](owl.png)

### Features

- A ENV solution for Humans and Workloads
- Specify, Validate, and Resolve ENV vars
- Verification of “Correctness” & better tools
- Smart Env Store, took inspiration from
- SSH-Agent and Javascript ⇄ Typescript
- The wisest of birds in the animal kingdom

### Environment “Specs”

**.env.example**

```ini {"id":"01HS8C1PN0T7BGJA0T6TT2G68R"}
    JWT_SECRET=Secret to sign authed JWT tokens # Secret!
    ANON_KEY=Secret to sign anonymous JWT tokens # Secret!
    SERVICE_ROLE_KEY=JWT to assume the service role # JWT
    POSTGRES_PASSWORD=Password for the postgres user # Password!
    DASHBOARD_USERNAME=Username for the dashboard # Plain!
    DASHBOARD_PASSWORD=Password for the dashboard # Password!
    SOME_OTHER_VAR=Needs a matching value # Regex(/^[a-z...a. -]+\.)
```

### Philosophy

- Composable, extensible, and progressive
- Queryable resolution thanks to Graph (DAG)
- Use Auth-Context, Machine & Runtime info, etc
- Connect to SOPS, Secret Managers, CLI tools etc
- E.g. different resolution paths per ENV class
- OWL easily better three letter acronym than ENV
- .env files on outside - Graph Engine on inside
- Progressive: use as much or little as you need
- Different facades possible e.g. CRDs, YAML-dialect, SDKs
- Runme’s fallback resolution → “securely prompt user”
- Get involved, help building out owl toolkit & ecosystem

### Environment Vars ⇄ “Specs”

![Specs](vars-specs.png)

### Extensible at every stage

#### Resolution (e.g. translated env.owl.yaml or JS/Golang/Java/etc SDKs)

![resolution](resolution.png)

#### .env Front-end (query ASTs rendered in text for illustration)

![front-end](front-end.png)

## Common set of Specs (not all available yet)

- Plain

  - Opaque
  - Regex(...)

- Secret

  - Password
  - JWT
  - x590Cer

- Resources

  - DbUrl
  - Redis

- Cred Sets (non-atomic)

  - FirebaseSdk
  - OpenAI
