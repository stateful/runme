package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.uber.org/zap"
)

type Option func(http.RoundTripper) http.RoundTripper

func WithTokenGetter(getter func() (string, error)) Option {
	return setHeaderFn("Authorization", func() (string, error) {
		token, err := getter()
		if err != nil {
			return "", err
		}
		return "Bearer " + token, nil
	})
}

func WithUserAgent(version string) Option {
	return setHeaderFn("User-Agent", func() (string, error) {
		return fmt.Sprintf("stateful-cli/%s (%s; %s)", version, runtime.GOOS, runtime.GOARCH), nil
	})
}

func WithContentType(value string) Option {
	return setHeaderFn("Content-Type", func() (string, error) {
		return value, nil
	})
}

type responseWriter struct {
	*http.Response
	buf *bytes.Buffer
}

var _ http.ResponseWriter = (*responseWriter)(nil)

func (w responseWriter) Header() http.Header { return w.Response.Header }

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.Response.StatusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.buf.Write(data)
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.Response.Status = http.StatusText(statusCode)
	w.Response.StatusCode = statusCode
}

func WithChaosMonkey(srvErrRate, gqlErrRate float64) Option {
	return func(rt http.RoundTripper) http.RoundTripper {
		return funcTripper(func(r *http.Request) (*http.Response, error) {
			rnd := rand.Float64() //#nosec
			if rnd > srvErrRate+gqlErrRate {
				return rt.RoundTrip(r)
			}

			buf := new(bytes.Buffer)
			rw := &responseWriter{
				Response: &http.Response{
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Header:     make(http.Header),
					Body:       io.NopCloser(buf),
				},
				buf: buf,
			}

			// GraphQL error
			if rnd < gqlErrRate {
				data := struct {
					Data   interface{}   `json:"data"`
					Errors gqlerror.List `json:"errors"`
				}{
					Errors: gqlerror.List{
						&gqlerror.Error{
							Message: "chaos gql error",
						},
					},
				}
				if err := json.NewEncoder(rw).Encode(&data); err != nil {
					return nil, err
				}
				return rw.Response, nil
			}

			// Server error
			http.Error(rw, "welcome from chaos", http.StatusNotFound)

			return rw.Response, nil
		})
	}
}

func WithLogger(log *zap.Logger) Option {
	return func(rt http.RoundTripper) http.RoundTripper {
		return funcTripper(func(r *http.Request) (*http.Response, error) {
			start := time.Now()
			log.Debug(
				"send an API request",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
				zap.Duration("latency", time.Since(start)),
			)
			resp, err := rt.RoundTrip(r)
			if resp != nil {
				log.Debug(
					"received an API response",
					zap.Int("status", resp.StatusCode),
				)
			}
			return resp, err
		})
	}
}

func NewHTTPClient(client *http.Client, opts ...Option) *http.Client {
	if client == nil {
		client = &http.Client{
			Transport: http.DefaultTransport,
		}
	}
	for _, o := range opts {
		client.Transport = o(client.Transport)
	}
	return client
}

type funcTripper func(*http.Request) (*http.Response, error)

func (f funcTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func setHeaderFn(name string, valueGetter func() (string, error)) Option {
	return func(rt http.RoundTripper) http.RoundTripper {
		return funcTripper(func(r *http.Request) (*http.Response, error) {
			value, err := valueGetter()
			if err != nil {
				return nil, err
			}
			if r.Header.Get(name) == "" {
				r.Header.Set(name, value)
			}
			return rt.RoundTrip(r)
		})
	}
}
