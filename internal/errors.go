package yggdrasil

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
