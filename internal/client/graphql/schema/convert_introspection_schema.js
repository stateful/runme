const { buildClientSchema, printSchema } = require("graphql");
const fs = require("fs");

const introspectionSchemaResult = JSON.parse(fs.readFileSync("introspection_query_result.json"));
const graphqlSchemaObj = buildClientSchema(introspectionSchemaResult);
const sdlString = printSchema(graphqlSchemaObj);
fs.writeFileSync("schema.graphql", sdlString);
