package owl

import (
	"os"
	"strings"
	"testing"

	"github.com/stateful/godotenv"
	"github.com/stretchr/testify/require"
)

func TestMapSpec(t *testing.T) {
	testCases := map[string]struct {
		Values   map[string]string
		Comments map[string]string
		Expected Specs
	}{
		"EmptyComments": {
			Comments: map[string]string{},
			Expected: Specs{},
		},
		"WithSpecs": {
			Values: map[string]string{
				"KEY1": "KEY1",
				"KEY2": "KEY2",
				"KEY3": "KEY3",
				"KEY4": "KEY4",
				"KEY5": "",
			},
			Comments: map[string]string{
				"KEY1": "",
				"KEY2": "Plain",
				"KEY3": "Password",
				"KEY4": "Secret",
				"KEY5": "Plain",
			},
			Expected: Specs{
				"KEY1": {Name: SpecNameOpaque, Valid: false},
				"KEY2": {Name: SpecNamePlain, Valid: true},
				"KEY3": {Name: SpecNamePassword, Valid: true},
				"KEY4": {Name: SpecNameSecret, Valid: true},
				"KEY5": {Name: SpecNamePlain},
			},
		},
		"WithRequiredSpecs": {
			Values: map[string]string{
				"KEY1": "KEY1",
				"KEY2": "KEY2",
				"KEY3": "KEY3",
				"KEY4": "KEY4",
			},
			Comments: map[string]string{
				"KEY1": "!",
				"KEY2": "Plain!",
				"KEY3": "Password!",
				"KEY4": "Secret!",
			},
			Expected: Specs{
				"KEY1": {Name: SpecNameOpaque, Valid: true, Required: true},
				"KEY2": {Name: SpecNamePlain, Valid: true, Required: true},
				"KEY3": {Name: SpecNamePassword, Valid: true, Required: true},
				"KEY4": {Name: SpecNameSecret, Valid: true, Required: true},
			},
		},
		"WithParams": {
			Values: map[string]string{
				"KEY1": "1234567890",
				"KEY2": "1234567890",
			},
			Comments: map[string]string{
				"KEY1": `Password!:{"length":10}`,
				"KEY2": `Password!:{"length":9}`,
			},
			Expected: Specs{
				"KEY1": {Name: SpecNamePassword, Required: true, Valid: true},
				"KEY2": {Name: SpecNamePassword, Required: true},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			specs := ParseRawSpec(tc.Values, tc.Comments)

			if len(specs) != len(tc.Expected) {
				t.Errorf("%s Unexpected number of specs. Expected %d, got %d", name, len(tc.Expected), len(specs))
			}

			for key, expectedSpec := range tc.Expected {
				actualSpec, ok := specs[key]
				if !ok {
					t.Errorf("%s Key %s missing in returned specs", name, key)
				} else if actualSpec != expectedSpec {
					t.Errorf("%s Unexpected spec for key %s. Expected %+v, got %+v", name, key, expectedSpec, actualSpec)
				}
			}
		})
	}
}

func TestValues(t *testing.T) {
	content, err := os.ReadFile("testdata/values/.env.example")
	require.NoError(t, err)

	values, comments, err := godotenv.UnmarshalBytesWithComments(content)
	if err != nil {
		t.Fatalf("failed to unmarshal content: %v", err)
	}

	require.Equal(t, map[string]string{
		"ALLOWED_URL_PATTERNS":                         "Allowed URL patterns for the frontend",
		"API_URL":                                      "Url for the backend API",
		"AUTH_DEV_SKIP_EXP":                            "Skip expiration validation for Auth0. Only dev purposes",
		"AUTH0_AUDIENCE":                               "Audience for Auth0",
		"AUTH0_CLIENT_ID":                              "Client ID for Auth0",
		"AUTH0_COOKIE_DOMAIN":                          "Cookie domain for Auth0",
		"AUTH0_DEV_ID":                                 "Auth0 Dev ID used for the seed",
		"AUTH0_DOMAIN":                                 "Domain for Auth0",
		"AUTH0_MANAGEMENT_AUDIENCE":                    "Audience for Auth0 Management API",
		"AUTH0_MANAGEMENT_CLIENT_ID":                   "Client ID for Auth0 Management API",
		"AUTH0_MANAGEMENT_CLIENT_SECRET":               "Client Secret for Auth0 Management API",
		"AUTH0_WEBHOOK_TOKEN":                          "Token for Auth0 webhook used when creating users",
		"CORS_ORIGINS":                                 "CORS origins for the frontend",
		"CRYPTOGRAPHY_KEY":                             "Key to encrypt/decrypt cell outputs",
		"CUSTOMER_IO_API_KEY":                          "API Key for Customer.io",
		"CUSTOMER_IO_SITE_ID":                          "Site ID for Customer.io",
		"DATABASE_URL":                                 "Database URL for the backend",
		"EXTERNAL_VECTOR_DB_COLLECTION":                "Collection for the external vector DB",
		"FRONTEND_URL":                                 "URL of the frontend",
		"GITHUB_APP_API_THROTTLING_CLUSTERING_ENABLED": "Enable API throttling and clustering for GitHub API",
		"GITHUB_APP_ID":                                "GitHub App ID. Will work even if this is not a real ID",
		"GITHUB_APP_PRIVATE_KEY":                       "Private key for GitHub App. Will work even if this is not a real key",
		"GITHUB_WEBHOOK_SECRET":                        "Secret for GitHub Webhook",
		"IDP_REDIRECT_URL":                             "Redirect URL for the IDE Login",
		"MIXPANEL_TOKEN":                               "Token for Mixpanel",
		"NODE_ENV":                                     "NodeJS Environment",
		"OPENAI_API_KEY":                               "API Key for OpenAI",
		"OPENAI_ORG_ID":                                "Organization ID for OpenAI",
		"PORT":                                         "Port for the backend",
		"RATE_LIMIT_MAX":                               "Max requests per time window",
		"RATE_LIMIT_TIME_WINDOW":                       "Time window for rate limiting",
		"REDIS_HOST":                                   "Redis host",
		"REDIS_PORT":                                   "Redis port",
		"REDWOOD_ENV_DEBUG_IDE":                        "Flag to enable debug mode for the IDE",
		"REDWOOD_ENV_GITHUB_APP":                       "Id of the GitHub App",
		"REDWOOD_ENV_INSIGHT_ENABLED":                  "Flag to enable insights",
		"REDWOOD_ENV_INSTRUMENTATION_KEY":              "Instrumentation Key for Application Insights",
		"RESEND_API_KEY":                               "API Key for Resend",
		"SENTRY_DSN":                                   "DSN for Sentry",
		"SLACK_CLIENT_ID":                              "Client ID for Slack",
		"SLACK_CLIENT_SECRET":                          "Client Secret for Slack",
		"SLACK_REDIRECT_URL":                           "Redirect URL for Slack. Use a tunnel with ngrok",
		"TEST_DATABASE_URL":                            "Database URL for the tests",
		"VECTOR_DB_COLLECTION":                         "Collection for the vector DB",
		"VECTOR_DB_URL":                                "URL for the vector DB",
		"WEB_PORT":                                     "Port for the web app",
	}, values)

	require.Equal(t, map[string]string{
		"ALLOWED_URL_PATTERNS":                         "Plain!",
		"API_URL":                                      "Plain!",
		"AUTH0_AUDIENCE":                               "Plain!",
		"AUTH0_CLIENT_ID":                              "Plain!",
		"AUTH0_COOKIE_DOMAIN":                          "Plain",
		"AUTH0_DEV_ID":                                 "Plain",
		"AUTH0_DOMAIN":                                 "Plain!",
		"AUTH0_MANAGEMENT_AUDIENCE":                    "Plain!",
		"AUTH0_MANAGEMENT_CLIENT_ID":                   "Plain!",
		"AUTH0_MANAGEMENT_CLIENT_SECRET":               "Secret!",
		"AUTH0_WEBHOOK_TOKEN":                          "Secret!",
		"AUTH_DEV_SKIP_EXP":                            "Plain",
		"CORS_ORIGINS":                                 "Plain",
		"CRYPTOGRAPHY_KEY":                             "Secret!",
		"CUSTOMER_IO_API_KEY":                          "Secret",
		"CUSTOMER_IO_SITE_ID":                          "Plain",
		"DATABASE_URL":                                 "DatabaseUrl!",
		"EXTERNAL_VECTOR_DB_COLLECTION":                "Plain",
		"FRONTEND_URL":                                 "Plain!",
		"GITHUB_APP_API_THROTTLING_CLUSTERING_ENABLED": "Plain",
		"GITHUB_APP_ID":                                "Plain!",
		"GITHUB_APP_PRIVATE_KEY":                       "Secret!",
		"GITHUB_WEBHOOK_SECRET":                        "Secret!",
		"IDP_REDIRECT_URL":                             "Plain!",
		"MIXPANEL_TOKEN":                               "Secret",
		"NODE_ENV":                                     "Plain",
		"OPENAI_API_KEY":                               "Secret!",
		"OPENAI_ORG_ID":                                "Secret!",
		"PORT":                                         "Plain!",
		"RATE_LIMIT_MAX":                               "Plain",
		"RATE_LIMIT_TIME_WINDOW":                       "Plain",
		"REDIS_HOST":                                   "Redis!",
		"REDIS_PORT":                                   "Redis!",
		"REDWOOD_ENV_DEBUG_IDE":                        "Plain",
		"REDWOOD_ENV_GITHUB_APP":                       "Plain!",
		"REDWOOD_ENV_INSIGHT_ENABLED":                  "Plain",
		"REDWOOD_ENV_INSTRUMENTATION_KEY":              "Secret",
		"RESEND_API_KEY":                               "Secret",
		"SENTRY_DSN":                                   "Secret",
		"SLACK_CLIENT_ID":                              "Plain!",
		"SLACK_CLIENT_SECRET":                          "Secret!",
		"SLACK_REDIRECT_URL":                           "Plain!",
		"TEST_DATABASE_URL":                            "DatabaseUrl",
		"VECTOR_DB_COLLECTION":                         "Plain!",
		"VECTOR_DB_URL":                                "Secret!",
		"WEB_PORT":                                     "Plain!",
	}, comments)
}

func TestUnmarshal(t *testing.T) {
	lines := []string{
		"naked= # Plain",
		`quotedEmpty="" # Plain`,
		`quoted="Foo bar baz" # Plain`,
		`unquoted=unquoted value # Plain`,
		`database=unquoted value # DatabaseUrl`,
	}

	expectedValues := map[string]string{
		"naked":       "",
		"quotedEmpty": "",
		"quoted":      "Foo bar baz",
		"unquoted":    "unquoted value",
		"database":    "unquoted value",
	}
	expectedComments := map[string]string{
		"naked":       "Plain",
		"quotedEmpty": "Plain",
		"quoted":      "Plain",
		"unquoted":    "Plain",
		"database":    "DatabaseUrl",
	}

	bytes := []byte(strings.Join(lines, "\n"))
	values, comments, err := godotenv.UnmarshalBytesWithComments(bytes)
	if err != nil {
		t.Errorf("Unable to parse content %s ", bytes)
	}

	require.Equal(t, expectedValues, values)
	require.Equal(t, expectedComments, comments)
}
