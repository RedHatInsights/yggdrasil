package yggdrasil

import (
	"fmt"
	"net/http"
)

// ErrInvalidContentType indicates an unsupported "collector" value was given
// in the upload request.
var ErrInvalidContentType = &APIResponseError{
	Code: http.StatusUnsupportedMediaType,
	Body: "Content type of payload is unsupported",
}

// ErrPayloadTooLarge indicates an upload request body exceeded the size limit.
var ErrPayloadTooLarge = &APIResponseError{
	Code: http.StatusRequestEntityTooLarge,
	Body: "Payload too large",
}

// ErrUnauthorized indicates an upload request without an Authentication header.
var ErrUnauthorized = &APIResponseError{
	Code: http.StatusUnauthorized,
	Body: "Authentication missing from request",
}

// An APIResponseError represents an unexpected response from an HTTP method call.
type APIResponseError struct {
	Code int
	Body string
}

func (e APIResponseError) Error() string {
	v := fmt.Sprintf("unexpected response: %v - %v", e.Code, http.StatusText(e.Code))
	if e.Body != "" {
		v += fmt.Sprintf(" (%v)", e.Body)
	}
	return v
}

// An InvalidValueTypeError represents an error when serializing data into an
// unsupported destination.
type InvalidValueTypeError struct {
	key string
	val interface{}
}

func (e InvalidValueTypeError) Error() string {
	return fmt.Sprintf("invalid type '%T' for key '%s'", e.val, e.key)
}

// An InvalidArgumentError represents an invalid value passed to a command line
// argument.
type InvalidArgumentError struct {
	flag, value string
}

func (e InvalidArgumentError) Error() string {
	if e.value == "" {
		return "missing value for argument '--" + e.flag + "'"
	}
	return "invalid value '" + e.value + "' for argument '" + e.flag + "'"
}
