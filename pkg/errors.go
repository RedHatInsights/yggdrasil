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
	v := fmt.Sprintf("unexpected response: %v - %v", e.code, http.StatusText(e.code))
	if e.body != "" {
		v += fmt.Sprintf(" (%v)", e.body)
	}
	return v
}
