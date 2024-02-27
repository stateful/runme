package graphql

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stateful/runme/v3/internal/client/graphql/query"
	"github.com/stretchr/testify/require"
)

func TestAPIError(t *testing.T) {
	t.Run("noErrors", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(rw, `{
	"data": {
		"createUserAnnotation": {
			"userAnnotation": {}
		}
	}
}`)
		})
		s := httptest.NewServer(mux)
		defer s.Close()
		client, err := New(s.URL, nil)
		require.NoError(t, err)

		resp, err := query.AddNote(context.Background(), client, query.CreateUserAnnotationInput{}, query.TrackInput{})
		err = NewAPIError(err, resp.CreateUserAnnotation.GetErrors())
		require.NoError(t, err)
	})

	t.Run("gqlerrors", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(rw, `{
	"data": {},
	"errors": [
		{
			"message": "Variable '$input' got invalid value 'dasdada' at 'input.projectId'; Value cannot represent a UUID: \"dasdada\".",
			"locations": [
				{
					"line": 2,
					"column": 19
				}
			],
			"path": null
		}
	]
}`)
		})
		s := httptest.NewServer(mux)
		defer s.Close()
		client, err := New(s.URL, nil)
		require.NoError(t, err)

		resp, err := query.AddNote(context.Background(), client, query.CreateUserAnnotationInput{}, query.TrackInput{})
		err = NewAPIError(err, resp.CreateUserAnnotation.GetErrors())
		require.EqualError(t, err, "input:2: Variable '$input' got invalid value 'dasdada' at 'input.projectId'; Value cannot represent a UUID: \"dasdada\".\n")
	})

	t.Run("userErrors", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(rw, `{
	"data": {
		"createUserAnnotation": {
			"userAnnotation": {},
			"errors": [
				{
					"message": "failed to create annotation",
					"field": ["content"]
				}
			]
		}
	}
}`)
		})
		s := httptest.NewServer(mux)
		defer s.Close()
		client, err := New(s.URL, nil)
		require.NoError(t, err)

		resp, err := query.AddNote(context.Background(), client, query.CreateUserAnnotationInput{}, query.TrackInput{})
		err = NewAPIError(err, resp.CreateUserAnnotation.GetErrors())
		require.EqualError(t, err, "error \"failed to create annotation\" affected content fields")
	})
}
