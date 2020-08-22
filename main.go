package main

import (
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
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
	CurrentContextName string
	Context            DaptinHostEndpoing `json:"-"`
	Hosts              []DaptinHostEndpoing
}

type ApplicationController struct {
	daptinClient    daptinClient.DaptinClient
	daptinCliConfig DaptinCliConfig
	configPath      string
}

func (c *ApplicationController) WriteConfig() {
	yamlStr, err := yaml.Marshal(c.daptinCliConfig)
	if err != nil {
		log.Printf("Failed to marshal json to save config: %v", err)
		return
	}
	err = ioutil.WriteFile(c.configPath, yamlStr, 0644)
	if err != nil {
		log.Printf("Failed to write config: %v", err)
	}
}

func (c *ApplicationController) SetContext(context *cli.Context) error {

	for _, host := range c.daptinCliConfig.Hosts {
		if host.Name == context.String("name") {
			c.daptinCliConfig.CurrentContextName = host.Name
			c.WriteConfig()
			return nil
		}
	}

	return errors.New(fmt.Sprintf("invalid name [%v], not found in config", context.String("name")))
}

func (c *ApplicationController) ActionBeforeSignIn(context *cli.Context) error {
	endpoint := context.String("endpoint")
	if c.daptinCliConfig.Context.Endpoint == "" && endpoint == "" {
		log.Printf("Set endpoint is config and set a endpoint with --endpoint http://localhost:6336")
		return errors.New("no endpoint found")
	}
	log.Printf("Endpoint: %s", endpoint)
	if c.daptinCliConfig.CurrentContextName == "" && endpoint != "" {
		c.daptinCliConfig.Context.Endpoint = endpoint
	}
	if c.daptinCliConfig.CurrentContextName != "" {
		var sH DaptinHostEndpoing
		for _, h := range c.daptinCliConfig.Hosts {
			if h.Name == c.daptinCliConfig.CurrentContextName || h.Endpoint == endpoint {
				sH = h
				break
			}
		}
		if sH.Name != "" {
			c.daptinCliConfig.Context = sH
		}
	}
	if endpoint != "" && endpoint != c.daptinCliConfig.Context.Endpoint {
		c.daptinCliConfig.Context.Endpoint = endpoint
		c.daptinCliConfig.Context.Name = endpoint
		c.daptinCliConfig.Context.Token = ""
	}

	daptinClientInstance := daptinClient.NewDaptinClient(c.daptinCliConfig.Context.Endpoint, false)
	if context.Bool("debug") {
		daptinClientInstance.SetDebug(true)
	}
	c.daptinClient = daptinClientInstance

	return nil
}

func (c *ApplicationController) ActionSignIn(context *cli.Context) error {

	responses, err := c.daptinClient.Execute("signin", "user_account", map[string]interface{}{
		"email":    context.String("email"),
		"password": context.String("password"),
	})
	if err != nil {
		return err
	}

	return c.HandleActionResponse(responses)
}

func (c *ApplicationController) ActionVerifyOtp(context *cli.Context) error {

	responses, err := c.daptinClient.Execute("signin_with_2fa", "user_account", map[string]interface{}{
		"otp":      context.String("otp"),
		"email":    context.String("email"),
		"password": context.String("password"),
	})
	if err != nil {
		return err
	}

	return c.HandleActionResponse(responses)
}

func (c *ApplicationController) HandleActionResponse(responses []daptinClient.DaptinActionResponse) error {
	for _, response := range responses {
		//log.Printf("Action Response: %v", response)
		switch response.ResponseType {
		case "client.store.set":
			keyName := response.Attributes["key"]
			if keyName == "token" {
				c.daptinCliConfig.Context.Token = response.Attributes["value"].(string)

				hostPresent := false

				for i, h := range c.daptinCliConfig.Hosts {
					if h.Endpoint == c.daptinCliConfig.Context.Endpoint || h.Name == c.daptinCliConfig.Context.Name {
						hostPresent = true
						h.Token = c.daptinCliConfig.Context.Token
						c.daptinCliConfig.Hosts[i] = h
						c.daptinCliConfig.CurrentContextName = h.Name
					}
				}
				if !hostPresent {
					c.daptinCliConfig.Context.Name = c.daptinCliConfig.Context.Endpoint
					c.daptinCliConfig.Context.Token = c.daptinCliConfig.Context.Token
					c.daptinCliConfig.Hosts = append(c.daptinCliConfig.Hosts, c.daptinCliConfig.Context)
				}
				c.daptinCliConfig.CurrentContextName = c.daptinCliConfig.Context.Name
				c.WriteConfig()
			}
		case "client.cookie.set":
		case "client.notify":
			log.Printf("Notice: %s", response.Attributes["message"])
		case "client.redirect":
		}
	}
	return nil
}

func (c *ApplicationController) ActionSignUp(context *cli.Context) error {

	return c.ActionSignIn(context)
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

	daptinHostEndpoint := DaptinHostEndpoing{}

	for _, config := range daptinCliConfig.Hosts {
		if config.Name == daptinCliConfig.CurrentContextName {
			daptinHostEndpoint = config
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

			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config, c",
				Usage:       "Load configuration from `FILE`",
				DefaultText: "~/.daptin/config.yaml",
				EnvVars:     []string{"DAPTIN_CLI_CONFIG"},
			},
			&cli.BoolFlag{
				Name:  "debug, v",
				Usage: "Print trace logs",
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
				Before: appController.ActionBeforeSignIn,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "email",
						Usage:    "Email",
						Required: true,
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
				Name:   "signin_with_2fa",
				Usage:  "Sign in with 2FA",
				Before: appController.ActionBeforeSignIn,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "email",
						Usage:    "Email",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "password",
						Usage:    "Password",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "otp",
						Required: true,
						Usage:    "OTP",
					},
					&cli.StringFlag{
						Name:        "endpoint",
						Usage:       "Endpoint",
						DefaultText: "http://localhost:6336",
					},
				},
				Action: appController.ActionVerifyOtp,
			},
			{
				Name:   "signup",
				Usage:  "sign in",
				Before: appController.ActionBeforeSignIn,
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
