package auth

import "context"

var emptyToken = Token{}

type Token struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type TokenProvider interface {
	GetToken(context.Context) (string, error)
	GetTokenPath() (string, error)
	GetFreshToken(context.Context) (string, error)
}
