package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/runmedev/runme/v3/pkg/agent/api"
	"github.com/runmedev/runme/v3/pkg/agent/config"
	"github.com/runmedev/runme/v3/pkg/agent/credentials"
	"github.com/runmedev/runme/v3/pkg/agent/iam"
)

type fakeGoogleMinter struct{ calls int }

func (m *fakeGoogleMinter) MintGoogleAccessToken(_ context.Context, req credentials.MintRequest) (credentials.GoogleAccessToken, error) {
	m.calls++
	return credentials.GoogleAccessToken{AccessToken: "google-test-token", TokenType: "Bearer", Scopes: req.Scopes}, nil
}

func TestGoogleCredentialHTTPBoundary(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "public.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))
	cfg := &config.AssistantServerConfig{Port: 9988, BindAddress: "127.0.0.1", CorsOrigins: []string{"*"}, OIDC: &config.OIDCConfig{Static: &config.StaticOIDCConfig{Issuer: "issuer", Audience: "runner", Keys: []config.StaticOIDCKey{{Kid: "key", Algorithm: "ES256", PublicKeyFile: path, Subject: "runme", Principal: "runme"}}}}}
	oidc, err := iam.NewOIDC(cfg.OIDC)
	require.NoError(t, err)
	checker, err := iam.NewChecker(api.IAMPolicy{Bindings: []api.IAMBinding{{Role: api.TokenCreatorRole, Members: []api.Member{{Kind: api.UserKind, Name: "runme"}}}}})
	require.NoError(t, err)
	now := time.Now().Unix()
	claims := jwt.MapClaims{"iss": "issuer", "aud": "runner", "sub": "runme", "iat": now, "nbf": now, "exp": now + 300}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	jwtToken.Header["kid"] = "key"
	raw, err := jwtToken.SignedString(key)
	require.NoError(t, err)
	m := &fakeGoogleMinter{}
	h, err := NewAuthMux(cfg, oidc)
	require.NoError(t, err)
	h.HandleProtected(googleMintPath, newGoogleTokenHandler(cfg, m), checker, api.TokenCreatorRole)
	for _, tt := range []struct {
		name, method, origin, host, authorization, query, body string
		status                                                 int
	}{
		{"success", "POST", "https://web.runme.dev", "localhost:9988", "Bearer " + raw, "", `{"scopes":["https://www.googleapis.com/auth/drive"]}`, 200},
		{"preflight", "OPTIONS", "https://web.runme.dev", "localhost:9988", "", "", "", 204},
		{"no_token", "POST", "https://web.runme.dev", "localhost:9988", "", "", "{}", 401},
		{"bad_token", "POST", "https://web.runme.dev", "localhost:9988", "Bearer bad", "", "{}", 401},
		{"other_https_origin", "POST", "https://other.example", "localhost:9988", "Bearer " + raw, "", "{}", 200},
		{"http_origin", "POST", "http://localhost:5173", "localhost:9988", "Bearer " + raw, "", "{}", 200},
		{"other_origin_no_token", "POST", "https://other.example", "localhost:9988", "", "", "{}", 401},
		{"http_preflight", "OPTIONS", "http://localhost:5173", "localhost:9988", "", "", "", 204},
		{"no_origin", "POST", "", "localhost:9988", "Bearer " + raw, "", "{}", 200},
		{"host", "POST", "https://web.runme.dev", "evil.example:9988", "Bearer " + raw, "", "{}", 403},
		{"query_auth", "POST", "https://web.runme.dev", "localhost:9988", "", "?authorization=" + url.QueryEscape("Bearer "+raw), "{}", 200},
		{"cookie_auth", "POST", "https://web.runme.dev", "localhost:9988", "", "", "{}", 200},
		{"arbitrary_target", "POST", "https://web.runme.dev", "localhost:9988", "Bearer " + raw, "", `{"serviceAccount":"other"}`, 400},
		{"trailing_json", "POST", "https://web.runme.dev", "localhost:9988", "Bearer " + raw, "", "{} {}", 400},
		{"null", "POST", "https://web.runme.dev", "localhost:9988", "Bearer " + raw, "", "null", 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := m.calls
			r := httptest.NewRequest(tt.method, "https://"+tt.host+googleMintPath+tt.query, strings.NewReader(tt.body))
			r.Header.Set("Origin", tt.origin)
			r.Header.Set("Authorization", tt.authorization)
			r.Header.Set("Content-Type", "application/json")
			if tt.name == "cookie_auth" {
				r.AddCookie(&http.Cookie{Name: iam.SessionCookieName, Value: raw})
			}
			if tt.method == "OPTIONS" {
				r.Header.Set("Access-Control-Request-Method", "POST")
				r.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tt.status, w.Code)
			if tt.status == http.StatusOK {
				require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			}
			if tt.origin != "" {
				require.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"))
			}
			require.NotContains(t, w.Body.String(), raw)
			if tt.status != 200 {
				require.Equal(t, before, m.calls)
			}
		})
	}
	_, err = (&iam.AuthContext{OIDC: oidc, Checker: checker, Role: api.RunnerUserRole}).AuthorizeBearerToken(context.Background(), "Bearer "+raw)
	require.ErrorIs(t, err, iam.ErrRoleDenied)
	cfg.GoogleTokenBroker = &config.GoogleTokenBrokerConfig{Enabled: true, Source: "workload_identity", TokenMode: "iam_credentials", ServiceAccountEmail: "runme@test.iam.gserviceaccount.com", AllowedScopes: []string{"https://www.googleapis.com/auth/drive"}}
	mux, err := NewAuthMux(cfg, oidc)
	require.NoError(t, err)
	s := &Server{serverConfig: cfg, checker: checker}
	require.NoError(t, s.registerGoogleCredentials(mux, oidc))
	require.Error(t, s.registerGoogleCredentials(mux, nil))

	for _, origins := range [][]string{nil, {"*"}, {"http://localhost:5173"}, {"https://web.runme.dev"}} {
		cfg.CorsOrigins = origins
		mux, err := NewAuthMux(cfg, oidc)
		require.NoError(t, err)
		require.NoError(t, s.registerGoogleCredentials(mux, oidc))
	}
	// A valid runner JWT is insufficient without the endpoint's token-creator role.
	runnerOnly, err := iam.NewChecker(api.IAMPolicy{Bindings: []api.IAMBinding{{Role: api.RunnerUserRole, Members: []api.Member{{Kind: api.UserKind, Name: "runme"}}}}})
	require.NoError(t, err)
	denied, err := NewAuthMux(cfg, oidc)
	require.NoError(t, err)
	denied.HandleProtected(googleMintPath, newGoogleTokenHandler(cfg, m), runnerOnly, api.TokenCreatorRole)
	before := m.calls
	r := httptest.NewRequest("POST", "http://localhost:9988"+googleMintPath, strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer "+raw)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	denied.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, before, m.calls)
	s.checker = nil
	require.ErrorContains(t, s.registerGoogleCredentials(mux, oidc), "explicit IAM policy")
	s.checker = checker
	cfg.BindAddress = "0.0.0.0"
	require.ErrorContains(t, s.registerGoogleCredentials(mux, oidc), "loopback")
}

// Query credentials accepted by shared auth must not enter wildcard CORS diagnostics.
func TestCORSDoesNotLogAuthorizationQuery(t *testing.T) {
	core, logs := observer.New(zapcore.Level(-10))
	restore := zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(restore)
	h := SetOriginHeader(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	r := httptest.NewRequest("POST", "http://localhost:9988"+googleMintPath+"?authorization=Bearer+secret-credential", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusNoContent, w.Code)
	require.NotEmpty(t, logs.All())
	for _, entry := range logs.All() {
		require.NotContains(t, entry.ContextMap(), "url")
		require.NotContains(t, entry.Message, "secret-credential")
		for _, value := range entry.ContextMap() {
			require.NotContains(t, fmt.Sprint(value), "secret-credential")
		}
	}
}
