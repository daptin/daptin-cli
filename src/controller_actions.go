package src

import (
	"encoding/json"
	"errors"
	"fmt"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type ApplicationController struct {
	daptinClient    daptinClient.DaptinClient
	daptinCliConfig DaptinCliConfig
	configPath      string
	worlds          map[string]map[string]interface{}
	renderer        Renderer
	actions         map[string]map[string]interface{}
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

	responses, err := c.daptinClient.Execute("signup", "user_account", map[string]interface{}{
		"email":           context.Args().Get(0),
		"password":        context.Args().Get(1),
		"passwordConfirm": context.Args().Get(2),
	})
	if err != nil {
		return err
	}

	return c.HandleActionResponse(responses)

}

func (c *ApplicationController) ActionSignIn(context *cli.Context) error {

	responses, err := c.daptinClient.Execute("signin", "user_account", map[string]interface{}{
		"email":    context.Args().Get(0),
		"password": context.Args().Get(1),
	})
	if err != nil {
		return err
	}

	return c.HandleActionResponse(responses)
}

func (c *ApplicationController) ActionListEntity(context *cli.Context) error {

	entityName := context.Args().Get(0)
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

	if len(colNames) > 0 {
		resultSet = FilterColumn(resultSet, colNames)
	}
	return c.renderer.RenderArray(resultSet)
}

func FilterColumn(array []map[string]interface{}, includedColumnNames []string) []map[string]interface{} {
	for i, row := range array {
		array[i] = IncludeColumnFromMap(row, includedColumnNames)
	}
	return array
}

func IncludeColumnFromMap(row map[string]interface{}, includedColumnNames []string) map[string]interface{} {
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
	return row
}

func ExcludeColumnFromMap(row map[string]interface{}, excludedColumnNames []string) map[string]interface{} {
	for colName, _ := range row {
		found := false
		for _, includedName := range excludedColumnNames {
			if colName == includedName {
				found = true
			}
		}
		if found {
			delete(row, colName)
		}
	}
	return row
}

func (c *ApplicationController) ActionVerifyOtp(context *cli.Context) error {

	responses, err := c.daptinClient.Execute("signin_with_2fa", "user_account", map[string]interface{}{
		"email":    context.Args().Get(0),
		"password": context.Args().Get(1),
		"otp":      context.Args().Get(2),
	})
	if err != nil {
		return err
	}

	return c.HandleActionResponse(responses)
}

func (c *ApplicationController) ExecuteAction(context *cli.Context) error {

	//actionName := context.String("name")

	responses, err := c.daptinClient.Execute("signin_with_2fa", "user_account", map[string]interface{}{
		"email":    context.Args().Get(0),
		"password": context.Args().Get(1),
		"otp":      context.Args().Get(2),
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

type ForeignKeyData struct {
	DataSource string
	Namespace  string
	KeyName    string
}

type ColumnInfo struct {
	Name              string         `db:"name"`
	ColumnName        string         `db:"column_name"`
	ColumnDescription string         `db:"column_description"`
	ColumnType        string         `db:"column_type"`
	IsPrimaryKey      bool           `db:"is_primary_key"`
	IsAutoIncrement   bool           `db:"is_auto_increment"`
	IsIndexed         bool           `db:"is_indexed"`
	IsUnique          bool           `db:"is_unique"`
	IsNullable        bool           `db:"is_nullable"`
	Permission        uint64         `db:"permission"`
	IsForeignKey      bool           `db:"is_foreign_key"`
	ExcludeFromApi    bool           `db:"include_in_api"`
	ForeignKeyData    ForeignKeyData `db:"foreign_key_data"`
	DataType          string         `db:"data_type"`
	DefaultValue      string         `db:"default_value"`
	Options           []ValueOptions
}
type ValueOptions struct {
	ValueType string
	Value     interface{}
	Label     string
}

type TableRelation struct {
	Subject     string
	Object      string
	Relation    string
	SubjectName string
	ObjectName  string
	Columns     []ColumnInfo
}

type TableInfo struct {
	TableName              string `db:"table_name"`
	TableId                int
	DefaultPermission      AuthPermission `db:"default_permission"`
	Columns                []ColumnInfo
	StateMachines          []LoopbookFsmDescription
	Relations              []TableRelation
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

func (c *ApplicationController) ActionShowWorldSchema(context *cli.Context) (err error) {
	entityName := context.Args().Get(0)

	world, ok := c.worlds[entityName]
	if !ok {
		err := fmt.Errorf("entity [%s] not found", entityName)
		return err
	}

	schemaJson := world["world_schema_json"].(string)
	//var worldSchemaStruct TableInfo
	var mapHolder map[string]interface{}
	//err := json.Unmarshal([]byte(schemaJson), &worldSchemaStruct)
	err = json.Unmarshal([]byte(schemaJson), &mapHolder)
	if err != nil {
		return err
	}

	data := mapHolder["Columns"].([]interface{})
	dataMap := make([]map[string]interface{}, len(data))
	for i, row := range data {
		dataMap[i] = row.(map[string]interface{})
	}

	columns := context.String("columns")

	if len(columns) == 0 {
		dataMap = FilterColumn(dataMap, []string{"ColumnName", "ColumnType"})
	} else {
		columList := strings.Split(columns, ",")
		dataMap = FilterColumn(dataMap, columList)
	}
	err = c.renderer.RenderArray(dataMap)
	worldActions := make([]map[string]interface{}, 0)
	for _, action := range c.actions {
		if action["world_id"].(string) == world["reference_id"] {
			worldActions = append(worldActions, action)
		}
	}
	_, err = fmt.Fprintf(os.Stdout, "\nActions: %d\n", len(worldActions))
	if len(worldActions) > 0 {
		worldActions = FilterColumn(worldActions, []string{"action_name", "label", "reference_id"})
		err = c.renderer.RenderArray(worldActions)
	}

	return err
}

func (c *ApplicationController) ActionShowActionSchema(context *cli.Context) error {
	worldName := context.Args().Get(0)
	actionName := context.Args().Get(1)
	world, err := c.GetWorldByName(worldName)
	if err != nil {
		return err
	}
	action, err := c.GetAction(world["reference_id"].(string), actionName)
	if err != nil {
		return err
	}
	var actionSchema map[string]interface{}
	err = json.Unmarshal([]byte(action["action_schema"].(string)), &actionSchema)
	if err != nil {
		return err
	}
	inFields := actionSchema["InFields"].([]interface{})
	fmt.Printf("InFields: %v\n", len(inFields))
	for _, inField := range inFields {
		inFieldMap := inField.(map[string]interface{})
		fmt.Printf("\t%s: %s\n", inFieldMap["ColumnName"], inFieldMap["ColumnType"])
	}

	outFields := actionSchema["OutFields"].([]interface{})
	fmt.Printf("OutFields: %v\n", len(outFields))
	for i, outField := range outFields {
		outFieldMap := outField.(map[string]interface{})
		fmt.Printf("\t%d: %v\n", i, outFieldMap["Type"])
	}
	actionSchema = ExcludeColumnFromMap(actionSchema, []string{"InFields", "OutFields"})
	return c.renderer.RenderObject(actionSchema)
}

func (c *ApplicationController) ActionExecute(context *cli.Context) error {
	worldName := context.Args().Get(0)
	actionName := context.Args().Get(1)
	log.Printf("Execute action [%s][%s]\n", worldName, actionName)
	world, err := c.GetWorldByName(worldName)
	if err != nil {
		return err
	}
	action, err := c.GetAction(world["reference_id"].(string), actionName)
	if err != nil {
		return err
	}
	var actionSchema map[string]interface{}
	err = json.Unmarshal([]byte(action["action_schema"].(string)), &actionSchema)
	if err != nil {
		return err
	}
	err = c.renderer.RenderObject(actionSchema)

	_, err = c.daptinClient.Execute("signin", "user_account", map[string]interface{}{
		"email":    context.Args().Get(0),
		"password": context.Args().Get(1),
	})
	//log.Printf("%v -%v", action, responses)
	if err != nil {
		return err
	}

	return err
}

func (c *ApplicationController) GetWorldByName(name string) (map[string]interface{}, error) {
	for _, world := range c.worlds {
		if world["table_name"] == name {
			return world, nil
		}
	}

	return nil, errors.New("world not found [" + name + "]")
}

func (c *ApplicationController) GetAction(worldId string, actionName string) (map[string]interface{}, error) {
	for _, action := range c.actions {
		if action["world_id"].(string) == worldId && action["action_name"].(string) == actionName {
			return action, nil
		}
	}
	return nil, errors.New("action not found [" + actionName + "]")
}

func (c *ApplicationController) SetDaptinClient(instance daptinClient.DaptinClient) {
	c.daptinClient = instance
}

func (appController *ApplicationController) SetWorlds(worlds []map[string]interface{}) {
	appController.worlds = make(map[string]map[string]interface{})
	for _, world := range worlds {
		appController.worlds[world["table_name"].(string)] = world
	}

}

func (appController *ApplicationController) SetActions(actions []map[string]interface{}) {
	appController.actions = make(map[string]map[string]interface{})
	for _, action := range actions {
		appController.actions[action["action_name"].(string)] = action
	}
}

func (appController *ApplicationController) SetRenderer(renderer Renderer) {
	appController.renderer = renderer
}

func NewApplicationController(config DaptinCliConfig, file string) *ApplicationController {
	return &ApplicationController{
		daptinCliConfig: config,
		configPath:      file,
	}
}
