package yggdrasil

import (
	"fmt"
	"net/http"
)

// ErrInvalidContentType indicates an unsupported "collector" value was given
// in the upload request.
var ErrInvalidContentType = &APIResponseError{
	Code: http.StatusUnsupportedMediaType,
	body: "Content type of payload is unsupported",
}

// ErrPayloadTooLarge indicates an upload request body exceeded the size limit.
var ErrPayloadTooLarge = &APIResponseError{
	Code: http.StatusRequestEntityTooLarge,
	body: "Payload too large",
}

// An APIResponseError represents an unexpected response from an HTTP method call.
type APIResponseError struct {
	Code int
	body string
}

func (e APIResponseError) Error() string {
	v := fmt.Sprintf("unexpected response: %v - %v", e.Code, http.StatusText(e.Code))
	if e.body != "" {
		v += fmt.Sprintf(" (%v)", e.body)
	}
	return v
}
