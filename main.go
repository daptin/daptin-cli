package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

type DaptinHostEndpoing struct {
	Name     string
	Endpoint string
}

type DaptinCliConfig struct {
	AuthToken      string
	CurrentContext string
	Hosts          []DaptinHostEndpoing
}

func main() {

	configFile, _ := os.UserHomeDir()
	daptinCliConfig := DaptinCliConfig{}

	configFile, ok := os.LookupEnv("DAPTIN_CLI_CONFIG")
	if !ok {
		configFile = configFile + string(os.PathSeparator) + ".daptin" + string(os.PathSeparator) + "config.yaml"
	}

	if _, err := os.Stat(configFile); err == nil {
		configFileBytes, _ := ioutil.ReadFile(configFile)
		err = json.Unmarshal(configFileBytes, &daptinCliConfig)
	} else {
		_, _ = os.Create(configFile)
	}

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config, c",
				Usage:       "Load configuration from `FILE`",
				DefaultText: "~/.daptin/config.yaml",
				EnvVars:     []string{"DAPTIN_CLI_CONFIG"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "set-context",
				Aliases: []string{"sc"},
				Usage:   "change current context to a different one in ~/.daptin/config.yaml",
				Action: func(c *cli.Context) error {
					return nil
				},
			},
			{
				Name:    "signin",
				Aliases: []string{"si"},
				Usage:   "add a task to the list",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "email",
						Usage: "Email",
					},
					&cli.StringFlag{
						Name:        "endpoint",
						Usage:       "Endpoint",
						DefaultText: "http://localhost:6336",
					},
				},
				Action: func(context *cli.Context) error {

					return nil
				},
			},
		},
		Version: "v0.0.1",
	}

	cli.VersionFlag = &cli.BoolFlag{
		Name: "version", Aliases: []string{"V"},
		Usage: "print only the version",
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
