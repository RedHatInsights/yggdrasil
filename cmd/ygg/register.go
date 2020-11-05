package main

import (
	"github.com/godbus/dbus/v5"
)

func register(username, password string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	object := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/RegisterServer")

	var privateDbusSocketURI string
	if err := object.Call("com.redhat.RHSM1.RegisterServer.Start", dbus.Flags(0), "").Store(&privateDbusSocketURI); err != nil {
		return err
	}
	defer object.Call("com.redhat.RHSM1.RegisterServer.Stop", dbus.FlagNoReplyExpected, "")

	privConn, err := dbus.Dial(privateDbusSocketURI)
	if err != nil {
		return err
	}
	defer privConn.Close()

	if err := privConn.Auth(nil); err != nil {
		return err
	}

	registerObject := privConn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Register")

	if err := registerObject.Call("com.redhat.RHSM1.Register.Register", dbus.Flags(0), "", username, password, map[string]string{}, map[string]string{}, "").Err; err != nil {
		return err
	}

	privConn.Close()

	return nil
}

func unregister() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	object := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Unregister")
	if err := object.Call("com.redhat.RHSM1.Unregister.Unregister", dbus.Flags(0), map[string]string{}, "").Err; err != nil {
		return err
	}

	return nil
}
