# Tortuga API

The API built for [Tortuga Prototype](https://www.notion.so/statefulhq/Tortuga-Prototype-c406dd5fa1ad452dba15560a6cead5f9).

> **Warning!** All code snippets below assume you're in the **`./api`** directory.

## Start

First 😇, install dev dependencies:

```sh {"id":"01HFW6VKQYFGMC9MX7BBAP4YM5","name":"install"}
brew bundle
```

Deploy site to Vercel

```Vercel {"id":"01HFW6VKQYFGMC9MX7BDT3D3P4"}
https://vercel.com/stateful/stateful-com
```

Next run dependencies:

```sh {"id":"01HFW6VKQYFGMC9MX7BFNCP512","name":"docker-compose"}
$ docker compose up -d
```

Then you should be able to successfully run:

```sh {"id":"01HFW6VKQYFGMC9MX7BK1X58KR","name":"run"}
$ echo "Running"
$ go run ./cmd/api/main.go
2022/05/10 12:18:18 starting to listen on: :8080
```

## Development

> Currently, VS Code and Go extension require opening `./api` as a project root directory to work properly.
> You can use Workspaces to open the project root directory and `./api` as a second folder.

Try using [watchexec](https://github.com/watchexec/watchexec) to autoreload.

```sh {"id":"01HFW6VKQYFGMC9MX7BK6BH27X","name":"watch"}
watchexec -r -e go -- go run ./cmd/api/main.go
```

## Deployment

Deployments are managed with Terraform. Go to [infra](../infra) to learn how to run it.

[infra](../infra) automatically discovers if source files of the api changed. If so, it triggers a Docker image build and updates a Cloud Run service.

## Database

It uses PostgreSQL.

### Migrations

[Atlas CLI](https://atlasgo.io/cli/getting-started/setting-up) is used to manage database migrations in a declarative way.

Migrations are run automatically by the API server process.

In a case you want to run them manually and re-use Postgres from Docker Compose:

```sh {"id":"01HFW6VKQYFGMC9MX7BME16373","name":"migrate"}
$ atlas schema apply -u "postgres://postgres:postgres@localhost:15432/tortuga?sslmode=disable" -f atlas.hcl
```

## API

> Each insert accepts also `user_id` which is nullable for now.
> All endpoints implement also `GET` method to return all collected results so far starting from the most recent.

### Tasks

Inserting task execution metadata:

```sh {"id":"01HFW6VKQYFGMC9MX7BN0DVBJ5","name":"post-task"}
$ curl -XPOST -H "Content-Type: application/json" localhost:8080/tasks/ -d '{"duration": "10s", "exit_code": 0, "name": "Run task", "runbook_name": "RB 1", "runbook_run_id": "6e975f1b-0c0f-4765-b24a-2aa87b901c06", "start_time": "2022-05-05T04:12:43Z", "command": "/bin/sh", "args": "echo hello", "feedback": "this is cool!", "extra": "{\"hello\": \"world\"}"}'
{"id":"6e975f1b-0c0f-4765-b24a-2aa87b901c06"}
```

A task can be patched:

```sh {"id":"01HFW6VKQYFGMC9MX7BPNPAYYH","name":"patch-task"}
$ curl -X PATCH -H "Content-Type: application/json" localhost:8080/tasks/6e975f1b-0c0f-4765-b24a-2aa87b901c06/ -d '{"duration": "15s", "exit_code": 1}'
{"id":"6e975f1b-0c0f-4765-b24a-2aa87b901c06"}
```

### Feedback

Inserting feedback can optionally take a `task_id`:

```sh {"id":"01HFW6VKQYFGMC9MX7BSYXYYAZ","name":"post-feedback"}
$ curl -XPOST -H "Content-Type: application/json" localhost:8080/feedback/ -d '{"message": "My feedback!", "task_id": "6e975f1b-0c0f-4765-b24a-2aa87b901c06"}'
{"id":"a02b6b5f-46c4-40ff-8160-ff7d55b8ca6f"}
```

Feedback can be patched:

```sh {"id":"01HFW6VKQYFGMC9MX7BWNP0EG0","name":"patch-feedback"}
$ curl -X PATCH -H "Content-Type: application/json" localhost:8080/feedback/a02b6b5f-46c4-40ff-8160-ff7d55b8ca6f/ -d '{"message": "Modified!"}'
{"id":"a02b6b5f-46c4-40ff-8160-ff7d55b8ca6f"}
```

### Editor configs

Inserting editor configs:

```sh {"id":"01HFW6VKQYFGMC9MX7BZK10CRJ","name":"post-editor-config"}
$ curl -XPOST -H "Content-Type: application/json" localhost:8080/editor-configs/ -d '{"data": "{\"files.autoSave\": \"afterDelay\"}"}'
{"id":"4c7d6fb5-eb53-44f7-8883-80f276af65a1"}
```
