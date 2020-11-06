package main

import (
	"fmt"

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
	properties, err := conn.GetUnitProperties(unitName)
	if err != nil {
		return err
	}
	activeState := properties["ActiveState"]
	if activeState.(string) != "active" {
		return fmt.Errorf("error: The unit %v failed to start. Run 'systemctl status %v' for more information", unitName, unitName)
	}

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
