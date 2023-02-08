package cmd

import "github.com/spf13/pflag"

const (
	apiURLF      = "api-url"
	authURLF     = "auth-url"
	traceF       = "trace"
	traceAllF    = "trace-all"
	enableChaosF = "enable-chaos"
	apiTokenF    = "api-token"
)

var (
	apiBaseURL  string
	authBaseURL string
	trace       bool
	traceAll    bool
	enableChaos bool
	apiToken    string
)

func getAPIURL() string    { return apiBaseURL }
func getAuthURL() string   { return authBaseURL }
func getTrace() bool       { return trace || traceAll }
func getTraceAll() bool    { return traceAll }
func getEnableChaos() bool { return enableChaos }
func getAPIToken() string  { return apiToken }

func setAPIFlags(flagSet *pflag.FlagSet) {
	flagSet.StringVar(&authBaseURL, authURLF, defaultAuthURL, "backend URL to authorize you")
	flagSet.StringVar(&apiBaseURL, apiURLF, "https://api.stateful.com", "backend URL with API")
	flagSet.StringVar(&apiToken, apiTokenF, "", "api token")
	flagSet.BoolVar(&trace, traceF, false, "trace HTTP calls")
	flagSet.BoolVar(&traceAll, traceAllF, false, "trace all HTTP calls including authentication (it might leak sensitive data to output)")
	flagSet.BoolVar(&enableChaos, enableChaosF, false, "enable Chaos Monkey mode for GraphQL requests")
}
