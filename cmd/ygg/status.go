package main

import (
	"fmt"
	"os"

	systemd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/redhatinsights/yggdrasil"
)

func getStatus() (string, error) {
	var status string

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	status += fmt.Sprintf("Connection status for %s:\n\n", hostname)

	uuid, err := getConsumerUUID()
	if err != nil {
		status += fmt.Sprintf("⛔️ error: Unable to check Red Hat Subscription Manager status: %s\n\n", err)
	} else {
		if uuid != "" {
			status += fmt.Sprintln("✅ Connected to Red Hat Subscription Manager (RHSM).")
		} else {
			status += fmt.Sprintln("❌ Not connected to Red Hat Subscription Manager (RHSM).")
		}
	}

	conn, err := systemd.NewSystemConnection()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	unitName := yggdrasil.ShortName + "d.service"

	properties, err := conn.GetUnitProperties(unitName)
	if err != nil {
		return "", err
	}
	activeState := properties["ActiveState"]
	if activeState.(string) == "active" {
		status += fmt.Sprintln("✅ Cloud Client service is active.")
	} else {
		status += fmt.Sprintln("❌ Cloud Client service is inactive.")
	}

	status += fmt.Sprintln()
	status += fmt.Sprintln("See all your connected systems: http://red.ht/connect")

	return status, nil
}
