package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/render"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
)

func describeCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "describe",
		Usage: "Show schema information",
		Subcommands: []*cli.Command{
			{
				Name:      "table",
				Usage:     "Show table schema (columns and actions)",
				ArgsUsage: "<entity>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "columns",
						Usage: "Comma-separated column names to show from schema",
					},
				},
				Action: func(c *cli.Context) error {
					entityName := c.Args().Get(0)
					if entityName == "" {
						return fmt.Errorf("entity name required")
					}
					return describeTable(appCtx, entityName, c.String("columns"))
				},
			},
			{
				Name:      "action",
				Usage:     "Show action schema (InFields, OutFields)",
				ArgsUsage: "<entity> <action_name>",
				Action: func(c *cli.Context) error {
					entityName := c.Args().Get(0)
					actionName := c.Args().Get(1)
					if entityName == "" || actionName == "" {
						return fmt.Errorf("usage: describe action <entity> <action_name>")
					}
					return describeAction(appCtx, entityName, actionName)
				},
			},
		},
	}
}

func describeTable(appCtx *AppContext, entityName, columnsFlag string) error {
	slog.Info("describe table", "entity", entityName)
	// Fetch the world definition on demand
	worlds, err := appCtx.Client.FindAll("world", daptinClient.DaptinQueryParameters{
		"page[size]": 500,
	})
	if err != nil {
		return err
	}

	worldAttrs := client.MapArray(worlds, "attributes")
	var world map[string]interface{}
	var worldRefId string
	for _, w := range worldAttrs {
		if w["table_name"] == entityName {
			world = w
			worldRefId, _ = w["reference_id"].(string)
			break
		}
	}
	if world == nil {
		return fmt.Errorf("entity %q not found", entityName)
	}
	slog.Debug("found world", "entity", entityName, "reference_id", worldRefId)

	schemaJson, ok := world["world_schema_json"].(string)
	if !ok {
		return fmt.Errorf("no schema found for %q", entityName)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJson), &schema); err != nil {
		return err
	}
	slog.Debug("parsed schema", "entity", entityName)

	columnsData, ok := schema["Columns"].([]interface{})
	if !ok {
		return fmt.Errorf("no columns in schema")
	}

	dataMap := make([]map[string]interface{}, 0, len(columnsData))
	for _, col := range columnsData {
		if cm, ok := col.(map[string]interface{}); ok {
			dataMap = append(dataMap, cm)
		}
	}

	if columnsFlag == "" {
		dataMap = render.FilterColumns(dataMap, []string{"ColumnName", "ColumnType"})
	} else {
		dataMap = render.FilterColumns(dataMap, strings.Split(columnsFlag, ","))
	}

	if err := appCtx.Renderer.RenderArray(dataMap); err != nil {
		return err
	}

	// Show actions for this table
	actions, err := appCtx.Client.FindAll("action", daptinClient.DaptinQueryParameters{
		"page[size]": 500,
	})
	if err != nil {
		return nil // Non-fatal: just skip actions
	}

	actionAttrs := client.MapArray(actions, "attributes")
	var worldActions []map[string]interface{}
	for _, a := range actionAttrs {
		if a["world_id"] == worldRefId {
			worldActions = append(worldActions, a)
		}
	}
	slog.Debug("found actions", "entity", entityName, "count", len(worldActions))

	fmt.Fprintf(os.Stdout, "\nActions: %d\n", len(worldActions))
	if len(worldActions) > 0 {
		worldActions = render.FilterColumns(worldActions, []string{"action_name", "label", "reference_id"})
		return appCtx.Renderer.RenderArray(worldActions)
	}
	return nil
}

func describeAction(appCtx *AppContext, entityName, actionName string) error {
	schema, err := fetchActionSchemaFromServer(appCtx, entityName, actionName)
	if err != nil {
		return err
	}

	refRequired := !schema.InstanceOptional
	fmt.Printf("Action: %s\n", schema.ActionName)
	fmt.Printf("Instance action: %s\n", yesNo(refRequired))
	fmt.Printf("Reference id required: %s\n", yesNo(refRequired))
	fmt.Printf("InFields: %d\n", len(schema.InFields))
	for _, field := range schema.InFields {
		colName, _ := field["ColumnName"].(string)
		colType, _ := field["ColumnType"].(string)
		fmt.Printf("  %s: %s\n", colName, colType)
	}
	fmt.Println("Example:")
	if refRequired {
		fmt.Printf("  daptin-cli execute %s %s --reference-id <%s_reference_id>\n", entityName, actionName, entityName)
	} else {
		fmt.Printf("  daptin-cli execute %s %s\n", entityName, actionName)
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
