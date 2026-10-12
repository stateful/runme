package iam

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/runme/v3/pkg/agent/api"
	"github.com/runmedev/runme/v3/pkg/agent/config"
)

func TestStaticOIDC(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "public.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))
	cfg := &config.StaticOIDCConfig{
		Issuer: "urn:runme:browser", Audience: "urn:runme:runner", MaxTokenLifetimeSeconds: 300,
		Keys: []config.StaticOIDCKey{{Kid: "key1", Algorithm: "ES256", PublicKeyFile: path, Subject: "runme", Principal: "runme"}},
	}
	oidc, err := NewOIDC(&config.OIDCConfig{Static: cfg})
	require.NoError(t, err)
	require.True(t, oidc.DoClientExchange())
	checker, err := NewChecker(api.IAMPolicy{Bindings: []api.IAMBinding{{Role: api.TokenCreatorRole, Members: []api.Member{{Kind: api.UserKind, Name: "runme"}}}}})
	require.NoError(t, err)
	base := func() jwt.MapClaims {
		now := time.Now().Unix()
		return jwt.MapClaims{"iss": cfg.Issuer, "aud": cfg.Audience, "sub": "runme", "iat": now, "nbf": now, "exp": now + 300, "email": "admin", "role": api.RunnerUserRole}
	}
	sign := func(claims jwt.MapClaims, kid string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		token.Header["kid"] = kid
		raw, err := token.SignedString(key)
		require.NoError(t, err)
		return raw
	}
	valid := sign(base(), "key1")
	token, err := oidc.verifyToken(valid)
	require.NoError(t, err)
	principal, err := checker.GetPrincipal(token)
	require.NoError(t, err)
	require.Equal(t, "runme", principal)
	require.False(t, checker.Check(principal, api.RunnerUserRole))
	for _, field := range []string{"iss", "aud", "sub", "iat", "nbf", "exp"} {
		t.Run("missing_"+field, func(t *testing.T) {
			c := base()
			delete(c, field)
			_, err := oidc.verifyToken(sign(c, "key1"))
			require.Error(t, err)
		})
	}
	for name, change := range map[string]func(jwt.MapClaims){
		"issuer": func(c jwt.MapClaims) { c["iss"] = "other" }, "audience": func(c jwt.MapClaims) { c["aud"] = "other" },
		"subject": func(c jwt.MapClaims) { c["sub"] = "admin" }, "expiry": func(c jwt.MapClaims) { c["exp"] = time.Now().Unix() - 10 },
		"future": func(c jwt.MapClaims) { c["iat"] = time.Now().Unix() + 100 }, "lifetime": func(c jwt.MapClaims) { c["exp"] = time.Now().Unix() + 1000 },
		"malformed": func(c jwt.MapClaims) { c["exp"] = "bad" },
	} {
		t.Run(name, func(t *testing.T) {
			c := base()
			change(c)
			_, err := oidc.verifyToken(sign(c, "key1"))
			require.Error(t, err)
		})
	}
	_, err = oidc.verifyToken(sign(base(), "unknown"))
	require.Error(t, err)
	_, err = oidc.verifyToken(valid[:len(valid)-8] + "tampered")
	require.Error(t, err)
	_, err = (&AuthContext{OIDC: oidc}).AuthorizeBearerToken(context.Background(), "Bearer invalid")
	require.Error(t, err)
	_, err = NewOIDC(&config.OIDCConfig{Static: cfg, Google: &config.GoogleOIDCConfig{}})
	require.Error(t, err)
	cfg.Keys = append(cfg.Keys, cfg.Keys[0])
	_, err = NewOIDC(&config.OIDCConfig{Static: cfg})
	require.Error(t, err)
}
