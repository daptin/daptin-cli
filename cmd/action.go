package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"syscall"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/render"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func executeCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "execute",
		Usage:     "Execute a Daptin action on an entity",
		ArgsUsage: "<entity> <action_name> [key=val ...]",
		UsageText: `daptin execute <entity> <action_name> [key=val ...]
   daptin execute user_account signin email=admin@example.com password=secret
   daptin execute world my_action --input-json '{"key":"value"}'
   daptin execute world my_action --input-file request.json
   cat request.json | daptin execute world my_action --input-stdin
   daptin execute integration install_integration --reference-id <integration_reference_id>
   daptin execute user_account signin --interactive`,
		Description: "Use describe action first when you need to know whether an action requires --reference-id or which fields it accepts.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "input-json",
				Usage: "Action attributes as a JSON object (maximum 16 MiB)",
			},
			&cli.StringFlag{
				Name:  "input-file",
				Usage: "Read action attributes as a JSON object from `FILE` (maximum 16 MiB)",
			},
			&cli.BoolFlag{
				Name:  "input-stdin",
				Usage: "Read action attributes as a JSON object from stdin (maximum 16 MiB)",
			},
			&cli.StringFlag{
				Name:  "reference-id",
				Usage: "Entity instance reference_id (for non-optional instance actions)",
			},
			&cli.BoolFlag{
				Name:  "interactive",
				Usage: "Prompt for missing fields based on action schema",
			},
		},
		Action: func(c *cli.Context) error {
			entityName := c.Args().Get(0)
			actionName := c.Args().Get(1)
			if entityName == "" || actionName == "" {
				return fmt.Errorf("usage: execute <entity> <action_name> [key=val ...]")
			}
			slog.Info("execute", "entity", entityName, "action", actionName)

			attrs, err := readActionPayload(c.Args().Slice()[2:], actionInputOptions{
				inputJSON:    c.String("input-json"),
				inputJSONSet: c.IsSet("input-json"),
				inputFile:    c.String("input-file"),
				inputFileSet: c.IsSet("input-file"),
				inputStdin:   c.Bool("input-stdin"),
				interactive:  c.Bool("interactive"),
			}, os.Stdin)
			slog.Debug("execute attributes parsed", "count", len(c.Args().Slice()[2:]))
			if err != nil {
				return err
			}

			var schema ActionSchema
			schemaLoaded := false
			loadSchema := func() (ActionSchema, error) {
				if schemaLoaded {
					return schema, nil
				}
				loaded, err := fetchActionSchemaFromServer(appCtx, entityName, actionName)
				if err != nil {
					return ActionSchema{}, err
				}
				schema = loaded
				schemaLoaded = true
				return schema, nil
			}

			// Interactive: fetch schema, compute missing fields, prompt at IO boundary
			if c.Bool("interactive") {
				slog.Debug("interactive mode enabled, fetching schema")
				schema, schemaErr := loadSchema()
				if schemaErr == nil {
					prompts := MissingFields(schema.InFields, attrs)
					filled, err := promptUser(prompts)
					if err != nil {
						return err
					}
					for k, v := range filled {
						attrs[k] = v
					}
				}
			}

			if hasActionFileReference(attrs) {
				schema, err := loadSchema()
				if err != nil {
					return fmt.Errorf("inspect action schema for file fields: %w", err)
				}
				if err := materializeActionFileFields(schema.InFields, attrs); err != nil {
					return err
				}
			}

			if refId := c.String("reference-id"); refId != "" {
				attrs[entityName+"_id"] = refId
			}

			responses, err := appCtx.Client.Execute(actionName, entityName, attrs)
			if err != nil {
				return err
			}
			if err := actionResponseError(responses); err != nil {
				return err
			}

			// Pure: compute effects from responses
			effects := ProcessResponses(responses)
			if len(effects) == 0 {
				effects = append(effects, BuildActionSuccessEffect(entityName, actionName, c.String("reference-id")))
			}

			// IO boundary: apply effects
			return applyEffects(effects, appCtx)
		},
	}
}

type actionInputOptions struct {
	inputJSON    string
	inputJSONSet bool
	inputFile    string
	inputFileSet bool
	inputStdin   bool
	interactive  bool
}

// readActionPayload selects and decodes one action input source. Validation is
// completed before action schema discovery or execution can make an HTTP call.
func readActionPayload(args []string, options actionInputOptions, stdin io.Reader) (map[string]interface{}, error) {
	sources := 0
	if len(args) > 0 {
		sources++
	}
	if options.inputJSONSet {
		sources++
	}
	if options.inputFileSet {
		sources++
	}
	if options.inputStdin {
		sources++
	}
	if sources > 1 {
		return nil, fmt.Errorf("provide only one of positional payload, --input-json, --input-file, or --input-stdin")
	}
	if options.inputStdin && options.interactive {
		return nil, fmt.Errorf("--input-stdin cannot be combined with --interactive")
	}

	if options.inputJSONSet {
		return readBoundedJSONObject(strings.NewReader(options.inputJSON))
	}
	if options.inputFileSet {
		file, err := os.Open(options.inputFile)
		if err != nil {
			return nil, fmt.Errorf("open input file: %w", err)
		}
		defer file.Close()
		return readBoundedJSONObject(file)
	}
	if options.inputStdin {
		return readBoundedJSONObject(stdin)
	}
	return parseAttributes(args)
}

// applyEffects performs the IO for each ResponseEffect. This is the edge.
func applyEffects(effects []ResponseEffect, appCtx *AppContext) error {
	for _, e := range effects {
		slog.Debug("applying effect", "type", e.Type)
		switch e.Type {
		case "token":
			host, err := appCtx.Config.ActiveHost()
			if err == nil {
				host.Token = e.Token
				appCtx.Config.UpsertHost(host)
				appCtx.Config.CurrentContext = host.Name
				_ = appCtx.Config.Save()
				fmt.Fprintln(os.Stderr, "Authenticated successfully")
			}
		case "notify":
			fmt.Fprintf(os.Stderr, "Notice: %s\n", e.Message)
		case "redirect":
			fmt.Fprintf(os.Stderr, "Redirect: %s\n", e.Message)
		case "file_download":
			if err := appCtx.Renderer.RenderObject(e.Data); err != nil {
				return err
			}
		case "render_object":
			if err := appCtx.Renderer.RenderObject(e.Data); err != nil {
				return err
			}
		case "success":
			if appCtx.Quiet {
				continue
			}
			if _, ok := appCtx.Renderer.(*render.JsonRenderer); ok {
				if err := appCtx.Renderer.RenderObject(e.Data); err != nil {
					return err
				}
				continue
			}
			fmt.Fprintln(os.Stdout, e.Message)
		}
	}
	return nil
}

// fetchActionSchemaFromServer fetches InFields for an action via the API.
// IO boundary: makes HTTP calls, then delegates to pure functions.
func fetchActionSchemaFromServer(appCtx *AppContext, entityName, actionName string) (ActionSchema, error) {
	slog.Debug("fetching action schema", "entity", entityName, "action", actionName)
	worlds, err := appCtx.Client.FindAll("world", daptinClient.DaptinQueryParameters{
		"page[size]": 500,
	})
	if err != nil {
		return ActionSchema{}, err
	}

	worldAttrs := client.MapArray(worlds, "attributes")
	worldRefId := FindWorldRefId(worldAttrs, entityName)
	if worldRefId == "" {
		return ActionSchema{}, fmt.Errorf("entity %q not found", entityName)
	}

	actions, err := appCtx.Client.FindAll("action", daptinClient.DaptinQueryParameters{
		"page[size]": 500,
	})
	if err != nil {
		return ActionSchema{}, err
	}

	actionAttrs := client.MapArray(actions, "attributes")
	schema := FindActionMetadata(actionAttrs, worldRefId, entityName, actionName)
	if schema.ReferenceID == "" {
		return ActionSchema{}, fmt.Errorf("action %q not found on %q", actionName, entityName)
	}

	// Execute get_action_schema to retrieve the schema (base64 encoded)
	responses, err := appCtx.Client.Execute("get_action_schema", "action", daptinClient.JsonApiObject{
		"action_id": schema.ReferenceID,
	})
	if err != nil {
		return ActionSchema{}, err
	}

	inFields, err := DecodeActionSchemaResponse(responses)
	if err != nil {
		return ActionSchema{}, err
	}
	schema.InFields = inFields
	return schema, nil
}

// promptUser performs IO to collect values for missing fields.
// Takes pure FieldPrompt values, returns collected values.
func promptUser(prompts []FieldPrompt) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	reader := bufio.NewReader(os.Stdin)

	for _, p := range prompts {
		label := p.Label
		if p.IsNullable {
			label = label + " (optional)"
		}

		if p.ColumnType == "password" {
			fmt.Fprintf(os.Stderr, "%s: ", label)
			pw, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return nil, err
			}
			val := strings.TrimSpace(string(pw))
			if val != "" || !p.IsNullable {
				result[p.ColumnName] = val
			}
		} else {
			fmt.Fprintf(os.Stderr, "%s: ", label)
			val, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			val = strings.TrimSpace(val)
			if val != "" || !p.IsNullable {
				result[p.ColumnName] = val
			}
		}
	}
	return result, nil
}
