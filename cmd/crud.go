package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/render"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
)

// maxCRUDPayloadBytes bounds file and stdin payloads before JSON decoding.
const maxCRUDPayloadBytes int64 = 16 << 20

func listCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List rows of an entity",
		ArgsUsage: "<entity>",
		UsageText: `daptin list <entity> [flags]
   daptin list usergroup --filter name=administrators --columns name,reference_id
   daptin list world --filter "table_name like %doc%" --page-size 50
   daptin list document --sort -created_at`,
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
				Usage: "Filter expression, e.g. name=value or \"name is value\"",
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
			slog.Info("list", "entity", entityName, "page", c.Int("page"), "page_size", c.Int("page-size"))

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

			rows := client.MapArray(result, "attributes")
			slog.Debug("list results", "count", len(rows))
			if appCtx.Quiet {
				return printRefs(rows)
			}
			if cols := c.String("columns"); cols != "" {
				colList := strings.Split(cols, ",")
				slog.Debug("filtering columns", "columns", colList)
				rows = render.FilterColumns(rows, colList)
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
			slog.Info("get", "entity", entityName, "reference_id", referenceId)

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
		UsageText: `daptin create <entity> [key=val ...] [flags]
   daptin create document '{"document_name":"report.pdf"}'
   daptin create asset --input-file ./asset.json
   cat asset.json | daptin create asset --input-stdin`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "input-file", Usage: "Read a JSON payload from a file (maximum 16 MiB)"},
			&cli.BoolFlag{Name: "input-stdin", Usage: "Read a JSON payload from stdin (maximum 16 MiB)"},
		},
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			if entityName == "" {
				return fmt.Errorf("entity name required")
			}
			slog.Info("create", "entity", entityName)

			attrs, err := readCRUDPayload(c.Args().Slice()[1:], c.String("input-file"), c.Bool("input-stdin"), os.Stdin)
			if err != nil {
				return err
			}
			body, err := createBodyFromAttributes(entityName, attrs)
			if err != nil {
				return err
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

func parseCreateBody(entityName string, args []string) (daptinClient.JsonApiObject, error) {
	attrs, err := parseAttributes(args)
	if err != nil {
		return nil, err
	}
	return createBodyFromAttributes(entityName, attrs)
}

func createBodyFromAttributes(entityName string, attrs map[string]interface{}) (daptinClient.JsonApiObject, error) {
	if data, ok := attrs["data"].(map[string]interface{}); ok {
		payloadType, _ := data["type"].(string)
		if payloadType == "" {
			return nil, fmt.Errorf("JSON:API create payload data.type is required")
		}
		if payloadType != entityName {
			return nil, fmt.Errorf("JSON:API create payload type %q does not match entity %q", payloadType, entityName)
		}
		return daptinClient.JsonApiObject(attrs), nil
	}
	return daptinClient.JsonApiObject{
		"data": map[string]interface{}{
			"type":       entityName,
			"attributes": attrs,
		},
	}, nil
}

func updateCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update a row",
		ArgsUsage: "<entity> <reference_id> [key=val ...] or <entity> <reference_id> <json>",
		UsageText: `daptin update <entity> <reference_id> [key=val ...] [flags]
   daptin update document <reference_id> '{"document_name":"updated.pdf"}'
   daptin update document <reference_id> --input-file ./attributes.json
   cat attributes.json | daptin update document <reference_id> --input-stdin`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "input-file", Usage: "Read a JSON attributes object from a file (maximum 16 MiB)"},
			&cli.BoolFlag{Name: "input-stdin", Usage: "Read a JSON attributes object from stdin (maximum 16 MiB)"},
		},
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			referenceId := c.Args().Get(1)
			if entityName == "" || referenceId == "" {
				return fmt.Errorf("usage: update <entity> <reference_id> [key=val ...]")
			}
			slog.Info("update", "entity", entityName, "reference_id", referenceId)

			attrs, err := readCRUDPayload(c.Args().Slice()[2:], c.String("input-file"), c.Bool("input-stdin"), os.Stdin)
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

func readCRUDPayload(args []string, inputFile string, inputStdin bool, stdin io.Reader) (map[string]interface{}, error) {
	sources := 0
	if len(args) > 0 {
		sources++
	}
	if inputFile != "" {
		sources++
	}
	if inputStdin {
		sources++
	}
	if sources > 1 {
		return nil, fmt.Errorf("provide only one of positional payload, --input-file, or --input-stdin")
	}

	if inputFile != "" {
		file, err := os.Open(inputFile)
		if err != nil {
			return nil, fmt.Errorf("open input file: %w", err)
		}
		defer file.Close()
		return readBoundedJSONObject(file)
	}
	if inputStdin {
		return readBoundedJSONObject(stdin)
	}
	return parseAttributes(args)
}

func readBoundedJSONObject(reader io.Reader) (map[string]interface{}, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxCRUDPayloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JSON payload: %w", err)
	}
	if int64(len(data)) > maxCRUDPayloadBytes {
		return nil, fmt.Errorf("JSON payload exceeds %d-byte limit", maxCRUDPayloadBytes)
	}

	var decoded interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("invalid JSON payload")
	}
	result, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("JSON payload must be an object")
	}
	return result, nil
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
			slog.Info("delete", "entity", entityName, "reference_id", referenceId)

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
			&cli.IntFlag{Name: "page-size", Usage: "Number of related rows per page", Value: 100},
			&cli.IntFlag{Name: "page", Usage: "Page number", Value: 1},
			&cli.BoolFlag{Name: "all", Usage: "Fetch all related rows using bounded pagination"},
			&cli.IntFlag{Name: "max-pages", Usage: "Maximum pages allowed with --all", Value: 100},
		},
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			referenceId := c.Args().Get(1)
			relation := c.Args().Get(2)
			if entityName == "" || referenceId == "" || relation == "" {
				return fmt.Errorf("usage: related <entity> <reference_id> <relation>")
			}
			slog.Info("related", "entity", entityName, "reference_id", referenceId, "relation", relation)

			pageSize := c.Int("page-size")
			if pageSize <= 0 {
				return fmt.Errorf("--page-size must be greater than zero")
			}
			var result []daptinClient.JsonApiObject
			var err error
			if c.Bool("all") {
				result, err = findAllRelated(appCtx, entityName, referenceId, relation, pageSize, c.Int("max-pages"))
			} else {
				pageNumber := c.Int("page")
				if pageNumber <= 0 {
					return fmt.Errorf("--page must be greater than zero")
				}
				result, err = appCtx.Client.FindRelated(entityName, referenceId, relation, daptinClient.DaptinQueryParameters{
					"page[size]": pageSize, "page[number]": pageNumber,
				})
			}
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

func findAllRelated(appCtx *AppContext, entityName, referenceId, relation string, pageSize, maxPages int) ([]daptinClient.JsonApiObject, error) {
	if maxPages <= 0 {
		return nil, fmt.Errorf("--max-pages must be greater than zero")
	}
	var result []daptinClient.JsonApiObject
	seen := make(map[string]bool)
	for pageNumber := 1; pageNumber <= maxPages+1; pageNumber++ {
		page, err := appCtx.Client.FindRelatedPage(entityName, referenceId, relation, daptinClient.DaptinQueryParameters{
			"page[size]": pageSize, "page[number]": pageNumber,
		})
		if err != nil {
			return nil, err
		}
		if pageNumber > maxPages {
			if len(page.Rows) > 0 {
				return nil, fmt.Errorf("related results exceed --max-pages=%d; no rows were printed", maxPages)
			}
			return result, nil
		}
		for _, row := range page.Rows {
			attrs, _ := row["attributes"].(map[string]interface{})
			identity, _ := attrs["reference_id"].(string)
			if identity == "" {
				identity, _ = row["id"].(string)
			}
			if identity == "" {
				return nil, fmt.Errorf("malformed related page %d: row has no reference_id", pageNumber)
			}
			if seen[identity] {
				return nil, fmt.Errorf("malformed related pagination: duplicate reference_id %q on page %d", identity, pageNumber)
			}
			seen[identity] = true
			result = append(result, row)
		}
		if len(page.Rows) < pageSize {
			return result, nil
		}
	}
	return nil, fmt.Errorf("related pagination did not terminate")
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
