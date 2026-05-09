package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/render"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
)

func integrationCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "integration",
		Usage: "Import, install, inspect, and execute OpenAPI integrations",
		UsageText: `daptin integration <command> [options]
   daptin integration import --provider asana.com --spec-file ./asana_oas.yaml --auth oauth2 --oauth-connect asana.com
   daptin integration install asana.com
   daptin integration operations asana.com
   daptin integration describe asana.com getWorkspaces
   daptin integration execute asana.com getWorkspaces --oauth-token-id <token_ref> --input-json '{"opt_fields":["name"]}'`,
		Description: "Wraps Daptin integration rows, install_integration, and provider-scoped /integration/<provider>/<operation> execution.",
		Subcommands: []*cli.Command{
			integrationImportCommand(appCtx),
			integrationInstallCommand(appCtx),
			integrationListCommand(appCtx),
			integrationOperationsCommand(appCtx),
			integrationDescribeCommand(appCtx),
			integrationExecuteCommand(appCtx),
		},
	}
}

func integrationImportCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "import",
		Usage:     "Create or update an integration from an OpenAPI spec",
		ArgsUsage: "[provider]",
		UsageText: `daptin integration import --provider <provider> [flags]
   daptin integration import --provider asana.com --spec-file ./asana_oas.yaml --auth oauth2 --oauth-connect asana.com
   daptin integration import --provider example.com --spec-url https://example.com/openapi.yaml --auth custom_credentials --auth-spec-file ./auth.json
   curl -L https://example.com/openapi.yaml | daptin integration import --provider example.com --spec-stdin --auth custom_credentials --auth-spec-json '{"name":"X-API-Key","in":"header","value_field":"api_key"}'`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "provider", Usage: "Provider name used as integration.name and provider-scoped route key"},
			&cli.StringFlag{Name: "spec-file", Usage: "Read OpenAPI spec from file"},
			&cli.StringFlag{Name: "spec-url", Usage: "Read OpenAPI spec from URL"},
			&cli.BoolFlag{Name: "spec-stdin", Usage: "Read OpenAPI spec from stdin"},
			&cli.StringFlag{Name: "spec-format", Usage: "Spec format: json or yaml (auto-detected when omitted)"},
			&cli.StringFlag{Name: "spec-language", Usage: "Spec language: openapiv2 or openapiv3 (auto-detected when omitted)"},
			&cli.StringFlag{Name: "auth", Usage: "Authentication type: oauth2 or custom_credentials"},
			&cli.StringFlag{Name: "oauth-connect", Usage: "oauth_connect name or reference_id for oauth2 integrations"},
			&cli.StringFlag{Name: "auth-spec-json", Usage: "Raw authentication_specification JSON for custom_credentials"},
			&cli.StringFlag{Name: "auth-spec-file", Usage: "Read authentication_specification JSON/YAML from file"},
			&cli.BoolFlag{Name: "disable", Usage: "Create/import integration with enable=false"},
			&cli.BoolFlag{Name: "update", Usage: "Update an existing integration with the same provider name"},
		},
		Action: func(c *cli.Context) error {
			provider := firstNonEmpty(c.String("provider"), c.Args().Get(0))
			if provider == "" {
				return fmt.Errorf("usage: integration import --provider <provider>")
			}
			specContent, err := readSpecInput(c)
			if err != nil {
				return err
			}
			specFormat, err := detectSpecFormat(specContent, c.String("spec-format"))
			if err != nil {
				return err
			}
			specLanguage, err := detectSpecLanguage(specContent, c.String("spec-language"))
			if err != nil {
				return err
			}
			authType, err := normalizeIntegrationAuthType(c.String("auth"))
			if err != nil {
				return err
			}
			if authType == "" {
				return fmt.Errorf("--auth is required (oauth2 or custom_credentials)")
			}
			authSpec, err := buildIntegrationAuthSpec(appCtx, authType, c.String("oauth-connect"), c.String("auth-spec-json"), c.String("auth-spec-file"))
			if err != nil {
				return err
			}

			attrs := map[string]interface{}{
				"name":                         provider,
				"specification_language":       specLanguage,
				"specification_format":         specFormat,
				"specification":                specContent,
				"authentication_type":          authType,
				"authentication_specification": authSpec,
				"enable":                       !c.Bool("disable"),
			}

			if c.Bool("update") {
				existing, err := findOneByName(appCtx, "integration", provider)
				if err != nil {
					created, createErr := appCtx.Client.Create("integration", jsonAPIObject("integration", attrs, ""))
					if createErr != nil {
						return createErr
					}
					return renderSingleAPIObject(appCtx, created)
				}
				ref := refID(existing)
				if ref == "" {
					return fmt.Errorf("integration %q has no reference_id", provider)
				}
				updated, err := appCtx.Client.Update("integration", ref, jsonAPIObject("integration", attrs, ref))
				if err != nil {
					return err
				}
				return renderSingleAPIObject(appCtx, updated)
			}

			created, err := appCtx.Client.Create("integration", jsonAPIObject("integration", attrs, ""))
			if err != nil {
				return err
			}
			return renderSingleAPIObject(appCtx, created)
		},
	}
}

func integrationInstallCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "install",
		Usage:     "Install an integration and register its operations",
		ArgsUsage: "<provider-or-reference-id>",
		UsageText: `daptin integration install <provider-or-reference-id>
   daptin integration install asana.com`,
		Action: func(c *cli.Context) error {
			ref, err := integrationRef(appCtx, c.Args().Get(0))
			if err != nil {
				return err
			}
			responses, err := appCtx.Client.Execute("install_integration", "integration", daptinClient.JsonApiObject{
				"integration_id": ref,
			})
			if err != nil {
				return err
			}
			effects := ProcessResponses(responses)
			if len(effects) == 0 {
				effects = append(effects, BuildActionSuccessEffect("integration", "install_integration", ref))
			}
			return applyEffects(effects, appCtx)
		},
	}
}

func integrationListCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List integration rows without large spec bodies",
		Action: func(c *cli.Context) error {
			result, err := appCtx.Client.FindAll("integration", daptinClient.DaptinQueryParameters{"page[size]": 100})
			if err != nil {
				return err
			}
			rows := client.MapArray(result, "attributes")
			rows = render.FilterColumns(rows, []string{"name", "authentication_type", "enable", "reference_id"})
			if appCtx.Quiet {
				return printRefs(rows)
			}
			return appCtx.Renderer.RenderArray(rows)
		},
	}
}

func integrationOperationsCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "operations",
		Usage:     "List operations for an installed integration provider",
		ArgsUsage: "<provider-or-reference-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "columns", Usage: "Comma-separated columns to show"},
		},
		Action: func(c *cli.Context) error {
			provider, err := integrationProviderName(appCtx, c.Args().Get(0))
			if err != nil {
				return err
			}
			document, err := appCtx.Client.IntegrationOperations(provider)
			if err != nil {
				return err
			}
			ops := operationRowsFromDiscovery(document)
			if appCtx.Quiet {
				for _, op := range ops {
					fmt.Println(op["operation_id"])
				}
				return nil
			}
			if cols := c.String("columns"); cols != "" {
				ops = render.FilterColumns(ops, strings.Split(cols, ","))
			}
			return appCtx.Renderer.RenderArray(ops)
		},
	}
}

func integrationDescribeCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "describe",
		Usage:     "Show operation method, path, input params, and response hints",
		ArgsUsage: "<provider-or-reference-id> <operation>",
		UsageText: `daptin integration describe <provider-or-reference-id> <operation>
   daptin integration describe asana.com getWorkspaces`,
		Action: func(c *cli.Context) error {
			nameOrRef := c.Args().Get(0)
			operationID := c.Args().Get(1)
			if nameOrRef == "" || operationID == "" {
				return fmt.Errorf("usage: integration describe <provider-or-reference-id> <operation>")
			}
			provider, err := integrationProviderName(appCtx, nameOrRef)
			if err != nil {
				return err
			}
			detail, err := appCtx.Client.IntegrationOperationDescription(provider, operationID)
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(detail)
		},
	}
}

func integrationExecuteCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "execute",
		Usage:     "Execute a provider-scoped integration operation",
		ArgsUsage: "<provider> <operation> [key=val ...]",
		UsageText: `daptin integration execute <provider> <operation> [key=val ...] [flags]
   daptin integration execute asana.com getWorkspaces --oauth-token-id <token_ref> --input-json '{"opt_fields":["name"]}'
   daptin integration execute example.com listUsers --credential-id <credential_ref> limit=10
   daptin integration execute example.com createUser --input-file ./input.json`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "oauth-token-id", Usage: "oauth_token reference_id to use for OAuth2 integrations"},
			&cli.StringFlag{Name: "credential-id", Usage: "credential reference_id to use for custom credential integrations"},
			&cli.StringFlag{Name: "input-json", Usage: "Operation input as JSON object"},
			&cli.StringFlag{Name: "input-file", Usage: "Read operation input JSON/YAML object from file"},
		},
		Action: func(c *cli.Context) error {
			provider := c.Args().Get(0)
			operation := c.Args().Get(1)
			if provider == "" || operation == "" {
				return fmt.Errorf("usage: integration execute <provider> <operation> [key=val ...]")
			}
			input, err := buildOperationInput(c.String("input-json"), c.String("input-file"), c.Args().Slice()[2:])
			if err != nil {
				return err
			}
			body := buildIntegrationOperationBody(input, c.String("oauth-token-id"), c.String("credential-id"))
			result, err := appCtx.Client.ExecuteIntegrationOperation(provider, operation, body)
			if err != nil {
				return err
			}
			return renderIntegrationOperationResult(appCtx, result)
		},
	}
}

func readSpecInput(c *cli.Context) (string, error) {
	sources := 0
	if c.String("spec-file") != "" {
		sources++
	}
	if c.String("spec-url") != "" {
		sources++
	}
	if c.Bool("spec-stdin") {
		sources++
	}
	if sources != 1 {
		return "", fmt.Errorf("provide exactly one of --spec-file, --spec-url, or --spec-stdin")
	}
	switch {
	case c.String("spec-file") != "":
		data, err := os.ReadFile(c.String("spec-file"))
		if err != nil {
			return "", err
		}
		return string(data), nil
	case c.String("spec-url") != "":
		resp, err := http.Get(c.String("spec-url"))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("fetch spec: HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

func detectSpecFormat(specContent, override string) (string, error) {
	if override != "" {
		switch override {
		case "json", "yaml":
			return override, nil
		default:
			return "", fmt.Errorf("--spec-format must be json or yaml")
		}
	}
	trimmed := strings.TrimSpace(specContent)
	if strings.HasPrefix(trimmed, "{") {
		return "json", nil
	}
	return "yaml", nil
}

func detectSpecLanguage(specContent, override string) (string, error) {
	if override != "" {
		switch override {
		case "openapiv2", "openapiv3":
			return override, nil
		default:
			return "", fmt.Errorf("--spec-language must be openapiv2 or openapiv3")
		}
	}
	doc, err := parseYAMLOrJSONMap(specContent)
	if err != nil {
		return "", fmt.Errorf("detect spec language: %w", err)
	}
	if _, ok := doc["openapi"]; ok {
		return "openapiv3", nil
	}
	if _, ok := doc["swagger"]; ok {
		return "openapiv2", nil
	}
	return "", fmt.Errorf("could not detect OpenAPI language; pass --spec-language openapiv2 or openapiv3")
}

func buildIntegrationAuthSpec(appCtx *AppContext, authType, oauthConnect, authSpecJSON, authSpecFile string) (string, error) {
	switch authType {
	case "oauth2":
		if oauthConnect == "" {
			return "", fmt.Errorf("--oauth-connect is required when --auth oauth2")
		}
		ref, err := oauthConnectRef(appCtx, oauthConnect)
		if err != nil {
			return "", err
		}
		return marshalJSONString(map[string]interface{}{"oauth_connect_id": ref})
	case "custom_credentials":
		if authSpecJSON == "" && authSpecFile == "" {
			return "", fmt.Errorf("--auth-spec-json or --auth-spec-file is required when --auth custom_credentials")
		}
		if authSpecJSON != "" && authSpecFile != "" {
			return "", fmt.Errorf("provide only one of --auth-spec-json or --auth-spec-file")
		}
		content := authSpecJSON
		if authSpecFile != "" {
			data, err := os.ReadFile(authSpecFile)
			if err != nil {
				return "", err
			}
			content = string(data)
		}
		authSpec, err := parseYAMLOrJSONMap(content)
		if err != nil {
			return "", fmt.Errorf("parse auth spec: %w", err)
		}
		return marshalJSONString(authSpec)
	default:
		return "", fmt.Errorf("--auth must be oauth2 or custom_credentials")
	}
}

func normalizeIntegrationAuthType(authType string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(authType)) {
	case "":
		return "", nil
	case "oauth2":
		return "oauth2", nil
	case "custom", "custom_credentials":
		return "custom_credentials", nil
	default:
		return "", fmt.Errorf("--auth must be oauth2 or custom_credentials")
	}
}

func buildOperationInput(inputJSON, inputFile string, keyVals []string) (map[string]interface{}, error) {
	provided := 0
	if inputJSON != "" {
		provided++
	}
	if inputFile != "" {
		provided++
	}
	if provided > 1 {
		return nil, fmt.Errorf("provide only one of --input-json or --input-file")
	}

	input := map[string]interface{}{}
	if inputJSON != "" {
		parsed, err := parseYAMLOrJSONMap(inputJSON)
		if err != nil {
			return nil, fmt.Errorf("parse --input-json: %w", err)
		}
		input = parsed
	}
	if inputFile != "" {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return nil, err
		}
		parsed, err := parseYAMLOrJSONMap(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse --input-file: %w", err)
		}
		input = parsed
	}
	attrs, err := parseAttributes(keyVals)
	if err != nil {
		return nil, err
	}
	for k, v := range attrs {
		input[k] = v
	}
	return input, nil
}

func buildIntegrationOperationBody(input map[string]interface{}, oauthTokenID, credentialID string) map[string]interface{} {
	body := map[string]interface{}{"input": input}
	if oauthTokenID != "" {
		body["oauth_token_id"] = oauthTokenID
	}
	if credentialID != "" {
		body["credential_id"] = credentialID
	}
	return body
}

func integrationRef(appCtx *AppContext, nameOrRef string) (string, error) {
	row, err := integrationRow(appCtx, nameOrRef)
	if err != nil {
		return "", err
	}
	if ref := refID(row); ref != "" {
		return ref, nil
	}
	if attrs, ok := row["attributes"].(map[string]interface{}); ok {
		if ref, ok := attrs["reference_id"].(string); ok {
			return ref, nil
		}
	}
	return "", fmt.Errorf("integration %q has no reference_id", nameOrRef)
}

func integrationProviderName(appCtx *AppContext, nameOrRef string) (string, error) {
	row, err := integrationRow(appCtx, nameOrRef)
	if err != nil {
		return "", err
	}
	if attrs, ok := row["attributes"].(map[string]interface{}); ok {
		if name, ok := attrs["name"].(string); ok && name != "" {
			return name, nil
		}
	}
	if name, ok := row["name"].(string); ok && name != "" {
		return name, nil
	}
	if !strings.Contains(nameOrRef, "-") {
		return nameOrRef, nil
	}
	return "", fmt.Errorf("integration %q has no name", nameOrRef)
}

func integrationRow(appCtx *AppContext, nameOrRef string) (map[string]interface{}, error) {
	if nameOrRef == "" {
		return nil, fmt.Errorf("integration provider name or reference_id required")
	}
	if strings.Contains(nameOrRef, "-") {
		row, err := appCtx.Client.FindOne("integration", nameOrRef, nil)
		if err == nil {
			return row, nil
		}
	}
	return findOneByName(appCtx, "integration", nameOrRef)
}

func renderIntegrationOperationResult(appCtx *AppContext, result interface{}) error {
	switch typed := result.(type) {
	case map[string]interface{}:
		return appCtx.Renderer.RenderObject(typed)
	case []interface{}:
		rows := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]interface{}); ok {
				rows = append(rows, row)
			}
		}
		return appCtx.Renderer.RenderArray(rows)
	default:
		return appCtx.Renderer.RenderObject(map[string]interface{}{"response": typed})
	}
}

func operationRowsFromDiscovery(document map[string]interface{}) []map[string]interface{} {
	rawOperations, ok := document["operations"].([]interface{})
	if !ok {
		return nil
	}
	rows := make([]map[string]interface{}, 0, len(rawOperations))
	for _, raw := range rawOperations {
		operation, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		rows = append(rows, operation)
	}
	return rows
}

func parseYAMLOrJSONMap(content string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &result); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("expected object")
	}
	return result, nil
}

func marshalJSONString(value map[string]interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func renderSingleAPIObject(appCtx *AppContext, obj map[string]interface{}) error {
	data, _ := obj["attributes"].(map[string]interface{})
	if data == nil {
		data = obj
	}
	data = redactSecretColumns(data)
	if appCtx.Quiet {
		return printRef(data)
	}
	return appCtx.Renderer.RenderObject(data)
}

func redactSecretColumns(row map[string]interface{}) map[string]interface{} {
	return render.ExcludeColumns(row, []string{
		"client_secret",
		"access_token",
		"refresh_token",
		"content",
		"specification",
		"authentication_specification",
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
