package work

import "github.com/godbus/dbus/v5"

func NewDBusError(name string, body ...string) *dbus.Error {
	e := dbus.Error{}
	e.Name = name
	for _, v := range body {
		e.Body = append(e.Body, v)
	}
	return &e
}
