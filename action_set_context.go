package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/artpar/api2go"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"strings"
)

type ApplicationController struct {
	daptinClient    daptinClient.DaptinClient
	daptinCliConfig DaptinCliConfig
	configPath      string
	worlds          map[string]map[string]interface{}
	renderer        *TableRenderer
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
	columnsFromArgs := context.String("columns")

	colNames := make([]string, 0)
	if len(columnsFromArgs) > 0 {
		cols := strings.Split(columnsFromArgs, ",")
		for _, col := range cols {
			colNames = append(colNames, col)
		}
	}

	result, err := c.daptinClient.FindAll(entityName, params)
	if err != nil {
		return err
	}

	if len(result) == 0 {
		fmt.Println("No entities found")
		return nil
	}

	resultSet := MapArray(result, "attributes")

	//if len(colNames) == 0 {
	//	row0 := result[0]
	//	for key, _ := range row0 {
	//		colNames = append(colNames, key)
	//	}
	//} else {
	//	resultSet = FilterColumn(resultSet, colNames)
	//}

	if len(colNames) > 0 {
		resultSet = FilterColumn(resultSet, colNames)
	}
	c.renderer.RenderArray(resultSet)

	return nil
}

func FilterColumn(array []map[string]interface{}, includedColumnNames []string) []map[string]interface{} {
	for _, row := range array {
		for colName, _ := range row {
			found := false
			for _, includedName := range includedColumnNames {
				if colName == includedName {
					found = true
				}
			}
			if !found {
				delete(row, colName)
			}
		}
	}
	return array
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

type AuthPermission int64

type LoopbookFsmDescription struct {
	InitialState string
	Name         string
	Label        string
	Events       []LoopbackEventDesc
}

type LoopbackEventDesc struct {
	// Name is the event name used when calling for a transition.
	Name  string
	Label string
	Color string

	// Src is a slice of source states that the FSM must be in to perform a
	// state transition.
	Src []string

	// Dst is the destination state that the FSM will be in if the transition
	// succeeds.
	Dst string
}

type ColumnTag struct {
	ColumnName string
	Tags       string
}

type TableInfo struct {
	TableName              string `db:"table_name"`
	TableId                int
	DefaultPermission      AuthPermission `db:"default_permission"`
	Columns                []api2go.ColumnInfo
	StateMachines          []LoopbookFsmDescription
	Relations              []api2go.TableRelation
	IsTopLevel             bool `db:"is_top_level"`
	Permission             AuthPermission
	UserId                 uint64              `db:"user_account_id"`
	IsHidden               bool                `db:"is_hidden"`
	IsJoinTable            bool                `db:"is_join_table"`
	IsStateTrackingEnabled bool                `db:"is_state_tracking_enabled"`
	IsAuditEnabled         bool                `db:"is_audit_enabled"`
	TranslationsEnabled    bool                `db:"translation_enabled"`
	DefaultGroups          []string            `db:"default_groups"`
	DefaultRelations       map[string][]string `db:"default_relations"`
	Validations            []ColumnTag
	Conformations          []ColumnTag
	DefaultOrder           string
	Icon                   string
	CompositeKeys          [][]string
}

func (c *ApplicationController) ActionShowSchema(context *cli.Context) error {
	entityName := context.String("name")
	world, ok := c.worlds[entityName]
	if !ok {
		panic(errors.New("entity not found: " + entityName))
	}

	schemaJson := world["world_schema_json"].(string)
	var worldSchemaStruct TableInfo
	var mapHolder map[string]interface{}
	err := json.Unmarshal([]byte(schemaJson), &worldSchemaStruct)
	err = json.Unmarshal([]byte(schemaJson), &mapHolder)
	if err != nil {
		panic(err)
	}

	data := mapHolder["Columns"].([]interface{})
	dataMap := make([]map[string]interface{}, len(data))
	for i, row := range data {
		dataMap[i] = row.(map[string]interface{})
	}
	c.renderer.RenderArray(dataMap)
	return err
}
