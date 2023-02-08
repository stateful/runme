package auth

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type oauthServerHandlerMock struct {
	*http.ServeMux

	state     string
	code      string
	completed bool
}

func (s *oauthServerHandlerMock) Completed() bool { return s.completed }

func (s *oauthServerHandlerMock) Reset() {
	s.state = ""
	s.code = ""
	s.completed = false
}

func newOAuthServerHandlerMock() *oauthServerHandlerMock {
	mux := http.NewServeMux()
	s := &oauthServerHandlerMock{ServeMux: mux}

	mux.HandleFunc("/login/oauth/authorize", s.authorizeHandler)

	mux.HandleFunc("/login/oauth/access_token", s.accessTokenHandler)

	return s
}

func (s *oauthServerHandlerMock) authorizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`invalid method`))
		return
	}

	state := r.URL.Query().Get("state")
	redirectURI := r.URL.Query().Get("redirect_uri")

	u, err := url.Parse(redirectURI)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		_, _ = w.Write([]byte(`invalid redirect_uri ` + err.Error()))
		return
	}

	s.state = state
	s.code = "test-code-for-" + state

	params := u.Query()
	params.Add("code", s.code)
	params.Add("state", s.state)

	u.RawQuery = params.Encode()

	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}

func (s *oauthServerHandlerMock) accessTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`invalid method`))
		return
	}

	// TODO: validate received code
	s.completed = true

	type response struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		TokenType   string `json:"token_type"`
	}

	payload := response{
		AccessToken: "access-token-123",
		Scope:       "user",
		TokenType:   "bearer",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	w.Header().Add("Content-Type", "application/json")
	_, _ = w.Write(data)
}
