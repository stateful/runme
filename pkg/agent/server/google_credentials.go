package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/runmedev/runme/v3/pkg/agent/api"
	"github.com/runmedev/runme/v3/pkg/agent/config"
	"github.com/runmedev/runme/v3/pkg/agent/credentials"
	"github.com/runmedev/runme/v3/pkg/agent/iam"
)

const googleMintPath = "/v1/credentials/google:mint"

type googleTokenMinter interface {
	MintGoogleAccessToken(context.Context, credentials.MintRequest) (credentials.GoogleAccessToken, error)
}

// registerGoogleCredentials refuses permissive server defaults. Other protected
// services use the same verifier/checker; the new role never implies execution.
func (s *Server) registerGoogleCredentials(mux *AuthMux, oidc *iam.OIDC) error {
	cfg := s.serverConfig.GoogleTokenBroker
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if oidc == nil {
		return errors.New("google token broker requires authentication")
	}
	if _, ok := s.checker.(*iam.PolicyChecker); !ok {
		return errors.New("google token broker requires explicit IAM policy")
	}
	bind := net.ParseIP(s.serverConfig.GetBindAddress())
	if bind == nil || !bind.IsLoopback() {
		return errors.New("google token broker must bind to a loopback address")
	}
	if len(s.serverConfig.CorsOrigins) == 0 {
		return errors.New("google token broker requires explicit CORS origins")
	}
	for _, origin := range s.serverConfig.CorsOrigins {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || strings.Contains(origin, "*") {
			return errors.New("google token broker requires exact HTTPS CORS origins")
		}
	}
	broker, err := credentials.NewGoogleBroker(*cfg)
	if err != nil {
		return err
	}
	mux.Handle(googleMintPath, newGoogleTokenHandler(s.serverConfig, oidc, s.checker, broker))
	return nil
}

// newGoogleTokenHandler accepts only explicit bearer authentication. It cannot
// inherit the normal OIDC middleware's cookie/query fallback or log token data.
func newGoogleTokenHandler(cfg *config.AssistantServerConfig, oidc *iam.OIDC, checker iam.Checker, broker googleTokenMinter) http.Handler {
	auth := &iam.AuthContext{OIDC: oidc, Checker: checker, Role: api.TokenCreatorRole}
	origins := make(map[string]bool, len(cfg.CorsOrigins))
	for _, origin := range cfg.CorsOrigins {
		origins[origin] = true
	}
	limiter := rate.NewLimiter(rate.Every(200*time.Millisecond), 20)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Content-Type", "application/json")
		fail := func(status int, code string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "requestId": requestID})
		}
		host, port, err := net.SplitHostPort(r.Host)
		if err != nil || port != strconv.Itoa(cfg.GetPort()) || (host != "localhost" && host != "127.0.0.1" && host != "::1") {
			fail(http.StatusForbidden, "HOST_DENIED")
			return
		}
		origin := r.Header.Get("Origin")
		if !origins[origin] || origin == "null" || origin == "" {
			fail(http.StatusForbidden, "ORIGIN_DENIED")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			if r.Header.Get("Access-Control-Request-Method") != http.MethodPost {
				fail(http.StatusMethodNotAllowed, "METHOD_DENIED")
				return
			}
			for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				header = strings.ToLower(strings.TrimSpace(header))
				if header != "" && header != "authorization" && header != "content-type" {
					fail(http.StatusForbidden, "HEADER_DENIED")
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			fail(http.StatusMethodNotAllowed, "METHOD_DENIED")
			return
		}
		if r.URL.RawQuery != "" {
			fail(http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		bearer := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(bearer), "bearer ") {
			fail(http.StatusUnauthorized, "UNAUTHENTICATED")
			return
		}
		ctx, err := auth.AuthorizeBearerToken(r.Context(), bearer)
		if err != nil {
			if errors.Is(err, iam.ErrRoleDenied) {
				fail(http.StatusForbidden, "ROLE_DENIED")
			} else {
				fail(http.StatusUnauthorized, "UNAUTHENTICATED")
			}
			return
		}
		if !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			fail(http.StatusTooManyRequests, "RATE_LIMITED")
			return
		}
		if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
			fail(http.StatusUnsupportedMediaType, "JSON_REQUIRED")
			return
		}
		var input *credentials.MintRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || input == nil || decoder.Decode(new(any)) != io.EOF {
			fail(http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		token, err := broker.MintGoogleAccessToken(ctx, *input)
		switch {
		case errors.Is(err, credentials.ErrInvalidRequest):
			fail(http.StatusBadRequest, "INVALID_REQUEST")
		case errors.Is(err, credentials.ErrScopeDenied):
			fail(http.StatusForbidden, "SCOPE_DENIED")
		case errors.Is(err, credentials.ErrInvalidResponse):
			fail(http.StatusBadGateway, "INVALID_UPSTREAM_RESPONSE")
		case err != nil:
			fail(http.StatusServiceUnavailable, "WORKLOAD_CREDENTIALS_UNAVAILABLE")
		default:
			_ = json.NewEncoder(w).Encode(token)
		}
	})
}
