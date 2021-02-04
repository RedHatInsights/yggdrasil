package main

import (
	"fmt"
	"net/url"

	"github.com/godbus/dbus/v5"
)

func getConsumerUUID() (string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	var uuid string
	if err := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Consumer").Call("com.redhat.RHSM1.Consumer.GetUuid", dbus.Flags(0), "").Store(&uuid); err != nil {
		return "", err
	}
	return uuid, nil
}

func register(username, password, serverURL string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	uuid, err := getConsumerUUID()
	if err != nil {
		return err
	}
	if uuid != "" {
		return fmt.Errorf("warning: the system is already registered")
	}

	registerServer := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/RegisterServer")

	var privateDbusSocketURI string
	if err := registerServer.Call("com.redhat.RHSM1.RegisterServer.Start", dbus.Flags(0), "").Store(&privateDbusSocketURI); err != nil {
		return err
	}
	defer registerServer.Call("com.redhat.RHSM1.RegisterServer.Stop", dbus.FlagNoReplyExpected, "")

	privConn, err := dbus.Dial(privateDbusSocketURI)
	if err != nil {
		return err
	}
	defer privConn.Close()

	if err := privConn.Auth(nil); err != nil {
		return err
	}

	connectionOptions := make(map[string]string)
	if serverURL != "" {
		URL, err := url.Parse(serverURL)
		if err != nil {
			return err
		}
		connectionOptions["host"] = URL.Hostname()
		connectionOptions["port"] = URL.Port()
		connectionOptions["handler"] = URL.EscapedPath()
	}

	if err := privConn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Register").Call("com.redhat.RHSM1.Register.Register", dbus.Flags(0), "", username, password, map[string]string{}, connectionOptions, "").Err; err != nil {
		return err
	}

	return nil
}

func unregister() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	uuid, err := getConsumerUUID()
	if err != nil {
		return err
	}
	if uuid == "" {
		return fmt.Errorf("warning: the system is already unregistered")
	}

	if err := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Unregister").Call("com.redhat.RHSM1.Unregister.Unregister", dbus.Flags(0), map[string]string{}, "").Err; err != nil {
		return err
	}

	return nil
}
