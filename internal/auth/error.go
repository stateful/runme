package auth

type APITokenError struct {
	message string
}

func NewAPITokenError(msg string) *APITokenError {
	return &APITokenError{message: msg}
}

func (e *APITokenError) Error() string {
	return e.message
}
