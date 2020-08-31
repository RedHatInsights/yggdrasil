package yggdrasil

import (
	"fmt"
	"net/http"
)

// An APIResponseError represents an unexpected response from an HTTP method call.
type APIResponseError struct {
	code int
	body string
}

func (e APIResponseError) Error() string {
	return fmt.Sprintf("unexpected response: %v - %v (%v)", e.code, http.StatusText(e.code), e.body)
}
