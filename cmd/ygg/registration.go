package main

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

func registerSubprocess(username, password string) error {
	cmd := exec.Command("subscription-manager", "register", "--username", username, "--password", password)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

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
	fmt.Println(privateDbusSocketURI)

	privConn, err := dbus.Dial(privateDbusSocketURI)
	if err != nil {
		return err
	}
	defer privConn.Close()

	registerObject := privConn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Register")
	node, err := introspect.Call(registerObject)
	if err != nil {
		return err
	}
	fmt.Println(node)
	var responseBody string
	call := registerObject.Call("com.redhat.RHSM1.Register.Register", dbus.Flags(0), "", username, password, map[string]string{}, map[string]string{}, "")
	if err := call.Store(&responseBody); err != nil {
		return err
	}
	var response struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal([]byte(responseBody), &response); err != nil {
		return err
	}

	// Do something with UUID like tell rhcd to sub to the topic
	fmt.Printf("Consumer ID: %v", response.UUID)

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
	if err := object.Call("com.redhat.RHSM1.Unregister.Unregister", dbus.FlagNoReplyExpected, "").Err; err != nil {
		return err
	}

	return nil
}
