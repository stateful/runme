package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

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
	cfg := &config.AssistantServerConfig{Port: 9988, BindAddress: "127.0.0.1", CorsOrigins: []string{"https://web.runme.dev"}, OIDC: &config.OIDCConfig{Static: &config.StaticOIDCConfig{Issuer: "issuer", Audience: "runner", Keys: []config.StaticOIDCKey{{Kid: "key", Algorithm: "ES256", PublicKeyFile: path, Subject: "runme", Principal: "runme"}}}}}
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
	h := newGoogleTokenHandler(cfg, oidc, checker, m)
	for _, tt := range []struct {
		name, method, origin, host, authorization, query, body string
		status                                                 int
	}{
		{"success", "POST", "https://web.runme.dev", "localhost:9988", "Bearer " + raw, "", `{"scopes":["https://www.googleapis.com/auth/drive"]}`, 200},
		{"preflight", "OPTIONS", "https://web.runme.dev", "localhost:9988", "", "", "", 204},
		{"no_token", "POST", "https://web.runme.dev", "localhost:9988", "", "", "{}", 401},
		{"bad_token", "POST", "https://web.runme.dev", "localhost:9988", "Bearer bad", "", "{}", 401},
		{"origin", "POST", "https://evil.example", "localhost:9988", "Bearer " + raw, "", "{}", 403},
		{"no_origin", "POST", "", "localhost:9988", "Bearer " + raw, "", "{}", 403},
		{"host", "POST", "https://web.runme.dev", "evil.example:9988", "Bearer " + raw, "", "{}", 403},
		{"query", "POST", "https://web.runme.dev", "localhost:9988", "Bearer " + raw, "?authorization=secret", "{}", 400},
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
			r.Header.Set("Cookie", "agent-session="+raw)
			if tt.method == "OPTIONS" {
				r.Header.Set("Access-Control-Request-Method", "POST")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tt.status, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
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
	cfg.CorsOrigins = []string{"*"}
	require.Error(t, s.registerGoogleCredentials(mux, oidc))
}
