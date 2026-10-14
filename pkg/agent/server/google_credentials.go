package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
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

// registerGoogleCredentials requires explicit authentication and IAM policy.
// CORS and authentication use the same protected-route middleware as other APIs;
// role/tokenCreator never implies execution permission.
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
	broker, err := credentials.NewGoogleBroker(*cfg)
	if err != nil {
		return err
	}
	mux.HandleProtected(googleMintPath, newGoogleTokenHandler(s.serverConfig, broker), s.checker, api.TokenCreatorRole)
	return nil
}

// newGoogleTokenHandler handles mint policy and response redaction after the
// shared protected-route middleware authenticates and checks role/tokenCreator.
func newGoogleTokenHandler(cfg *config.AssistantServerConfig, broker googleTokenMinter) http.Handler {
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
		if r.Method != http.MethodPost {
			fail(http.StatusMethodNotAllowed, "METHOD_DENIED")
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
		token, err := broker.MintGoogleAccessToken(r.Context(), *input)
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
