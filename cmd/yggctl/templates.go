package main

var DBusServiceTemplate = `[D-BUS Service]
Name=com.redhat.Yggdrasil1.Worker1.{{ .Name }}
SystemdService=com.redhat.Yggdrasil1.Worker1.{{ .Name }}.service
`

var DBusPolicyConfigTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-BUS Bus Configuration 1.0//EN" "https://dbus.freedesktop.org/doc/busconfig.dtd">
<busconfig>
    <policy group="{{ .Group }}">
        <!-- Only {{ .Group }} can own the Worker1.{{ .Name }} name. -->
        <allow own="com.redhat.Yggdrasil1.Worker1.{{ .Name }}" />

        <!-- Only {{ .Group }} can send messages to the Worker1 interface. -->
        <allow send_destination="com.redhat.Yggdrasil1.Worker1.{{ .Name }}"
            send_interface="com.redhat.Yggdrasil1.Worker1" />

        <!-- Only {{ .Group }} can send messages to the Properties interface. -->
        <allow send_destination="com.redhat.Yggdrasil1.Worker1.{{ .Name }}"
            send_interface="org.freedesktop.DBus.Properties" />

        <!-- Only {{ .Group }} can send messages to the Introspectable interface. -->
        <allow send_destination="com.redhat.Yggdrasil1.Worker1.{{ .Name }}"
            send_interface="org.freedesktop.DBus.Introspectable" />

        <!-- Only {{ .Group }} can send messages to the Peer interface. -->
        <allow send_destination="com.redhat.Yggdrasil1.Worker1.{{ .Name }}"
            send_interface="org.freedesktop.DBus.Peer" />
    </policy>
</busconfig>
`

var SystemdServiceTemplate = `[Unit]
Description=yggdrasil {{ .Name }} worker service
Documentation=https://github.com/RedHatInsights/yggdrasil

[Service]
Type=dbus
User={{ .User }}
Group={{ .Group }}
ExecStart={{ .Program }}
BusName=com.redhat.Yggdrasil1.Worker1.{{ .Name }}

[Install]
WantedBy=multi-user.target
`
