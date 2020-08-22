package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
)

type DaptinHostEndpoing struct {
	Name     string
	Endpoint string
	Token    string
}

type DaptinCliConfig struct {
	Token          string
	CurrentContext string
	Hosts          []DaptinHostEndpoing
}

type ApplicationController struct {
	daptinClient    daptinClient.DaptinClient
	context         DaptinHostEndpoing
	daptinCliConfig DaptinCliConfig
	configPath      string
}

func (c ApplicationController) WriteConfig() error {
	jsonStr, err := json.MarshalIndent(c.daptinCliConfig, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(c.configPath, jsonStr, 0644)
}

func (c ApplicationController) SetContext(context *cli.Context) error {

	for _, host := range c.daptinCliConfig.Hosts {
		if host.Name == context.String("name") {
			c.daptinCliConfig.CurrentContext = host.Name
			return c.WriteConfig()
		}
	}

	return errors.New(fmt.Sprintf("invalid name [%v], not found in config", context.String("name")))
}

func (c ApplicationController) ActionBeforeSignIn(context *cli.Context) error {
	if c.context.Endpoint == "" && context.String("endpoint") == "" {
		log.Printf("Set endpoint is config and set a endpoint with --endpoint http://localhost:6336")
		return errors.New("no endpoint found")
	}
	return nil
}

func (c ApplicationController) ActionSignIn(context *cli.Context) error {

	responses, err := c.daptinClient.Execute("signin", "user_account", map[string]interface{}{
		"email":    context.String("email"),
		"password": context.String("password"),
	})
	if err != nil {
		return err
	}

	return c.HandleActionResponse(responses)
}

func (c ApplicationController) ActionVerifyOtp(context *cli.Context) error {

	responses, err := c.daptinClient.Execute("verify_otp", "user_account", map[string]interface{}{
		"otp":           context.String("otp"),
		"mobile_number": context.String("mobile_number"),
	})
	if err != nil {
		return err
	}

	return c.HandleActionResponse(responses)
}

func (c ApplicationController) HandleActionResponse(responses daptinClient.DaptinActionResponse) error {
	//for _, response := range responses {
	//
	//}
}

func (c ApplicationController) ActionSignUp(context *cli.Context) error {

	return c.ActionSignIn(context)
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

	daptinClientInstance := daptinClient.NewDaptinClient("http://localhost:6336")

	daptinHostEndpoint := DaptinHostEndpoing{}

	for _, config := range daptinCliConfig.Hosts {
		if config.Name == daptinCliConfig.CurrentContext {
			daptinHostEndpoint = config
			break
		}
	}
	if daptinHostEndpoint.Token == "" && daptinCliConfig.Token != "" {
		daptinHostEndpoint.Token = daptinCliConfig.Token
	}

	appController := ApplicationController{
		daptinClient:    daptinClientInstance,
		context:         daptinHostEndpoint,
		daptinCliConfig: daptinCliConfig,
		configPath:      configFile,
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
				Usage:   "set the default context by name in config.yaml",
				Action:  appController.SetContext,
			},
			{
				Name:    "signin",
				Aliases: []string{"si"},
				Usage:   "sign in",
				Before:  appController.ActionBeforeSignIn,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "email",
						Usage:    "Email",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "otp",
						Usage: "OTP",
					},
					&cli.StringFlag{
						Name:     "password",
						Required: true,
						Usage:    "Password",
					},
					&cli.StringFlag{
						Name:        "endpoint",
						Usage:       "Endpoint",
						DefaultText: "http://localhost:6336",
					},
				},
				Action: appController.ActionSignIn,
			},
			{
				Name:    "signup",
				Aliases: []string{"si"},
				Usage:   "sign in",
				Before:  appController.ActionBeforeSignIn,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "email",
						Usage:    "Email",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "name",
						Usage:    "Name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "password",
						Required: true,
						Usage:    "Password",
					},
					&cli.StringFlag{
						Name:     "passwordConfirm",
						Required: true,
						Usage:    "Password confirm",
					},
					&cli.StringFlag{
						Name:        "endpoint",
						Usage:       "Endpoint",
						DefaultText: "http://localhost:6336",
					},
				},
				Action: appController.ActionSignUp,
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
