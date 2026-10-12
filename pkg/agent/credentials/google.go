// Package credentials issues short-lived Google access tokens under an explicit
// server policy. It never accepts browser-supplied credential sources or targets.
package credentials

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/runmedev/runme/v3/pkg/agent/config"
)

var (
	ErrInvalidRequest  = errors.New("invalid token request")
	ErrScopeDenied     = errors.New("requested scope is not allowed")
	ErrUnavailable     = errors.New("google workload credentials or token service unavailable")
	ErrInvalidResponse = errors.New("invalid Google token response")
)

// MintRequest exposes only scope and lifetime selection to the caller.
type MintRequest struct {
	Scopes          []string `json:"scopes"`
	LifetimeSeconds *int     `json:"lifetimeSeconds,omitempty"`
}

// GoogleAccessToken contains only the final, requested-scope credential.
type GoogleAccessToken struct {
	AccessToken         string    `json:"accessToken"`
	TokenType           string    `json:"tokenType"`
	ExpiresAt           time.Time `json:"expiresAt"`
	ServiceAccountEmail string    `json:"serviceAccountEmail"`
	Scopes              []string  `json:"scopes"`
}

type pendingMint struct {
	done  chan struct{}
	token GoogleAccessToken
	err   error
}

// GoogleBroker caches bounded scope/lifetime combinations and coalesces callers.
type GoogleBroker struct {
	cfg     config.GoogleTokenBrokerConfig
	allowed map[string]bool
	mu      sync.Mutex
	cache   map[string]GoogleAccessToken
	pending map[string]*pendingMint
	mint    func(context.Context, []string, int) (GoogleAccessToken, error)
	source  func(context.Context) (oauth2.TokenSource, error)
}

// NewGoogleBroker validates deployment policy without fetching credentials.
// Metadata can be briefly unavailable while a pod starts; acquisition is lazy.
func NewGoogleBroker(cfg config.GoogleTokenBrokerConfig) (*GoogleBroker, error) {
	if cfg.Source != "workload_identity" || cfg.TokenMode != "iam_credentials" ||
		!regexp.MustCompile(`^[a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.iam\.gserviceaccount\.com$`).MatchString(cfg.ServiceAccountEmail) {
		return nil, fmt.Errorf("broker requires workload_identity, iam_credentials and a service account email")
	}
	if cfg.DefaultLifetimeSeconds == 0 {
		cfg.DefaultLifetimeSeconds = 900
	}
	if cfg.MaxLifetimeSeconds == 0 {
		cfg.MaxLifetimeSeconds = 3600
	}
	if cfg.DefaultLifetimeSeconds < 60 || cfg.DefaultLifetimeSeconds > cfg.MaxLifetimeSeconds || cfg.MaxLifetimeSeconds > 3600 {
		return nil, fmt.Errorf("invalid broker lifetime policy")
	}
	if len(cfg.AllowedScopes) == 0 || len(cfg.AllowedScopes) > 32 {
		return nil, fmt.Errorf("broker requires 1 to 32 allowed scopes")
	}
	b := &GoogleBroker{cfg: cfg, allowed: map[string]bool{}, cache: map[string]GoogleAccessToken{}, pending: map[string]*pendingMint{}}
	for _, scope := range cfg.AllowedScopes {
		if !validScope(scope) {
			return nil, fmt.Errorf("invalid broker scope policy")
		}
		b.allowed[scope] = true
	}
	b.mint = b.mintGoogle
	b.source = workloadTokenSource
	return b, nil
}

func validScope(scope string) bool {
	return strings.HasPrefix(scope, "https://www.googleapis.com/auth/") && len(scope) <= 256 && !strings.ContainsAny(scope, " \t\r\n")
}

// MintGoogleAccessToken validates policy before accessing any cache or upstream.
func (b *GoogleBroker) MintGoogleAccessToken(ctx context.Context, req MintRequest) (GoogleAccessToken, error) {
	if len(req.Scopes) == 0 || len(req.Scopes) > 32 {
		return GoogleAccessToken{}, ErrInvalidRequest
	}
	scopes := slices.Clone(req.Scopes)
	for _, scope := range scopes {
		if !validScope(scope) {
			return GoogleAccessToken{}, ErrInvalidRequest
		}
		if !b.allowed[scope] {
			return GoogleAccessToken{}, ErrScopeDenied
		}
	}
	slices.Sort(scopes)
	scopes = slices.Compact(scopes)
	lifetime := b.cfg.DefaultLifetimeSeconds
	if req.LifetimeSeconds != nil {
		lifetime = *req.LifetimeSeconds
	}
	if lifetime < 60 || lifetime > b.cfg.MaxLifetimeSeconds {
		return GoogleAccessToken{}, ErrInvalidRequest
	}
	key := fmt.Sprintf("%d:%s", lifetime, strings.Join(scopes, " "))
	b.mu.Lock()
	if token, ok := b.cache[key]; ok && time.Until(token.ExpiresAt) > time.Duration(lifetime/5)*time.Second && time.Until(token.ExpiresAt) <= time.Duration(lifetime)*time.Second {
		b.mu.Unlock()
		token.Scopes = slices.Clone(token.Scopes)
		return token, nil
	}
	if pending, ok := b.pending[key]; ok {
		b.mu.Unlock()
		select {
		case <-ctx.Done():
			return GoogleAccessToken{}, ctx.Err()
		case <-pending.done:
			token := pending.token
			token.Scopes = slices.Clone(token.Scopes)
			return token, pending.err
		}
	}
	// Limit concurrent distinct combinations as well as retained cache entries.
	if len(b.pending) >= 16 {
		b.mu.Unlock()
		return GoogleAccessToken{}, ErrUnavailable
	}
	pending := &pendingMint{done: make(chan struct{})}
	b.pending[key] = pending
	b.mu.Unlock()
	mintCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	token, err := b.mint(mintCtx, scopes, lifetime)
	cancel()
	b.mu.Lock()
	if err == nil {
		if len(b.cache) >= 128 {
			clear(b.cache)
		}
		b.cache[key] = token
	}
	pending.token, pending.err = token, err
	delete(b.pending, key)
	close(pending.done)
	b.mu.Unlock()
	token.Scopes = slices.Clone(token.Scopes)
	return token, err
}

// workloadTokenSource deliberately rejects local user ADC and service-account
// JSON keys. An external-account config is operator-controlled, not request data.
func workloadTokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, ErrUnavailable
	}
	if len(creds.JSON) > 0 {
		var typ struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(creds.JSON, &typ) != nil || typ.Type != "external_account" {
			return nil, ErrUnavailable
		}
	}
	return creds.TokenSource, nil
}

func (b *GoogleBroker) mintGoogle(ctx context.Context, scopes []string, lifetime int) (GoogleAccessToken, error) {
	source, err := b.source(ctx)
	if err != nil {
		return GoogleAccessToken{}, err
	}
	client := oauth2.NewClient(ctx, source)
	client.Timeout = 15 * time.Second
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	body, err := json.Marshal(struct {
		Scope    []string `json:"scope"`
		Lifetime string   `json:"lifetime"`
	}{scopes, fmt.Sprintf("%ds", lifetime)})
	if err != nil {
		return GoogleAccessToken{}, ErrInvalidRequest
	}
	endpoint := "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + url.PathEscape(b.cfg.ServiceAccountEmail) + ":generateAccessToken"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return GoogleAccessToken{}, ErrInvalidRequest
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	// Retry one transient upstream status under the same request deadline. Never
	// retry caller/IAM policy failures, and never reflect upstream response bodies.
	if err == nil && (resp.StatusCode == 429 || resp.StatusCode >= 500) {
		_ = resp.Body.Close()
		select {
		case <-ctx.Done():
			return GoogleAccessToken{}, ErrUnavailable
		case <-time.After(200 * time.Millisecond):
		}
		req.Body, err = req.GetBody()
		if err != nil {
			return GoogleAccessToken{}, ErrUnavailable
		}
		resp, err = client.Do(req)
	}
	if err != nil {
		return GoogleAccessToken{}, ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GoogleAccessToken{}, ErrUnavailable
	}
	var result struct {
		AccessToken string    `json:"accessToken"`
		ExpireTime  time.Time `json:"expireTime"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result) != nil || result.AccessToken == "" ||
		time.Until(result.ExpireTime) < 5*time.Second || time.Until(result.ExpireTime) > time.Duration(lifetime)*time.Second+5*time.Second {
		return GoogleAccessToken{}, ErrInvalidResponse
	}
	return GoogleAccessToken{result.AccessToken, "Bearer", result.ExpireTime, b.cfg.ServiceAccountEmail, slices.Clone(scopes)}, nil
}
