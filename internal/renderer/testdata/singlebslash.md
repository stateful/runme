# Preview Content

Doublecheck what's in the CMS to be published in the blog section:

```sh {"id":"01HFW6VKQX9B4ZJH9TFJYWDQJ8","interactive":"false"}
$ curl "https://api-us-west-2.graphcms.com/v2/cksds5im94b3w01xq4hfka1r4/master?query=$(deno run -A query.ts)" --compressed 2>/dev/null \
  | jq -r '.[].posts[] | "\(.title) - by \(.authors[0].name), id: \(.id)"'
```
