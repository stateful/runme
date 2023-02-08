package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type apiServerHandlerMock struct {
	*http.ServeMux
}

func newAPIServerHandlerMock() *apiServerHandlerMock {
	mux := http.NewServeMux()
	s := &apiServerHandlerMock{ServeMux: mux}
	mux.HandleFunc(APIAuthEndpoint, authHandler)
	return s
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`invalid method`))
		return
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := decoder.Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`invalid payload`))
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte{})

	result := struct {
		Status string `json:"status"`
		Token  string `json:"token"`
	}{
		Status: "OK",
		Token:  tokenStr,
	}
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(&result)
}
