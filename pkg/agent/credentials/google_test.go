package credentials

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/runmedev/runme/v3/pkg/agent/config"
)

func TestGoogleBrokerPolicyAndCoalescing(t *testing.T) {
	scope := "https://www.googleapis.com/auth/drive"
	read := "https://www.googleapis.com/auth/drive.readonly"
	b, err := NewGoogleBroker(config.GoogleTokenBrokerConfig{Source: "workload_identity", TokenMode: "iam_credentials", ServiceAccountEmail: "runme@project.iam.gserviceaccount.com", AllowedScopes: []string{scope, read}})
	require.NoError(t, err)
	var calls atomic.Int32
	gate := make(chan struct{})
	b.mint = func(ctx context.Context, scopes []string, lifetime int) (GoogleAccessToken, error) {
		calls.Add(1)
		<-gate
		return GoogleAccessToken{AccessToken: "test", ExpiresAt: time.Now().Add(time.Duration(lifetime) * time.Second), Scopes: scopes}, nil
	}
	_, err = b.MintGoogleAccessToken(context.Background(), MintRequest{})
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = b.MintGoogleAccessToken(context.Background(), MintRequest{Scopes: []string{scope, "https://www.googleapis.com/auth/cloud-platform"}})
	require.ErrorIs(t, err, ErrScopeDenied)
	bad := 3601
	_, err = b.MintGoogleAccessToken(context.Background(), MintRequest{Scopes: []string{scope}, LifetimeSeconds: &bad})
	require.ErrorIs(t, err, ErrInvalidRequest)
	require.Equal(t, int32(0), calls.Load())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := b.MintGoogleAccessToken(context.Background(), MintRequest{Scopes: []string{scope, scope}})
			if err != nil || token.AccessToken != "test" {
				t.Errorf("mint failed: %v", err)
			}
		}()
	}
	require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)
	close(gate)
	wg.Wait()
	require.Equal(t, int32(1), calls.Load())
	_, err = b.MintGoogleAccessToken(context.Background(), MintRequest{Scopes: []string{read}})
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load())
	short := 60
	_, err = b.MintGoogleAccessToken(context.Background(), MintRequest{Scopes: []string{scope}, LifetimeSeconds: &short})
	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load())
}

type tokenTransport func(*http.Request) (*http.Response, error)

func (f tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGoogleIAMRequest(t *testing.T) {
	scope := "https://www.googleapis.com/auth/drive"
	b, err := NewGoogleBroker(config.GoogleTokenBrokerConfig{Source: "workload_identity", TokenMode: "iam_credentials", ServiceAccountEmail: "runme@project.iam.gserviceaccount.com", AllowedScopes: []string{scope}})
	require.NoError(t, err)
	b.source = func(context.Context) (oauth2.TokenSource, error) {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "workload-A"}), nil
	}
	client := &http.Client{Transport: tokenTransport(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "iamcredentials.googleapis.com", r.URL.Host)
		require.Equal(t, "/v1/projects/-/serviceAccounts/runme@project.iam.gserviceaccount.com:generateAccessToken", r.URL.Path)
		require.Equal(t, "Bearer workload-A", r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "900s", body["lifetime"])
		require.Equal(t, []any{scope}, body["scope"])
		payload, _ := json.Marshal(map[string]any{"accessToken": "Google-B", "expireTime": time.Now().Add(899 * time.Second)})
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
	})}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)
	token, err := b.MintGoogleAccessToken(ctx, MintRequest{Scopes: []string{scope}})
	require.NoError(t, err)
	require.Equal(t, "Google-B", token.AccessToken)
	require.Equal(t, []string{scope}, token.Scopes)
}

func TestRejectUserADC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adc.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"type":"authorized_user","client_id":"test","client_secret":"test","refresh_token":"must-not-be-used"}`), 0o600))
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	_, err := workloadTokenSource(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)
}
