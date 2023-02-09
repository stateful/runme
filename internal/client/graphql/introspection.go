package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"

	"github.com/Khan/genqlient/graphql"
)

// introspectionQueryTpl comes from
// https://github.com/graphql/graphql-js/blob/0276d685b51262686c841763ba0b6e71103f64f3/src/utilities/introspectionQuery.js
const introspectionQueryTpl = `
query IntrospectionQuery {
  __schema {
	queryType { name }
	mutationType { name }
	subscriptionType { name }
	types {
      ...FullType
	}
	directives {
	  name
	  {{ if .InclDescriptions }}description{{ end }}
	  locations
	  args {
	    ...InputValue
	  }
	}
  }
}

fragment FullType on __Type {
  kind
  name
  {{ if .InclDescriptions }}description{{ end }}
  fields(includeDeprecated: true) {
    name
    {{ if .InclDescriptions }}description{{ end }}
    args {
      ...InputValue
	}
	type {
	  ...TypeRef
	}
	isDeprecated
	deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
	{{ if .InclDescriptions }}description{{ end }}
	isDeprecated
	deprecationReason
  }
  possibleTypes {
	...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  {{ if .InclDescriptions }}description{{ end }}
  type { ...TypeRef }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
	  name
	  ofType {
	    kind
	    name
	    ofType {
	      kind
		  name
		  ofType {
	        kind
			name
			ofType {
			  kind
			  name
			  ofType {
			    kind
			    name
			  }
			}
		  }
		}
	  }
	}
  }
}
`

type introspectionQueryOptions struct {
	InclDescriptions bool
}

func introspectionQuery(opts introspectionQueryOptions) string {
	t := template.Must(template.New("introspectionQuery").Parse(introspectionQueryTpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, opts); err != nil {
		panic(err)
	}
	return buf.String()
}

func (c *Client) IntrospectionQuery(ctx context.Context) (json.RawMessage, error) {
	q := introspectionQuery(introspectionQueryOptions{InclDescriptions: false})
	var result graphql.Response
	err := c.Client.MakeRequest(ctx, &graphql.Request{Query: q, OpName: "IntrospectionQuery"}, &result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result.Data)
}
