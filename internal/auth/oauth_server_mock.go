package auth

import (
	"encoding/json"
	"log"
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

	mux.HandleFunc("/login/oauth/authorize", wrapMuxHandlerErr(s.authorizeHandler))

	mux.HandleFunc("/login/oauth/access_token", wrapMuxHandlerErr(s.accessTokenHandler))

	return s
}

func (s *oauthServerHandlerMock) authorizeHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`invalid method`))
		return err
	}

	state := r.URL.Query().Get("state")
	redirectURI := r.URL.Query().Get("redirect_uri")

	u, err := url.Parse(redirectURI)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`invalid redirect_uri ` + err.Error()))
		return err
	}

	s.state = state
	s.code = "test-code-for-" + state

	params := u.Query()
	params.Add("code", s.code)
	params.Add("state", s.state)

	u.RawQuery = params.Encode()

	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)

	return nil
}

func (s *oauthServerHandlerMock) accessTokenHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`invalid method`))
		return err
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
		_, err := w.Write([]byte(err.Error()))
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	_, err = w.Write(data)

	return err
}

func wrapMuxHandlerErr(handler func(http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			log.Fatal(err)
		}
	}
}
