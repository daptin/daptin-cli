package main

import (
	"errors"
	"fmt"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
)

type ApplicationController struct {
	daptinClient    daptinClient.DaptinClient
	daptinCliConfig DaptinCliConfig
	configPath      string
	worlds          []map[string]interface{}
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

func (c *ApplicationController) ActionSignUp(context *cli.Context) error {

	return c.ActionSignIn(context)
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

func (c *ApplicationController) ActionListEntity(context *cli.Context) error {

	entityName := context.String("name")
	query := context.String("query")
	params := daptinClient.DaptinQueryParameters{}
	if len(query) > 0 {
		params["query"] = query
	}
	params["page[size]"] = context.Int("pageSize")
	result, err := c.daptinClient.FindAll(entityName, params)
	if err != nil {
		return err
	}

	if len(result) == 0 {
		fmt.Println("No entities found")
		return nil
	}

	colNames := make([]string, 0)
	row0 := result[0]
	for key, _ := range row0 {
		colNames = append(colNames, key)
	}

	PrintTable(MapArray(result, "attributes"))

	return nil
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
