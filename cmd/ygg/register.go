package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/godbus/dbus/v5"
)

func getConsumerUUID() (string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	var uuid string
	if err := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Consumer").Call("com.redhat.RHSM1.Consumer.GetUuid", dbus.Flags(0), "").Store(&uuid); err != nil {
		return "", unpackError(err)
	}
	return uuid, nil
}

func registerPassword(username, password, serverURL string) error {
	if serverURL != "" {
		if err := configureRHSM(serverURL); err != nil {
			return fmt.Errorf("cannot configure RHSM: %w", err)
		}
	}

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

	if err := privConn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Register").Call("com.redhat.RHSM1.Register.Register", dbus.Flags(0), "", username, password, map[string]string{}, map[string]string{}, "").Err; err != nil {
		return unpackError(err)
	}

	return nil
}

func registerActivationKey(orgID string, activationKeys []string, serverURL string) error {
	if serverURL != "" {
		if err := configureRHSM(serverURL); err != nil {
			return fmt.Errorf("cannot configure RHSM: %w", err)
		}
	}

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

	if err := privConn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Register").Call("com.redhat.RHSM1.Register.RegisterWithActivationKeys", dbus.Flags(0), orgID, activationKeys, map[string]string{}, map[string]string{}, "").Err; err != nil {
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

func configureRHSM(serverURL string) error {
	if _, err := os.Stat("/etc/rhsm/rhsm.conf.orig"); os.IsNotExist(err) {
		src, err := os.Open("/etc/rhsm/rhsm.conf")
		if err != nil {
			return fmt.Errorf("cannot open file for reading: %w", err)
		}
		defer src.Close()

		dst, err := os.Create("/etc/rhsm/rhsm.conf.orig")
		if err != nil {
			return fmt.Errorf("cannot open file for writing: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("cannot backup rhsm.conf: %w", err)
		}
		src.Close()
		dst.Close()
	}

	URL, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %w", err)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("cannot connect to system D-Bus: %w", err)
	}

	config := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")

	// If the scheme is empty, attempt to set the server.hostname based on the
	// path component alone. This enables the --server argument to accept just a
	// host name without a full URI.
	if URL.Scheme == "" {
		if URL.Path != "" {
			if err := config.Call("com.redhat.RHSM1.Config.Set", 0, "server.hostname", URL.Path, "").Err; err != nil {
				return unpackError(err)
			}
		}
	} else {
		if URL.Hostname() != "" {
			if err := config.Call("com.redhat.RHSM1.Config.Set", 0, "server.hostname", URL.Hostname(), "").Err; err != nil {
				return unpackError(err)
			}
		}

		if URL.Port() != "" {
			if err := config.Call("com.redhat.RHSM1.Config.Set", 0, "server.port", URL.Port(), "").Err; err != nil {
				return unpackError(err)
			}
		}

		if URL.Path != "" {
			if err := config.Call("com.redhat.RHSM1.Config.Set", 0, "server.prefix", URL.Path, "").Err; err != nil {
				return unpackError(err)
			}
		}
	}

	return nil
}
