package main

import (
	"os"

	"nfbb/bot"

	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "Set debug mode, default false",
		},
		cli.BoolFlag{
			Name:  "updates, u",
			Usage: "Set method get updates: webhook or long polling. Default long polling",
		},
	}
	app.Action = func(c *cli.Context) error {
		debug := c.GlobalBool("debug")
		updatesMode := c.GlobalBool("updates")
		bot.Run(debug, updatesMode)
		return nil
	}
	app.Run(os.Args)
}
