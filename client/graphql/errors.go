package graphql

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/stateful/runme/client/graphql/query"
)

var ErrNoData = errors.New("no data")

type APIError struct {
	apiError   error
	userErrors []query.UserError
}

func NewAPIError(apiErr error, uErrs ...interface{}) error {
	e := APIError{}
	e.SetAPIErrors(apiErr)
	for _, err := range uErrs {
		e.SetUserErrors(convertUserErrors(err))
	}
	if e.apiError != nil || e.userErrors != nil {
		return &e
	}
	return nil
}

func (e *APIError) SetAPIErrors(err error) {
	if err == nil {
		return
	}
	e.apiError = err
}

func (e *APIError) SetUserErrors(val []query.UserError) {
	if len(val) == 0 {
		return
	}
	e.userErrors = val
}

func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.apiError
}

func (e *APIError) Error() string {
	if e.apiError != nil {
		return e.apiError.Error()
	}

	if len(e.userErrors) > 0 {
		var b bytes.Buffer
		b.WriteString(userErrorString(e.userErrors[0]))
		for i := 1; i < len(e.userErrors); i++ {
			b.WriteByte('\n')
			b.WriteString(userErrorString(e.userErrors[i]))
		}
		return b.String()
	}

	return ""
}

func userErrorString(err query.UserError) string {
	return fmt.Sprintf("error %q affected %s fields", err.GetMessage(), strings.Join(err.GetField(), ", "))
}

func convertUserErrors(uErrs interface{}) (result []query.UserError) {
	if uErrs == nil {
		return nil
	}

	userErrorType := reflect.TypeOf((*query.UserError)(nil)).Elem()

	if typ := reflect.TypeOf(uErrs); typ.Kind() == reflect.Slice {
		s := reflect.ValueOf(uErrs)
		for i := 0; i < s.Len(); i++ {
			v := s.Index(i)
			if v.Type().Kind() != reflect.Ptr {
				v = v.Addr()
			}
			if v.Type().Implements(userErrorType) {
				result = append(result, v.Interface().(query.UserError))
			}
		}
	} else {
		panic("uErrs is not a slice")
	}
	return result
}
