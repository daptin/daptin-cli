package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/render"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
)

func listCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List rows of an entity",
		ArgsUsage: "<entity>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "columns",
				Usage: "Comma-separated column names to show",
			},
			&cli.IntFlag{
				Name:  "page-size",
				Usage: "Number of items per page",
				Value: 10,
			},
			&cli.IntFlag{
				Name:  "page",
				Usage: "Page number",
				Value: 1,
			},
			&cli.StringFlag{
				Name:  "sort",
				Usage: "Sort column (prefix - for descending)",
			},
			&cli.StringFlag{
				Name:  "filter",
				Usage: "Filter expression",
			},
			&cli.StringFlag{
				Name:  "include",
				Usage: "Comma-separated relation names to include",
			},
		},
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			if entityName == "" {
				return fmt.Errorf("entity name required")
			}

			params := daptinClient.DaptinQueryParameters{
				"page[size]":   c.Int("page-size"),
				"page[number]": c.Int("page"),
			}
			if s := c.String("sort"); s != "" {
				params["sort"] = s
			}
			if f := c.String("filter"); f != "" {
				clauses, err := ParseFilter(f)
				if err != nil {
					return err
				}
				params["query"] = FilterToJSON(clauses)
			}
			if inc := c.String("include"); inc != "" {
				params["included_relations"] = inc
			}

			result, err := appCtx.Client.FindAll(entityName, params)
			if err != nil {
				return err
			}
			if len(result) == 0 {
				fmt.Println("No rows found")
				return nil
			}

			rows := client.MapArray(result, "attributes")
			if appCtx.Quiet {
				return printRefs(rows)
			}
			if cols := c.String("columns"); cols != "" {
				rows = render.FilterColumns(rows, strings.Split(cols, ","))
			}
			return appCtx.Renderer.RenderArray(rows)
		},
	}
}

func getCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a single row by reference_id",
		ArgsUsage: "<entity> <reference_id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "columns",
				Usage: "Comma-separated column names to show",
			},
		},
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			referenceId := c.Args().Get(1)
			if entityName == "" || referenceId == "" {
				return fmt.Errorf("usage: get <entity> <reference_id>")
			}

			result, err := appCtx.Client.FindOne(entityName, referenceId, nil)
			if err != nil {
				return err
			}

			row, ok := result["attributes"].(map[string]interface{})
			if !ok {
				fmt.Println("No data found")
				return nil
			}

			if appCtx.Quiet {
				return printRef(row)
			}
			if cols := c.String("columns"); cols != "" {
				row = render.IncludeColumns(row, strings.Split(cols, ","))
			}
			return appCtx.Renderer.RenderObject(row)
		},
	}
}

func createCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a new row",
		ArgsUsage: "<entity> [key=val ...] or <entity> <json>",
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			if entityName == "" {
				return fmt.Errorf("entity name required")
			}

			attrs, err := parseAttributes(c.Args().Slice()[1:])
			if err != nil {
				return err
			}

			body := daptinClient.JsonApiObject{
				"data": map[string]interface{}{
					"type":       entityName,
					"attributes": attrs,
				},
			}

			result, err := appCtx.Client.Create(entityName, body)
			if err != nil {
				return err
			}

			data, _ := result["attributes"].(map[string]interface{})
			if data == nil {
				data = result
			}
			if appCtx.Quiet {
				return printRef(data)
			}
			return appCtx.Renderer.RenderObject(data)
		},
	}
}

func updateCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update a row",
		ArgsUsage: "<entity> <reference_id> [key=val ...] or <entity> <reference_id> <json>",
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			referenceId := c.Args().Get(1)
			if entityName == "" || referenceId == "" {
				return fmt.Errorf("usage: update <entity> <reference_id> [key=val ...]")
			}

			attrs, err := parseAttributes(c.Args().Slice()[2:])
			if err != nil {
				return err
			}

			body := daptinClient.JsonApiObject{
				"data": map[string]interface{}{
					"type":       entityName,
					"id":         referenceId,
					"attributes": attrs,
				},
			}

			result, err := appCtx.Client.Update(entityName, referenceId, body)
			if err != nil {
				return err
			}

			data, _ := result["attributes"].(map[string]interface{})
			if data == nil {
				data = result
			}
			if appCtx.Quiet {
				return printRef(data)
			}
			return appCtx.Renderer.RenderObject(data)
		},
	}
}

func deleteCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a row",
		ArgsUsage: "<entity> <reference_id>",
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			referenceId := c.Args().Get(1)
			if entityName == "" || referenceId == "" {
				return fmt.Errorf("usage: delete <entity> <reference_id>")
			}

			err := appCtx.Client.Delete(entityName, referenceId)
			if err != nil {
				return err
			}
			fmt.Println("Deleted")
			return nil
		},
	}
}

func relatedCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "related",
		Usage:     "Get related rows via a relationship",
		ArgsUsage: "<entity> <reference_id> <relation>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "columns",
				Usage: "Comma-separated column names to show",
			},
		},
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			referenceId := c.Args().Get(1)
			relation := c.Args().Get(2)
			if entityName == "" || referenceId == "" || relation == "" {
				return fmt.Errorf("usage: related <entity> <reference_id> <relation>")
			}

			result, err := appCtx.Client.FindRelated(entityName, referenceId, relation, nil)
			if err != nil {
				return err
			}

			rows := client.MapArray(result, "attributes")
			if appCtx.Quiet {
				return printRefs(rows)
			}
			if cols := c.String("columns"); cols != "" {
				rows = render.FilterColumns(rows, strings.Split(cols, ","))
			}
			return appCtx.Renderer.RenderArray(rows)
		},
	}
}

// printRef outputs only the reference_id from a single row.
func printRef(row map[string]interface{}) error {
	if ref, ok := row["reference_id"].(string); ok {
		fmt.Println(ref)
	}
	return nil
}

// printRefs outputs one reference_id per line from a list of rows.
func printRefs(rows []map[string]interface{}) error {
	for _, row := range rows {
		if ref, ok := row["reference_id"].(string); ok {
			fmt.Println(ref)
		}
	}
	return nil
}

// parseAttributes parses [key=val ...] args or a single JSON string into a map.
func parseAttributes(args []string) (map[string]interface{}, error) {
	if len(args) == 0 {
		return map[string]interface{}{}, nil
	}

	// If the first arg looks like JSON, parse it
	first := args[0]
	if strings.HasPrefix(first, "{") {
		joined := strings.Join(args, " ")
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(joined), &result); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return result, nil
	}

	// Otherwise parse key=val pairs
	result := make(map[string]interface{}, len(args))
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid argument %q, expected key=value", arg)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}
