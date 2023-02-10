package auth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func Test_isAPITokenValid(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name   string
		Exp    int64
		Result bool
	}{
		{
			Name:   "Valid token",
			Exp:    time.Now().Add(time.Hour).Unix(),
			Result: true,
		},
		{
			Name:   "Invalid token",
			Exp:    time.Now().Unix(),
			Result: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"exp": tc.Exp,
			})
			tokenStr, err := token.SignedString([]byte{})
			require.NoError(t, err)

			err = isAPITokenValid(tokenStr)
			if tc.Result {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestAuthGetToken(t *testing.T) {
	authSrv := httptest.NewServer(newOAuthServerHandlerMock())
	apiSrv := httptest.NewServer(newAPIServerHandlerMock())

	storage := &DiskStorage{Location: t.TempDir()}
	auth := New(oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  authSrv.URL + "/login/oauth/authorize",
			TokenURL: authSrv.URL + "/login/oauth/access_token",
		},
	}, apiSrv.URL, storage, WithEnv(&TestEnv{}))

	apiTokKey, err := auth.getAPITokenKey()
	require.NoError(t, err)

	// There is no GH token so it will fail.
	_, err = auth.GetToken(context.Background())
	require.Error(t, err)

	// Create and store a GH token.
	ghTok := Token{
		Type: "access_token",
		Data: `{}`, // empty JSON object
	}
	err = storage.Save(oauthTokenKey, ghTok)
	require.NoError(t, err)

	// With the GH token present, it should succeed.
	_, err = auth.GetToken(context.Background())
	require.NoError(t, err)
	// And it should produce a valid API token.
	_, err = auth.loadAndValidateAPIToken(apiTokKey)
	require.NoError(t, err)

	// When the API token expires...
	expToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	expTokenStr, _ := expToken.SignedString([]byte{})
	err = storage.Save(apiTokKey, Token{
		Type: "jwt",
		Data: expTokenStr,
	})
	require.NoError(t, err)
	// ...it is not valid anymore.
	_, err = auth.loadAndValidateAPIToken(apiTokKey)
	require.EqualError(t, err, "invalid API Token: token is expired")

	// But the new API token can be retrieved...
	_, err = auth.GetToken(context.Background())
	require.NoError(t, err)
	// ...and should be valid again.
	_, err = auth.loadAndValidateAPIToken(apiTokKey)
	require.NoError(t, err)
}

func TestAuthLogin_withTestEnv(t *testing.T) {
	authSrv := httptest.NewServer(newOAuthServerHandlerMock())
	apiSrv := httptest.NewServer(newAPIServerHandlerMock())

	storage := &DiskStorage{Location: t.TempDir()}
	auth := New(oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  authSrv.URL + "/login/oauth/authorize",
			TokenURL: authSrv.URL + "/login/oauth/access_token",
		},
	}, apiSrv.URL, storage, WithEnv(&TestEnv{}))

	err := auth.Login(context.Background())
	require.NoError(t, err)
}

func TestAuthLogin_withTerminalEnv(t *testing.T) {
	authSrv := httptest.NewServer(newOAuthServerHandlerMock())
	apiSrv := httptest.NewServer(newAPIServerHandlerMock())

	storage := &DiskStorage{Location: t.TempDir()}

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	env := &TerminalEnv{inR, outW}

	auth := New(oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  authSrv.URL + "/login/oauth/authorize",
			TokenURL: authSrv.URL + "/login/oauth/access_token",
		},
	}, apiSrv.URL, storage, WithEnv(env))

	// Discard any output.
	go func() {
		buf := bufio.NewReader(outR)
		for {
			_, _, _ = buf.ReadLine()
		}
	}()

	// Write any code which is expected by the TerminalEnv.
	go func() {
		_, _ = fmt.Fprintf(inW, "test-code\n")
	}()

	err := auth.Login(context.Background())
	require.NoError(t, err)
}

func TestAuthLogin_outdatedClient(t *testing.T) {
	authSrv := httptest.NewServer(newOAuthServerHandlerMock())
	apiMux := http.NewServeMux()
	apiMux.HandleFunc(APIAuthEndpoint, func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte(`{
	"code": 400,
	"message": "CLI is too old. Please upgrade to the latest version.",
	"alerts": [{
		"level": "error",
		"command": "open",
		"message": "Please upgrade the CLI to the latest version."
	}]
}`))
	})
	apiSrv := httptest.NewServer(apiMux)
	defer apiSrv.Close()
	storage := &DiskStorage{Location: t.TempDir()}
	auth := New(oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  authSrv.URL + "/login/oauth/authorize",
			TokenURL: authSrv.URL + "/login/oauth/access_token",
		},
	}, apiSrv.URL, storage, WithEnv(&TestEnv{}))

	err := auth.Login(context.Background())
	require.IsType(t, new(APITokenError), err)
	require.EqualError(t, err, "CLI is too old. Please upgrade to the latest version.")
}
