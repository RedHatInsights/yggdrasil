package work

import (
	"fmt"
	"regexp"

	"github.com/godbus/dbus/v5"
)

func NewDBusError(name string, body ...string) *dbus.Error {
	e := dbus.Error{}
	e.Name = name
	for _, v := range body {
		e.Body = append(e.Body, v)
	}
	return &e
}

// ScrubName cleans up invalid bus names to ensure D-Bus name specification
// conformance. An error is returned along with the scrubbed value if the name
// contained invalid characters.
func ScrubName(name string) (string, error) {
	r := regexp.MustCompile("-")
	if r.Match([]byte(name)) {
		newName := string(r.ReplaceAll([]byte(name), []byte("_")))
		return newName, fmt.Errorf("directive name '%v' transformed to '%v'", name, newName)
	}
	return name, nil
}
