+++
[runme]
id = '01HRA297WC2HJP7X48FM3DR1V0'
version = 'v3'
+++

# The Owl Store 🦉

### ...aka smart ENV store. A better solution to specify, validate, and resolve a user's or workload's environment. Because:

![Owl Store](owl.png)

### The owl 🦉 is wise and knows environment variables aren't just for workloads they are for humans, too.

- A ENV solution for Humans and Workloads
- Specify, Validate, and Resolve ENV vars
- Verification of “Correctness” & better tools
- Smart Env Store, took inspiration from
- SSH-Agent and Javascript ⇄ Typescript
- The wisest of birds in the animal kingdom

### Environment “Specs”

#### .env.example

```
    JWT_SECRET=Secret to sign authed JWT tokens # Secret!
    ANON_KEY=Secret to sign anonymous JWT tokens # Secret!
    SERVICE_ROLE_KEY=JWT to assume the service role # JWT
    POSTGRES_PASSWORD=Password for the postgres user # Password!
    DASHBOARD_USERNAME=Username for the dashboard # Plain!
    DASHBOARD_PASSWORD=Password for the dashboard # Password!
    SOME_OTHER_VAR=Needs a matching value # Regex(/^[a-z...a. -]+\.)
```

### Environment Vars ⇄ “Specs”

POSTGRES_PASSWORD <-> Password! (! means Required)
[Name] [Spec/Type]

Your-super-secret...-password
[Value]

### Owl Store

- Composable, extensible, and progressive
- Queryable resolution thanks to Graph (DAG)
- Use Auth-Context, Machine & Runtime info, etc
- Connect to SOPS, Secret Managers, CLI tools etc
- E.g. different resolution paths per ENV class
- OWL easily better three letter acronym than ENV

### Low entry barrier

- .env files on outside - Graph Engine on inside
- Progressive: use as much or little as you need
- Different facades possible e.g. CRDs, YAML-dialect, SDKs
- Runme’s fallback resolution → “securely prompt user”
- Get involved, help building out owl toolkit & ecosystem

### Extensible at every stage

g. translated env.owl.yaml or JS/Golang/Java/etc SDKs

#### Resolution

![resolution](resolution.png)

#### .env Front-end

![front-end](front-end.png)

(query ASTs rendered in text for illustration)

## Common set of Specs

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
