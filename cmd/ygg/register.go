package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/godbus/dbus/v5"
	"github.com/redhatinsights/yggdrasil"
)

func getConsumerUUID() (string, error) {
	return yggdrasil.ReadCert("/etc/pki/consumer/cert.pem")
}

func registerPassword(username, password, serverURL string) error {
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
		return unpackError(err)
	}

	return nil
}

func registerActivationKey(orgID string, activationKeys []string, serverURL string) error {
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

	if err := privConn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Register").Call("com.redhat.RHSM1.Register.RegisterWithActivationKeys", dbus.Flags(0), orgID, activationKeys, map[string]string{}, connectionOptions, "").Err; err != nil {
		return unpackError(err)
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
		return unpackError(err)
	}

	return nil
}

func unpackError(err error) error {
	switch e := err.(type) {
	case dbus.Error:
		switch e.Name {
		case "com.redhat.RHSM1.Error":
			rhsmError := struct {
				Exception string `json:"exception"`
				Severity  string `json:"severity"`
				Message   string `json:"message"`
			}{}
			if err := json.Unmarshal([]byte(e.Error()), &rhsmError); err != nil {
				return err
			}
			return fmt.Errorf("%v: %v", rhsmError.Severity, rhsmError.Message)
		default:
			return e
		}
	default:
		return err
	}
}
