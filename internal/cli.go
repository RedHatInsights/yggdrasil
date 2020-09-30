package yggdrasil

import (
	yggdrasil "github.com/redhatinsights/yggdrasil"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

// NewApp creates a new cli Application with name and default flags.
func NewApp(name string) (*cli.App, error) {
	app := cli.NewApp()
	app.Name = name
	app.Version = yggdrasil.Version

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
			Name: "base-url",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "auth-mode",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "username",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "password",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "cert-file",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "key-file",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name: "ca-root",
		}),
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
