package main

import (
	"github.com/daptin/daptin-cli/src"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"os"
)

func main() {

	configFile, _ := os.UserHomeDir()
	daptinCliConfig := src.DaptinCliConfig{}

	configFileEnv, ok := os.LookupEnv("DAPTIN_CLI_CONFIG")
	if ok {
		configFile = configFileEnv
	} else {
		dirPath := configFile + string(os.PathSeparator) + ".daptin"
		if _, err := os.Stat(dirPath); err != nil {
			err = os.Mkdir(dirPath, 0644)
		}
		configFile = dirPath + string(os.PathSeparator) + "config.yaml"
	}

	if _, err := os.Stat(configFile); err == nil {
		configFileBytes, _ := ioutil.ReadFile(configFile)
		err = yaml.Unmarshal(configFileBytes, &daptinCliConfig)
	} else {
		_, _ = os.Create(configFile)
	}

	for _, config := range daptinCliConfig.Hosts {
		if config.Name == daptinCliConfig.CurrentContextName {
			daptinCliConfig.Context = config
			break
		}
	}

	appController := src.NewApplicationController(daptinCliConfig, configFile)

	app := &cli.App{
		Before: func(context *cli.Context) error {
			if daptinCliConfig.CurrentContextName == "" {
				daptinCliConfig.Context.Endpoint = context.String("endpoint")
			}
			if daptinCliConfig.CurrentContextName != "" {
				var sH src.DaptinHostEndpoint
				for _, h := range daptinCliConfig.Hosts {
					if h.Name == daptinCliConfig.CurrentContextName {
						sH = h
						break
					}
				}
				if sH.Name != "" {
					daptinCliConfig.Context = sH
				}
			}

			var daptinClientInstance daptinClient.DaptinClient

			if daptinCliConfig.Context.Token == "" {
				daptinClientInstance = daptinClient.NewDaptinClient(daptinCliConfig.Context.Endpoint, false)
			} else {
				daptinClientInstance = daptinClient.NewDaptinClientWithAuthToken(daptinCliConfig.Context.Endpoint, daptinCliConfig.Context.Token, false)
			}
			appController.SetDaptinClient(daptinClientInstance)
			worlds, err := getAllItems("world", daptinClientInstance)
			if err != nil {
				return err
			}
			appController.SetWorlds(worlds)

			actions, err := getAllItems("action", daptinClientInstance)
			if err != nil {
				return err
			}
			appController.SetActions(actions)
			outputRenderer := context.String("output")
			switch outputRenderer {
			case "table":
				appController.SetRenderer(src.NewTableRenderer())
			case "json":
				appController.SetRenderer(src.NewJsonRenderer())
			}

			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config, c",
				Usage:       "Load configuration from `FILE`",
				DefaultText: "~/.daptin/config.yaml",
				EnvVars:     []string{"DAPTIN_CLI_CONFIG"},
			},
			&cli.StringFlag{
				Name:        "output, o",
				Usage:       "output format",
				DefaultText: "table",
				Value:       "table",
				EnvVars:     []string{"DAPTIN_CLI_OUTPUT"},
			},
			&cli.StringFlag{
				Name:        "endpoint",
				Usage:       "endpoint",
				DefaultText: "http://localhost:6336",
				Value:       "http://localhost:6336",
				EnvVars:     []string{"DAPTIN_ENDPOINT"},
			},
			&cli.BoolFlag{
				Name:  "debug, v",
				Usage: "Print trace logsf",
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "set-context",
				Aliases: []string{"sc"},
				Usage:   "set the default context by name in config.yaml",
				Action:  appController.SetContext,
			},
			{
				Name:   "signin",
				Usage:  "sign in",
				Flags:  []cli.Flag{},
				Action: appController.ActionSignIn,
			},
			{
				Name:   "signin_with_2fa",
				Usage:  "Sign in with 2FA",
				Flags:  []cli.Flag{},
				Action: appController.ActionVerifyOtp,
			},
			{
				Name:   "signup",
				Usage:  "sign up",
				Flags:  []cli.Flag{},
				Action: appController.ActionSignUp,
			},
			{
				Name:  "describe",
				Usage: "show schema",
				Subcommands: []*cli.Command{
					{
						Name:   "table",
						Action: appController.ActionShowWorldSchema,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "columns",
								Usage:    "comma separated column names to output",
								Required: false,
								Value:    "",
							},
						},
					},
					{
						Name:   "action",
						Action: appController.ActionShowActionSchema,
						Flags:  []cli.Flag{},
					},
				},
			},
			{
				Name:  "list",
				Usage: "list entity",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "filter",
						Usage:    "filter by keyword",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "columns",
						Usage:    "columns to print",
						Required: false,
					},
					&cli.IntFlag{
						Name:        "pageSize",
						Usage:       "number of items per page",
						Required:    false,
						Value:       10,
						DefaultText: "10",
					},
					&cli.IntFlag{
						Name:        "pageNumber",
						Usage:       "page number",
						Required:    false,
						Value:       0,
						DefaultText: "0",
					},
				},
				Action: appController.ActionListEntity,
			},
			{
				Name:   "execute",
				Usage:  "execute an action",
				Flags:  []cli.Flag{},
				Action: appController.ActionExecute,
			},
		},
		Version: "v0.0.1",
	}

	cli.VersionFlag = &cli.BoolFlag{
		Name: "version", Aliases: []string{"v"},
		Usage: "print only the version",
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func getAllItems(entityName string, daptinClientInstance daptinClient.DaptinClient) ([]map[string]interface{}, error) {
	allWorlds, err := daptinClientInstance.FindAll(entityName, daptinClient.DaptinQueryParameters{
		"page[size]": 500,
	})
	if err != nil {
		return nil, err
	}
	worlds := src.MapArray(allWorlds, "attributes")
	return worlds, nil
}
