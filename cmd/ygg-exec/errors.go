package main

// An invalidArgumentError represents an invalid value passed to a command line
// argument.
type invalidArugmentError struct {
	flag, value string
}

func (e invalidArugmentError) Error() string {
	if e.value == "" {
		return "missing value for argument '--" + e.flag + "'"
	}
	return "invalid value '" + e.value + "' for argument '" + e.flag + "'"
}
