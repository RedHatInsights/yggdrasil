package main

import (
	"fmt"
	"io"

	"git.sr.ht/~spc/go-log"
	"github.com/urfave/cli/v2"
)

// BashCompleteCommand prints all visible flag options for the given command,
// and then recursively calls itself on each subcommand.
func BashCompleteCommand(cmd *cli.Command, w io.Writer) {
	for _, name := range cmd.Names() {
		if _, err := fmt.Fprintf(w, "%v\n", name); err != nil {
			log.Errorf("cannot print command name: %v", err)
		}
	}

	PrintFlagNames(cmd.VisibleFlags(), w)

	for _, command := range cmd.Subcommands {
		BashCompleteCommand(command, w)
	}
}

// PrintFlagNames prints the long and short names of each flag in the slice.
func PrintFlagNames(flags []cli.Flag, w io.Writer) {
	for _, flag := range flags {
		for _, name := range flag.Names() {
			if len(name) > 1 {
				if _, err := fmt.Fprintf(w, "--%v\n", name); err != nil {
					log.Errorf("cannot print flag names: %v", err)
				}
			} else {
				if _, err := fmt.Fprintf(w, "-%v\n", name); err != nil {
					log.Errorf("cannot print flag names: %v", err)
				}
			}
		}
	}
}

// BashComplete prints all commands, subcommands and flags to the application
// writer.
func BashComplete(c *cli.Context) {
	for _, command := range c.App.VisibleCommands() {
		BashCompleteCommand(command, c.App.Writer)

		// global flags
		PrintFlagNames(c.App.VisibleFlags(), c.App.Writer)
	}
}
