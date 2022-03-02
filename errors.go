package yggdrasil

import (
	"fmt"
)

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
