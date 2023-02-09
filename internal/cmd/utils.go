package cmd

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/henvic/httpretty"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/auth"
	"github.com/stateful/runme/internal/client"
	"github.com/stateful/runme/internal/client/graphql/query"
	"github.com/stateful/runme/internal/log"
	"github.com/stateful/runme/internal/version"
	"golang.org/x/oauth2"
)

func oauthConfig(authBaseURL string) oauth2.Config {
	return oauth2.Config{
		ClientID:     "bf568e40cfbd1c1261a9",
		ClientSecret: "0de10314c28b754d0cedbf34d081c990865e1363",
		Scopes:       []string{"read:user", "user:email"},
		Endpoint: oauth2.Endpoint{
			// These URLs are modeleted after Github API.
			// If we want to switch between various IdP,
			// we should allow more verbose configuration.
			AuthURL:  authBaseURL + "/login/oauth/authorize",
			TokenURL: authBaseURL + "/login/oauth/access_token",
		},
	}
}

func newAuthClient() *http.Client {
	opts := []client.Option{
		client.WithUserAgent(version.BuildVersion),
		client.WithContentType("application/json"),
	}
	if getTraceAll() {
		opts = append(opts, httpLoggerMiddleware(os.Stderr))
	}
	return client.NewHTTPClient(nil, opts...)
}

var (
	authEnv        auth.Env        // overwritten only in unit tests; when nil a default env will be used
	authAuthorizer auth.Authorizer // overwritten only in unit tests
	tokenStorage   = &auth.DiskStorage{Location: getDefaultConfigHome()}
)

// authorizerWithEnv is a decorator that can return a token
// from the environment variables.
type authorizerWithEnv struct {
	auth.Authorizer
}

func (a *authorizerWithEnv) GetToken(ctx context.Context) (string, error) {
	if apiToken := getAPIToken(); apiToken != "" {
		return apiToken, nil
	}
	return a.Authorizer.GetToken(ctx)
}

func newAuth() auth.Authorizer {
	if authAuthorizer != nil {
		return authAuthorizer
	}

	conf := oauthConfig(getAuthURL())
	opts := []auth.Opts{}

	if getTraceAll() {
		opts = append(opts, auth.WithClient(newAuthClient()))
	}

	if authEnv != nil {
		opts = append(opts, auth.WithEnv(authEnv))
	}

	return &authorizerWithEnv{
		Authorizer: auth.New(conf, getAPIURL(), tokenStorage, opts...),
	}
}

func newAPIClient(ctx context.Context) *http.Client {
	opts := []client.Option{
		client.WithTokenGetter(func() (string, error) {
			return newAuth().GetToken(ctx)
		}),
		client.WithUserAgent(version.BuildVersion),
	}
	if getTrace() {
		opts = append(opts, httpLoggerMiddleware(os.Stderr))
	}
	if l := log.Get(); l != nil {
		opts = append(opts, client.WithLogger(l.Named("APIClient")))
	}
	if getEnableChaos() {
		log.Get().Debug("enabling chaos monkey")
		opts = append(opts, client.WithChaosMonkey(0.1, 0.1))
	}
	return client.NewHTTPClient(nil, opts...)
}

func graphqlEndpoint() string {
	return getAPIURL() + "/graphql"
}

func isTerminal() bool {
	return isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
}

func httpLoggerMiddleware(out io.Writer) func(http.RoundTripper) http.RoundTripper {
	logger := &httpretty.Logger{
		Time:            true,
		TLS:             false,
		Colors:          isTerminal(),
		RequestHeader:   true,
		RequestBody:     true,
		ResponseHeader:  true,
		ResponseBody:    true,
		Formatters:      []httpretty.Formatter{&httpretty.JSONFormatter{}},
		MaxResponseBody: 50000,
	}
	logger.SetOutput(out)
	return logger.RoundTripper
}

func recoverableWithLogin(err error) bool {
	if err == nil {
		return false
	}
	// This error comes from the CLI's auth package.
	if errors.Is(err, auth.ErrNotFound) {
		return true
	}
	// TODO(mxs): implement this for REST client
	// And this from the Stateful's API.
	// var errResponse *rest.ErrorResponse
	// return errors.As(err, &errResponse) && errResponse.StatusCode == http.StatusUnauthorized

	return false
}

func trackInputFromCmd(cmd *cobra.Command, args []string) query.TrackInput {
	fragments := append([]string{"cli", "command"}, strings.Split(cmd.CommandPath(), " ")...)
	fragments = append(fragments, args...)

	return query.TrackInput{
		Events: []query.TrackEvent{
			{
				Event: strings.Join(fragments, "/"),
			},
		},
	}
}
