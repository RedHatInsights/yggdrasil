package yggdrasil

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

// NewApp creates a new cli Application with name and default flags.
func NewApp() (*cli.App, error) {
	app := cli.NewApp()
	app.Version = Version

	defaultConfigFilePath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:      "config",
			Value:     defaultConfigFilePath,
			TakesFile: true,
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "log-level",
		}),
	}

	// This BeforeFunc will load flag values from a config file only if the
	// "config" flag value is non-zero.
	app.Before = func(c *cli.Context) error {
		if c.String("config") != "" {
			inputSource, err := altsrc.NewTomlSourceFromFlagFunc("config")(c)
			if err != nil {
				return err
			}
			return altsrc.ApplyInputSourceValues(c, inputSource, app.Flags)
		}
		return nil
	}

	return app, nil
}
