package work

import "reflect"

// typeConversionError represents a conversion error when converting one type
// to  another.
type typeConversionError struct {
	p reflect.Value
	t reflect.Type
}

func (e typeConversionError) Error() string {
	return "cannot convert " + e.p.Type().Name() + " to " + e.t.Name()
}

func newUint32TypeConversionError(parameter interface{}) typeConversionError {
	return typeConversionError{
		p: reflect.ValueOf(parameter),
		t: reflect.TypeOf(uint32(0)),
	}
}

func newStringTypeConversionError(parameter interface{}) typeConversionError {
	return typeConversionError{
		p: reflect.ValueOf(parameter),
		t: reflect.TypeOf(""),
	}
}

func newStringMapTypeConversionError(parameter interface{}) typeConversionError {
	return typeConversionError{
		p: reflect.ValueOf(parameter),
		t: reflect.TypeOf(map[string]string{}),
	}
}
