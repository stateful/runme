# Runme.dev

This project contains the application for [runme.dev](https://runme.dev) and
also acts as a guinea pig to showcase the
[Runme Extension](https://marketplace.visualstudio.com/items?itemName=stateful.runme).

## Prerequisites

This project is based on [Fresh](https://fresh.deno.dev/) and requires Deno to
run. To install Deno via homebrew on macOS:

```sh {"id":"01HFW6VKQYFGMC9MX7AGYM52YJ"}
# macOS
brew bundle --no-lock
```

Or the installer which also works on Linux:

```sh {"id":"01HFW6VKQYFGMC9MX7AJXQMV87"}
# macOS or Linux
curl -fsSL https://deno.land/x/install/install.sh | sh
```

Add Deno to your path:

```sh {"id":"01HFW6VKQYFGMC9MX7APAZH1Z9"}
export DENO_INSTALL="$HOME/.deno"
export PATH="$DENO_INSTALL/bin:$PATH"
```

### Development

Start the project:

```sh {"background":"true","id":"01HFW6VKQYFGMC9MX7AQEYSQ6V"}
deno task start
```

Open the project in your browser:

```sh {"id":"01HFW6VKQYFGMC9MX7ARYFT9VT","interactive":"false"}
open http://localhost:8000/
```

This will watch the project directory and restart as necessary.

### Testing

We have added some basic components tests to verify that they render correctly, run them via:

```sh {"id":"01HFW6VKQYFGMC9MX7AVV0AKRB"}
deno test --no-check --allow-env --allow-read
```

### Preview Content

Doublecheck what's in the CMS to be published in the blog section:

```sh {"id":"01HFW6VKQYFGMC9MX7AW84CQWY","interactive":"false"}
curl "https://api-us-west-2.graphcms.com/v2/cksds5im94b3w01xq4hfka1r4/master?query=$(deno run -A query.ts)" --compressed 2>/dev/null \
  | jq -r '.[].posts[] | "\(.title) - by \(.authors[0].name), id: \(.id)"'
```

```sh {"id":"01HFW6VKQYFGMC9MX7AWSVQMFD"}
echo "Set up your deno environment"
export DENO_PROJECT_NAME="safe-hedgehog-53"
export DENO_ACCESS_TOKEN="ddp_6wpSbCUQbUoOEcBRC8DjmoxdrNipeu2OBKIj"
```

### Deployment

To deploy this project you need to have `deployctl` installed on your system. To
install, please run:

```sh {"id":"01HFW6VKQYFGMC9MX7AZTFHQBN"}
deno install \
  --allow-read --allow-write \
  --allow-env --allow-net --allow-run \
  --no-check \
  -r -f https://deno.land/x/deploy/deployctl.ts
```

Once installed successfully, create a
[new access token](https://dash.deno.com/account#access-tokens) and export it
into your environment:

```sh {"id":"01HFW6VKQYFGMC9MX7B3D6379D"}
echo "Set up your deno environment"
export DENO_PROJECT_NAME="<insert-project-name>"
export DENO_ACCESS_TOKEN="<insert-token-here>"
```

then you can run a preview deployment via:

```sh {"background":"true","id":"01HFW6VKQYFGMC9MX7B5RBE1GY"}
deployctl deploy \
    --project=$DENO_PROJECT_NAME \
    --exclude=node_modules \
    --import-map=import_map.json \
    --token=$DENO_ACCESS_TOKEN \
    main.ts
```

---

<p align="center"><small>Copyright 2022 © <a href="https://stateful.com/">Stateful</a> (<a href="https://discord.gg/BQm8zRCBUY">💬 Join Discord</a>) – Apache 2.0 License</small> </p>
