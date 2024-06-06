package main

import (
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"os"
)

type DaptinHostEndpoint struct {
	Name     string
	Endpoint string
	Token    string
}

type DaptinCliConfig struct {
	CurrentContextName string
	Context            DaptinHostEndpoint `json:"-"`
	Hosts              []DaptinHostEndpoint
}

func main() {

	configFile, _ := os.UserHomeDir()
	daptinCliConfig := DaptinCliConfig{}

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

	daptinHostEndpoint := DaptinHostEndpoint{}

	for _, config := range daptinCliConfig.Hosts {
		if config.Name == daptinCliConfig.CurrentContextName {
			daptinHostEndpoint = config
			daptinCliConfig.Context = daptinHostEndpoint
			break
		}
	}
	if daptinHostEndpoint.Token == "" && daptinCliConfig.Context.Token != "" {
		daptinHostEndpoint.Token = daptinCliConfig.Context.Token
	}

	appController := ApplicationController{
		daptinCliConfig: daptinCliConfig,
		configPath:      configFile,
	}

	app := &cli.App{
		Before: func(context *cli.Context) error {
			if daptinCliConfig.CurrentContextName == "" {
				daptinCliConfig.Context.Endpoint = context.String("endpoint")
			}
			if daptinCliConfig.CurrentContextName != "" {
				var sH DaptinHostEndpoint
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
			appController.daptinClient = daptinClientInstance
			worlds := getAllItems("world", daptinClientInstance)
			appController.worlds = make(map[string]map[string]interface{})
			for _, world := range worlds {
				appController.worlds[world["table_name"].(string)] = world
			}

			actions := getAllItems("action", daptinClientInstance)
			appController.actions = make(map[string]map[string]interface{})
			for _, action := range actions {
				appController.actions[action["action_name"].(string)] = action
			}
			outputRenderer := context.String("output")
			switch outputRenderer {
			case "table":
				appController.renderer = NewTableRenderer()
			case "json":
				appController.renderer = NewJsonRenderer()
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
				Usage:  "sign in",
				Flags:  []cli.Flag{},
				Action: appController.ActionSignUp,
			},
			{
				Name:  "schema",
				Usage: "show schema",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "columns",
						Usage:    "comma separated column names to output",
						Required: false,
						Value:    "",
					},
				},
				Action: appController.ActionShowSchema,
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

func getAllItems(entityName string, daptinClientInstance daptinClient.DaptinClient) []map[string]interface{} {
	allWorlds, err := daptinClientInstance.FindAll(entityName, daptinClient.DaptinQueryParameters{
		"page[size]": 500,
	})
	if err != nil {
		panic(err)
	}
	worlds := MapArray(allWorlds, "attributes")
	return worlds
}
