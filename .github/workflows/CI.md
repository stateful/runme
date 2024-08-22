---
cwd: ../..
shell: bash
skipPrompts: true
---

## CI/CD

Run all tests with coverage reports.

```sh {"id":"01J5XTG2WKVR4WG7B2FNPF6VZT","name":"ci-test"}
export SHELL="/bin/bash"
export TZ="UTC"
TAGS="test_with_docker" make test/coverage
make test/coverage/func
```

Run parser/serializer against a large quantity of markdown files.

```sh {"id":"01J5XXFEGPJ5ZJZERQ5YGBBRN8","name":"ci-test-robustness"}
make test/robustness
```
