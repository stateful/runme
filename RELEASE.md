---
runme:
  id: 01HF7BT3HEQBTBM9SSSRB5RCB8
  version: v3
---

# Release

## Upgrading go mods

Over time, it's essential to update your project dependencies for smoother and secure functioning. This section guides you through the process of upgrading Go module dependencies.

### Upgrading Minor Version Dependencies

For upgrading dependencies that have minor version changes, as well as test dependencies, use the following command. This command fetches the latest minor or patch versions of the modules required for building the current module, including their dependencies:
Periodically dependencies need to be upgraded. For minor versions with test deps:

```sh {"id":"01HF7BT3HEQBTBM9SSSG798S2D"}
$ go get -t -u ./...

```

### Upgrading Major Version Dependencies

When you need to upgrade to major versions of your dependencies, it’s prudent to upgrade each dependency one at a time to manage potential breaking changes efficiently. The `gomajor` tool can assist you in listing and upgrading major version dependencies. To list all major version upgrades available for your project, use the following command:

```sh {"id":"01HF7BT3HEQBTBM9SSSK6JMS58"}
$ gomajor list

```

## Protos

Protocol Buffers (Protos) are a language-agnostic binary serialization format. If you have made changes to your protobuf definitions and need to release these, follow the steps outlined below.

### Set Buffer Token

Firstly, set up your Buffer (Buf) token by exporting it as an environment variable. Replace `Your buf token` with your actual Buf token:

```sh {"id":"01HF7BT3HEQBTBM9SSSNM5ZT19","name":"buf-token"}
export BUF_TOKEN=Your buf token

```

### Publish Protobuf Definitions

After setting the Buf token, proceed to publish the updated protobuf definitions. The `make proto/publish` command below will trigger the publishing process, ensuring your protobuf definitions are released and available for use:

```sh {"id":"01HF7BT3HEQBTBM9SSSPD5B5WW","name":"release-buf"}
make proto/publish

```
