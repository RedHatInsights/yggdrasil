package main

import (
	systemd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/redhatinsights/yggdrasil"
)

func activate() error {
	conn, err := systemd.NewSystemConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	unitName := yggdrasil.ShortName + "d.service"

	if _, _, err := conn.EnableUnitFiles([]string{unitName}, false, true); err != nil {
		return err
	}

	done := make(chan string)
	if _, err := conn.StartUnit(unitName, "replace", done); err != nil {
		return err
	}
	<-done

	return nil
}

func deactivate() error {
	conn, err := systemd.NewSystemConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	unitName := yggdrasil.ShortName + "d.service"

	done := make(chan string)
	if _, err := conn.StopUnit(unitName, "replace", done); err != nil {
		return err
	}
	<-done

	if _, err := conn.DisableUnitFiles([]string{unitName}, false); err != nil {
		return err
	}

	return nil
}
