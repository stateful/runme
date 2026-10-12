package iam

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/runmedev/runme/v3/pkg/agent/config"
)

// staticClaims carries an authenticated principal separately from untrusted
// email/role claims. Only the verifier can populate principal.
type staticClaims struct {
	jwt.RegisteredClaims
	principal string
}

type pinnedKey struct {
	key       *ecdsa.PublicKey
	subject   string
	principal string
}

type staticVerifier struct {
	issuer   string
	audience string
	lifetime time.Duration
	skew     time.Duration
	keys     map[string]pinnedKey
}

func newStaticVerifier(cfg *config.StaticOIDCConfig) (*staticVerifier, error) {
	lifetime := cfg.MaxTokenLifetimeSeconds
	if lifetime == 0 {
		lifetime = 300
	}
	if strings.TrimSpace(cfg.Issuer) == "" || strings.TrimSpace(cfg.Audience) == "" || len(cfg.Keys) == 0 ||
		lifetime < 1 || lifetime > 3600 || cfg.ClockSkewSeconds < 0 || cfg.ClockSkewSeconds > 60 {
		return nil, fmt.Errorf("invalid static OIDC issuer, audience, keys, or time bounds")
	}
	v := &staticVerifier{
		issuer: cfg.Issuer, audience: cfg.Audience, lifetime: time.Duration(lifetime) * time.Second,
		skew: time.Duration(cfg.ClockSkewSeconds) * time.Second, keys: make(map[string]pinnedKey),
	}
	for _, k := range cfg.Keys {
		if k.Kid == "" || k.Algorithm != "ES256" || k.Subject == "" || k.Principal == "" || !filepath.IsAbs(k.PublicKeyFile) {
			return nil, fmt.Errorf("static OIDC keys require kid, ES256, subject, principal and an absolute publicKeyFile")
		}
		if _, exists := v.keys[k.Kid]; exists {
			return nil, fmt.Errorf("duplicate static OIDC kid")
		}
		pem, err := os.ReadFile(k.PublicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read static OIDC public key: %w", err)
		}
		publicKey, err := jwt.ParseECPublicKeyFromPEM(pem)
		if err != nil || publicKey.Curve != elliptic.P256() {
			return nil, fmt.Errorf("static OIDC requires a P-256 public PEM key")
		}
		v.keys[k.Kid] = pinnedKey{publicKey, k.Subject, k.Principal}
	}
	return v, nil
}

func (v *staticVerifier) verify(raw string) (*jwt.Token, error) {
	claims := &staticClaims{}
	var key pinnedKey
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing key id")
		}
		key, ok = v.keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key id")
		}
		return key.key, nil
	}, jwt.WithValidMethods([]string{"ES256"}), jwt.WithIssuer(v.issuer), jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(v.skew))
	// Keep all rejection messages independent of JWT bytes and caller claims.
	if err != nil || token == nil || !token.Valid {
		return nil, fmt.Errorf("invalid runner credential")
	}
	if claims.IssuedAt == nil || claims.NotBefore == nil || claims.ExpiresAt == nil || claims.Subject != key.subject ||
		!claims.ExpiresAt.After(claims.IssuedAt.Time) || claims.ExpiresAt.Sub(claims.IssuedAt.Time) > v.lifetime ||
		claims.NotBefore.After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("invalid runner credential claims")
	}
	claims.principal = key.principal
	return token, nil
}
