package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/stateful/runme/v3/internal/log"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	oauthTokenKey = "github"
)

//go:generate mockgen --build_flags=--mod=mod -destination=./auth_mock_gen.go -package=auth . Authorizer
type Authorizer interface {
	TokenProvider
	Login(context.Context) error
	Logout() error
}

// Auth provides an authentication and authorization method for the CLI.
// It relies on a browser and known web application to do it securely.
type Auth struct {
	// OAuthConfig describes OAuth flow.
	OAuthConfig oauth2.Config

	// APIBaseURL is Stateful API address.
	// It is used to obtain Stateful's JWT token.
	APIBaseURL string

	// Storage is responsible for storing and retrieving token.
	Storage Storage

	// client is an optional *http.Client to use to make requests.
	// It might be used, for example, to enable logging or metrics.
	client *http.Client

	// env helps to interact with the environment in which
	// the authentication is performed.
	//
	// For example, one can do the dance fully automatically
	// allowing the code to open a browser and handle the redirect
	// while other can rely on manual copy&paste.
	env Env

	// loginInProgress is a mutex to prevent concurrent `Login` calls.
	loginInProgress uint32

	// loginSession contains details about the current login session.
	loginSession *oauthSession

	log *zap.Logger
}

type oauthSession struct {
	oauth2.Config

	id string

	// CodeC is a channel to which the code obtained from OAuth flow in the browser
	// is sent to. Later it's used to obtain a token.
	CodeC chan authorizationCode

	// State is used in the OAuth flow to protect against CSRF.
	State string
}

func (s *oauthSession) AuthCodeURL() string {
	return s.Config.AuthCodeURL(s.State, oauth2.AccessTypeOffline)
}

func (s *oauthSession) WaitForCodeAndState(ctx context.Context) (string, string, error) {
	select {
	case result := <-s.CodeC:
		return result.Value, s.State, result.Err
	case <-ctx.Done():
		return "", s.State, ctx.Err()
	}
}

func newOAuthSession(cfg oauth2.Config) *oauthSession {
	state := genState()
	id := uuid.NewSHA1(uuid.New(), []byte(state))
	return &oauthSession{
		Config: cfg,
		CodeC:  make(chan authorizationCode, 1),
		State:  state,
		id:     id.String(),
	}
}

type authorizationCode struct {
	Value string
	Err   error
}

type Opts func(*Auth)

func WithClient(c *http.Client) func(*Auth) {
	return func(a *Auth) {
		a.client = c
	}
}

func WithEnv(env Env) func(*Auth) {
	return func(a *Auth) {
		a.env = env
	}
}

// New creates a new instance of Auth.
func New(oauthConfig oauth2.Config, apiURL string, storage Storage, opts ...Opts) *Auth {
	a := &Auth{
		OAuthConfig: oauthConfig,
		APIBaseURL:  apiURL,
		Storage:     storage,
		log:         log.Get().Named("Auth"),
	}

	for _, f := range opts {
		f(a)
	}

	return a
}

// Login opens browser and authenticates user.
// It fallbacks to no browser flow if opening a URL fails,
func (a *Auth) Login(ctx context.Context) error {
	if !atomic.CompareAndSwapUint32(&a.loginInProgress, 0, 1) {
		return errors.New("login session already in progress")
	}
	defer atomic.StoreUint32(&a.loginInProgress, 0)

	a.log.Debug("start logging in")

	a.loginSession = newOAuthSession(a.OAuthConfig)

	if a.env == nil {
		a.env = &desktopEnv{Session: a.loginSession}
	}

	if a.env.IsAutonomous() {
		return a.loginAuto(ctx)
	}
	return a.loginManual(ctx)
}

func (a *Auth) loginAuto(ctx context.Context) error {
	l := a.log.With(zap.String("session", a.loginSession.id))

	l.Debug("trying to automatically log in")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	// Start a local server which will be used by the backend application
	// to redirect to in order to complete the flow.
	serverAddr, serverErrC := a.serveHTTP(ln)

	// Update the redirect URL. It's needed because the port is dynamically assigned.
	a.loginSession.RedirectURL = serverAddr

	url := a.loginSession.AuthCodeURL()

	l.Debug("obtained an auth URL", zap.String("url", url))

	if err := a.env.RequestCode(url, a.loginSession.State); err != nil {
		l.Info("failed to request a code", zap.String("url", url), zap.Error(err))
		return err
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		err = <-serverErrC
		cancel()
	}()

	code, _, waitErr := a.env.WaitForCodeAndState(ctx)
	if errors.Is(waitErr, context.Canceled) && err != nil {
		waitErr = err // original error
	}
	if waitErr != nil {
		l.Info("failed to wait for code and state", zap.Error(waitErr))
		return waitErr
	}

	err = a.finishLoginWithCode(ctx, code)
	if err != nil {
		l.Info("failed to finish login", zap.Error(err))
	} else {
		l.Debug("finished logging successfully")
	}
	return err
}

func (a *Auth) loginManual(ctx context.Context) error {
	l := a.log.With(zap.String("session", a.loginSession.id))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	// Start a local server which will be used by the backend application
	// to redirect to in order to complete the flow.
	//
	// If we fail to start the server, it's still possible to finish the flow
	// by examining the redirect URL. This local server merely acts as a nicer
	// presentation of the query params of the redirect URL.
	serverAddr, _ := a.serveHTTP(ln)

	// Update the redirect URL. It's needed because the port is dynamically assigned.
	a.loginSession.RedirectURL = serverAddr + "/manual"

	url := a.loginSession.AuthCodeURL()

	l.Debug("obtained an auth URL", zap.String("url", url))

	if err := a.env.RequestCode(url, a.loginSession.State); err != nil {
		l.Info("failed to request a code", zap.String("url", url), zap.Error(err))
		return err
	}

	code, _, err := a.env.WaitForCodeAndState(ctx)
	if err != nil {
		l.Info("failed to wait for code and state", zap.Error(err))
		return err
	}

	err = a.finishLoginWithCode(ctx, code)
	if err != nil {
		l.Info("failed to finish login", zap.Error(err))
	} else {
		l.Debug("finished logging successfully")
	}
	return err
}

func (a *Auth) finishLoginWithCode(ctx context.Context, code string) error {
	ctx1, cancel1 := context.WithTimeout(ctx, 15*time.Second)
	defer cancel1()

	token, err := a.getAndSaveOAuthToken(ctx1, code)
	if err != nil {
		return err
	}

	ctx2, cancel2 := context.WithTimeout(ctx, 15*time.Second)
	defer cancel2()

	_, err = a.getAndSaveAPIToken(ctx2, string(token.Data))
	return err
}

func parseJWTUnverified(token string, claims jwt.Claims) (*jwt.Token, error) {
	tok, _, err := new(jwt.Parser).ParseUnverified(token, claims)
	return tok, err
}

func isAPITokenValid(token string) error {
	var claims jwt.MapClaims
	_, err := parseJWTUnverified(token, &claims)
	if err != nil {
		return err
	}

	notExpired := claims.VerifyExpiresAt(time.Now().Add(time.Second*15).Unix(), true)
	if !notExpired {
		return errors.New("token is expired")
	}

	return nil
}

func (a *Auth) loadAndValidateAPIToken(key string) (Token, error) {
	var apiToken Token

	if err := a.Storage.Load(key, &apiToken); err != nil {
		return emptyToken, errors.Wrap(err, "load API Token failed")
	}

	if err := isAPITokenValid(apiToken.Data); err != nil {
		return emptyToken, errors.Wrap(err, "invalid API Token")
	}

	return apiToken, nil
}

func getTokenKey(key string, domain string) (string, error) {
	u, err := url.Parse(domain)
	if err != nil {
		return "", err
	}

	host := strings.ReplaceAll(u.Host, ".", "_")

	return key + "_" + host, nil
}

var cleanOldAPITokenOnce = sync.Once{}

func (a *Auth) getAPITokenKey() (string, error) {
	const apiTokenKey = "api"

	cleanOldAPITokenOnce.Do(func() {
		_ = a.Storage.Delete(apiTokenKey)
	})

	return getTokenKey(apiTokenKey, a.APIBaseURL)
}

func (a *Auth) GetTokenPath() (string, error) {
	key, err := a.getAPITokenKey()
	if err != nil {
		return "", err
	}
	return a.Storage.Path(key), nil
}

// GetToken returns an API token to access the Stateful API.
// If the token exists and is valid for at least 15 seconds,
// it is returned immediately. Otherwise, it's fetched
// using a never-expiring GitHub token.
func (a *Auth) GetToken(ctx context.Context) (string, error) {
	key, err := a.getAPITokenKey()
	if err != nil {
		return "", fmt.Errorf("failed to get token key: %w", err)
	}

	apiToken, err := a.loadAndValidateAPIToken(key)
	if err == nil {
		a.log.Debug("found a cached API token")
		return apiToken.Data, nil
	}
	a.log.Debug("failed to load and validate API token; trying to refresh it", zap.Error(err))

	var ghToken Token
	err = a.Storage.Load(oauthTokenKey, &ghToken)
	if err != nil {
		a.log.Info("failed to load GitHub token", zap.Error(err))
		return "", fmt.Errorf("failed to load GitHub token: %w", err)
	}

	a.log.Debug("getting a new API token")

	apiToken, err = a.getAndSaveAPIToken(ctx, ghToken.Data)
	if err != nil {
		a.log.Warn("failed to get and save the API token", zap.Error(err))
	}
	return apiToken.Data, err
}

// GetFreshToken is similar to GetToken but it always returns
// a new API token.
func (a *Auth) GetFreshToken(ctx context.Context) (string, error) {
	var ghToken Token
	err := a.Storage.Load(oauthTokenKey, &ghToken)
	if err != nil {
		a.log.Info("failed to load GitHub token", zap.Error(err))
		return "", err
	}

	apiToken, err := a.getAPIToken(ctx, ghToken.Data)
	if err != nil {
		a.log.Info("failed to save API token", zap.Error(err))
	}
	return apiToken, err
}

func (a *Auth) serveHTTP(ln net.Listener) (string, <-chan error) {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(a.callbackHandler))
	mux.Handle("/manual", http.HandlerFunc(a.manualCallbackHandler))

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)

	go func() {
		if err := server.Serve(ln); err != nil {
			errc <- err
		}
	}()

	return fmt.Sprintf("http://%s", ln.Addr().String()), errc
}

var cliAuthStatusPage = func() url.URL {
	u, err := url.Parse("https://www.stateful.com/cli/runme")
	if err != nil {
		panic(err)
	}
	return *u
}()

func getCLILoginURL(params url.Values) string {
	u := cliAuthStatusPage
	p := u.Query()

	for key, val := range params {
		for _, item := range val {
			p.Add(key, item)
		}
	}

	u.RawQuery = p.Encode()

	return u.String()
}

func (a *Auth) callbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if state != a.loginSession.State {
		params := url.Values{}
		params.Set("success", "false")
		params.Set("reason", "Invalid state value")

		http.Redirect(w, r, getCLILoginURL(params), http.StatusTemporaryRedirect)
		return
	}

	if code == "" {
		msg := `missing "code" query string parameter`

		go func() {
			a.loginSession.CodeC <- authorizationCode{Err: errors.New(msg)}
		}()

		params := url.Values{}
		params.Set("success", "false")
		params.Set("reason", strings.ToTitle(msg))

		http.Redirect(w, r, getCLILoginURL(params), http.StatusTemporaryRedirect)
		return
	}

	go func() {
		a.loginSession.CodeC <- authorizationCode{Value: code}
	}()

	http.Redirect(w, r, getCLILoginURL(url.Values{"success": []string{"true"}}), http.StatusTemporaryRedirect)
}

func (a *Auth) manualCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if state != a.loginSession.State {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`Invalid state received from the Identity Provider.`))
	}

	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`Missing "code" query string parameter.`))
	}

	_, _ = w.Write([]byte(fmt.Sprintf("<p>State: <strong>%s</strong></p>\n<p>Code: <strong>%s</strong></p>", a.loginSession.State, code)))
}

func (a *Auth) getHTTPClient() *http.Client {
	if a.client != nil {
		return a.client
	}
	return http.DefaultClient
}

func (a *Auth) getOAuthToken(ctx context.Context, code string) (string, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.getHTTPClient())

	tok, err := a.OAuthConfig.Exchange(ctx, code)
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func (a *Auth) getAndSaveOAuthToken(ctx context.Context, code string) (Token, error) {
	accessToken, err := a.getOAuthToken(ctx, code)
	if err != nil {
		return emptyToken, err
	}
	token := Token{
		Type: "access_token",
		Data: accessToken,
	}
	return token, a.Storage.Save(oauthTokenKey, token)
}

const APIAuthEndpoint = "/auth/cli/runme"

// TODO(adamb): this is a hack, it temporarily uses an endpoint
// that is dedicated for the VS Code to exchange the GH access token
// for the application JWT.
func (a *Auth) getAPIToken(ctx context.Context, accessToken string) (string, error) {
	u, err := url.Parse(a.APIBaseURL)
	if err != nil {
		return "", err
	}
	u.Path = APIAuthEndpoint

	payload := struct {
		AccessToken string `json:"access_token"`
	}{
		AccessToken: string(accessToken),
	}
	marshaledPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBuffer(marshaledPayload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.getHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResponse struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Alerts  []struct {
				Command string `json:"command"`
				Level   string `json:"level"`
				Message string `json:"message"`
			} `json:"alerts"`
		}
		err := json.NewDecoder(resp.Body).Decode(&errResponse)
		if err == nil && errResponse.Message != "" {
			a.log.Info("received error response when getting API token", zap.Any("response", errResponse))
			return "", &APITokenError{message: errResponse.Message}
		}
		a.log.Info("failed to use error response", zap.Error(err))
		return "", fmt.Errorf("request failed: %s", resp.Status)
	}

	var result struct {
		Token string `json:"token"`
	}

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return "", err
	}

	if result.Token == "" {
		return "", fmt.Errorf("invalid response: %v", result)
	}

	return result.Token, nil
}

func (a *Auth) getAndSaveAPIToken(ctx context.Context, accessToken string) (Token, error) {
	token, err := a.getAPIToken(ctx, accessToken)
	if err != nil {
		return emptyToken, err
	}

	key, err := a.getAPITokenKey()
	if err != nil {
		return emptyToken, err
	}

	value := Token{
		Type: "jwt",
		Data: token,
	}

	return value, a.Storage.Save(key, value)
}

func (a *Auth) Logout() error {
	key, err := a.getAPITokenKey()
	if err != nil {
		return err
	}

	err = multierr.Combine(
		a.Storage.Delete(oauthTokenKey),
		a.Storage.Delete(key),
	)

	var result error

	for _, err := range multierr.Errors(err) {
		a.log.Info("failed to delete token", zap.Error(err))
		if !errors.Is(err, os.ErrNotExist) {
			result = multierr.Append(result, err)
		}
	}

	return result
}

func genState() string {
	stateBytes := [8]byte{}
	_, _ = rand.Read(stateBytes[:])
	return hex.EncodeToString(stateBytes[:])
}
