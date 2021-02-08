package main

import "github.com/godbus/dbus/v5"

// getFacts calls the GetFacts method on the RHSM daemon.
func getFacts() (map[string]interface{}, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	var facts map[string]interface{}
	if err = conn.Object("com.redhat.RHSM1.Facts", "/com/redhat/RHSM1/Facts").Call("com.redhat.RHSM1.Facts.GetFacts", dbus.Flags(0)).Store(&facts); err != nil {
		return nil, err
	}

	return facts, nil
}
