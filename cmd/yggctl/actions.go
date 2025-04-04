package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/godbus/dbus/v5"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/constants"
	"github.com/redhatinsights/yggdrasil/ipc"
	"github.com/urfave/cli/v2"
)

func generateDataMessageAction(ctx *cli.Context) error {
	var metadata map[string]string
	if err := json.Unmarshal([]byte(ctx.String("metadata")), &metadata); err != nil {
		return cli.Exit(fmt.Errorf("cannot unmarshal metadata: %w", err), 1)
	}

	var err error
	var content []byte
	var reader io.Reader
	if ctx.Args().First() == "-" {
		reader = os.Stdin
	} else {
		reader, err = os.Open(ctx.Args().First())
	}
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot open file for reading: %w", err), 1)
	}
	content, err = io.ReadAll(reader)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot read data: %w", err), 1)
	}

	data, err := generateMessage(
		"data",
		ctx.String("response-to"),
		ctx.String("directive"),
		content,
		metadata,
		ctx.Int("version"),
	)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot marshal message: %w", err), 1)
	}

	fmt.Println(string(data))

	return nil
}

func generateControlMessageAction(ctx *cli.Context) error {
	var err error
	var content []byte
	var reader io.Reader
	if ctx.Args().First() == "-" {
		reader = os.Stdin
	} else {
		reader, err = os.Open(ctx.Args().First())
	}
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot open file for reading: %w", err), 1)
	}
	content, err = io.ReadAll(reader)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot read data: %w", err), 1)
	}
	data, err := generateMessage(
		ctx.String("type"),
		ctx.String("response-to"),
		"",
		content,
		nil,
		ctx.Int("version"),
	)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot marshal message: %w", err), 1)
	}

	fmt.Println(string(data))

	return nil
}

func messageJournalAction(ctx *cli.Context) error {
	var conn *dbus.Conn
	var err error

	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		conn, err = dbus.ConnectSessionBus()
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot connect to session bus: %w", err), 1)
		}
	} else {
		conn, err = dbus.ConnectSystemBus()
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot connect to system bus: %w", err), 1)
		}
	}

	var journalEntries []map[string]string
	args := []interface{}{
		ctx.String("message-id"),
		ctx.String("worker"),
		ctx.String("since"),
		ctx.String("until"),
		ctx.Bool("persistent"),
	}
	obj := conn.Object("com.redhat.Yggdrasil1", "/com/redhat/Yggdrasil1")
	if err := obj.Call("com.redhat.Yggdrasil1.MessageJournal", dbus.Flags(0), args...).Store(&journalEntries); err != nil {
		return cli.Exit(fmt.Errorf("cannot list message journal entries: %v", err), 1)
	}

	switch ctx.String("format") {
	case "json":
		data, err := json.Marshal(journalEntries)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot marshal journal entries: %v", err), 1)
		}
		fmt.Println(string(data))
	case "text":
		journalTextTemplate := template.New("journalTextTemplate")
		journalTextTemplate, err := journalTextTemplate.Parse(
			"{{range .}}{{.message_id}} : {{.sent}} : {{.worker_name}} : " +
				"{{if .response_to}}{{.response_to}}{{else}}...{{end}} : " +
				"{{if .worker_event}}{{.worker_event}}{{else}}...{{end}} : " +
				"{{if .worker_data}}{{.worker_data}}{{else}}...{{end}}\n{{end}}",
		)
		if err != nil {
			return fmt.Errorf("cannot parse journal text template parameters: %w", err)
		}
		var compiledTextTemplate bytes.Buffer
		textCompileErr := journalTextTemplate.Execute(&compiledTextTemplate, journalEntries)
		if textCompileErr != nil {
			return fmt.Errorf("cannot compile journal text template: %w", textCompileErr)
		}
		fmt.Println(compiledTextTemplate.String())
	case "table":
		writer := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
		fmt.Fprint(
			writer,
			"MESSAGE #\tMESSAGE ID\tSENT\tWORKER NAME\tRESPONSE TO\tWORKER EVENT\tWORKER DATA\n",
		)
		for idx, entry := range journalEntries {
			fmt.Fprintf(
				writer,
				"%d\t%s\t%s\t%s\t%s\t%v\t%s\n",
				idx,
				entry["message_id"],
				entry["sent"],
				entry["worker_name"],
				entry["response_to"],
				entry["worker_event"],
				entry["worker_data"],
			)
		}
		if err := writer.Flush(); err != nil {
			return cli.Exit(fmt.Errorf("unable to flush tab writer: %v", err), 1)
		}
	default:
		return cli.Exit(fmt.Errorf("unknown format type: %v", ctx.String("format")), 1)
	}

	return nil
}

func workersAction(c *cli.Context) error {
	conn, err := connectBus()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot connect to bus: %w", err), 1)
	}

	obj := conn.Object("com.redhat.Yggdrasil1", "/com/redhat/Yggdrasil1")
	var workers map[string]map[string]string
	if err := obj.Call("com.redhat.Yggdrasil1.ListWorkers", dbus.Flags(0)).Store(&workers); err != nil {
		return cli.Exit(fmt.Errorf("cannot list workers: %v", err), 1)
	}

	// Remove worker names with hyphens.
	// Why:
	// Hyphenated worker names were introduced to satisfy a console-dot assumption that
	// worker names were hyphenated, after which the "yggctl workers list" command was
	// showing both the "legacy" and hyphenated  worker names. Since hyphens in worker
	// names are disallowed by the D-Bus naming policy it is safe to remove any worker
	// names with hyphens when listing them.
	for key := range workers {
		if strings.ContainsRune(key, '-') {
			delete(workers, key)
		}
	}

	switch c.String("format") {
	case "json":
		data, err := json.Marshal(workers)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot marshal workers: %v", err), 1)
		}
		fmt.Println(string(data))
	case "table":
		writer := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
		fmt.Fprintf(writer, "WORKER\tFEATURES\n")
		for worker, features := range workers {
			featureSummary, err := json.Marshal(features)
			if err != nil {
				return cli.Exit(fmt.Errorf("cannot marshal features: %v", err), 1)
			}
			fmt.Fprintf(writer, "%v\t%v\n", worker, string(featureSummary))
			_ = writer.Flush()
		}
	default:
		return cli.Exit(fmt.Errorf("unknown format type: %v", c.String("format")), 1)
	}

	return nil
}

func dispatchAction(c *cli.Context) error {
	conn, err := connectBus()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot connect to bus: %w", err), 1)
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(c.String("metadata")), &metadata); err != nil {
		return cli.Exit(fmt.Errorf("cannot unmarshal metadata: %w", err), 1)
	}

	var data []byte
	var r io.Reader
	if c.Args().First() == "-" {
		r = os.Stdin
	} else {
		r, err = os.Open(c.Args().First())
	}
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot open file for reading: %w", err), 1)
	}
	data, err = io.ReadAll(r)
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot read data: %w", err), 1)
	}

	id := uuid.New().String()

	obj := conn.Object("com.redhat.Yggdrasil1", "/com/redhat/Yggdrasil1")
	if err := obj.Call("com.redhat.Yggdrasil1.Dispatch", dbus.Flags(0), c.String("worker"), id, metadata, data).Store(); err != nil {
		return cli.Exit(fmt.Errorf("cannot dispatch message: %w", err), 1)
	}

	fmt.Printf("Dispatched message %v to worker %v\n", id, c.String("worker"))

	return nil
}

func listenAction(ctx *cli.Context) error {
	conn, err := connectBus()
	if err != nil {
		return cli.Exit(fmt.Errorf("cannot connect to bus: %w", err), 1)
	}

	if err := conn.AddMatchSignal(); err != nil {
		return cli.Exit(fmt.Errorf("cannot add match signal: %w", err), 1)
	}

	signals := make(chan *dbus.Signal)
	conn.Signal(signals)
	for s := range signals {
		switch s.Name {
		case "com.redhat.Yggdrasil1.WorkerEvent":
			worker, ok := s.Body[0].(string)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as string", s.Body[0]), 1)
			}
			name, ok := s.Body[1].(uint32)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as uint32", s.Body[1]), 1)
			}
			messageID, ok := s.Body[2].(string)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as string", s.Body[2]), 1)
			}
			responseTo, ok := s.Body[3].(string)
			if !ok {
				return cli.Exit(fmt.Errorf("cannot cast %T as string", s.Body[3]), 1)
			}
			data := map[string]string{}
			if len(s.Body) > 4 {
				data, ok = s.Body[4].(map[string]string)
				if !ok {
					return cli.Exit(fmt.Errorf("cannot cast %T as map[string]string", s.Body[4]), 1)
				}
			}
			parsedData, err := json.Marshal(data)
			if err != nil {
				return cli.Exit(fmt.Errorf("unable to parse optional data: %v", data), 1)
			}

			log.Printf(
				"%v: %v: %v: %v: %v",
				worker,
				messageID,
				ipc.WorkerEventName(name),
				responseTo,
				string(parsedData),
			)
		}
	}
	return nil
}

// generateWorkerDataAction is the cli action function for the "generate
// worker-data" subcommand. It formats and outputs files needed by workers to
// communicate with the yggdrasil service over D-Bus.
func generateWorkerDataAction(ctx *cli.Context) error {
	config := struct {
		User    string
		Group   string
		Name    string
		Program string
	}{
		User:    ctx.String("user"),
		Group:   ctx.String("group"),
		Name:    ctx.String("name"),
		Program: ctx.String("program"),
	}

	// If "Group" is unspecified, assume it matches the user.
	if config.Group == "" {
		config.Group = config.User
	}

	if err := os.MkdirAll(ctx.Path("output"), 0755); err != nil {
		return cli.Exit(
			fmt.Errorf("error: cannot create output directory %v: %v", ctx.Path("output"), err),
			1,
		)
	}

	joinpath := func(outputDir, installDir string, name string, install bool) string {
		if install {
			return filepath.Join(installDir, name)
		} else {
			return filepath.Join(outputDir, name)
		}
	}

	data := []struct {
		FileName string
		FilePath string
		Template *template.Template
	}{
		{
			FileName: fmt.Sprintf("com.redhat.Yggdrasil1.Worker1.%v.service", config.Name),
			FilePath: joinpath(
				filepath.Join(ctx.Path("output"), "dbus-1", "system-services"),
				constants.DBusSystemServicesDir,
				fmt.Sprintf("com.redhat.Yggdrasil1.Worker1.%v.service", config.Name),
				ctx.Bool("install"),
			),
			Template: template.Must(template.New("").Parse(DBusServiceTemplate)),
		},
		{
			FileName: fmt.Sprintf("com.redhat.Yggdrasil1.Worker1.%v.conf", config.Name),
			FilePath: joinpath(
				filepath.Join(ctx.Path("output"), "dbus-1", "system.d"),
				constants.DBusPolicyConfigDir,
				fmt.Sprintf("com.redhat.Yggdrasil1.Worker1.%v.conf", config.Name),
				ctx.Bool("install"),
			),
			Template: template.Must(template.New("").Parse(DBusPolicyConfigTemplate)),
		},
		{
			FileName: fmt.Sprintf("com.redhat.Yggdrasil1.Worker1.%v.service", config.Name),
			FilePath: joinpath(
				filepath.Join(ctx.Path("output"), "systemd", "system"),
				constants.SystemdSystemServicesDir,
				fmt.Sprintf("com.redhat.Yggdrasil1.Worker1.%v.service", config.Name),
				ctx.Bool("install"),
			),
			Template: template.Must(template.New("").Parse(SystemdServiceTemplate)),
		},
	}

	for _, d := range data {
		if err := os.MkdirAll(filepath.Dir(d.FilePath), 0755); err != nil {
			return cli.Exit(
				fmt.Errorf("cannot create directory %v: %v", filepath.Dir(d.FilePath), err),
				1,
			)
		}
		f, err := os.Create(d.FilePath)
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot create file %v: %v", d.FilePath, err), 1)
		}
		if err := d.Template.Execute(f, config); err != nil {
			_ = f.Close()
			return cli.Exit(fmt.Errorf("cannot write file %v: %v", d.FilePath, err), 1)
		}
		err = f.Close()
		if err != nil {
			return cli.Exit(fmt.Errorf("cannot close file %v: %v", d.FilePath, err), 1)
		}
	}

	return nil
}

func generateMessage(
	messageType string,
	responseTo string,
	directive string,
	content []byte,
	metadata map[string]string,
	version int,
) ([]byte, error) {
	switch messageType {
	case "data":
		msg, err := generateDataMessage(
			yggdrasil.MessageType(messageType),
			responseTo,
			directive,
			content,
			metadata,
			version,
		)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return data, nil
	case "command":
		msg, err := generateControlMessage(
			yggdrasil.MessageType(messageType),
			responseTo,
			version,
			content,
		)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %v", messageType)
	}
}

func connectBus() (*dbus.Conn, error) {
	var connect func(...dbus.ConnOption) (*dbus.Conn, error)
	var conn *dbus.Conn
	var err error
	var errMsg string
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" && os.Geteuid() > 0 {
		connect = dbus.ConnectSessionBus
		errMsg = "cannot connect to session bus (" + os.Getenv("DBUS_SESSION_BUS_ADDRESS") + "): %w"
	} else {
		connect = dbus.ConnectSystemBus
		errMsg = "cannot connect to system bus: %w"
	}

	conn, err = connect()
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}
	return conn, nil
}
