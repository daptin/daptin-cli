package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/render"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
)

func oauthCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "oauth",
		Usage: "Manage OAuth connections and discover user tokens",
		UsageText: `daptin oauth <command> [options]
   daptin oauth connect create asana.com --client-id <id> --client-secret-env ASANA_CLIENT_SECRET --auth-url <url> --token-url <url> --profile-url <url> --scope default
   daptin oauth connect list
   daptin oauth login-url asana.com
   daptin oauth tokens list --provider asana.com`,
		Description: "Wraps Daptin's oauth_connect and oauth_token tables without printing client secrets or tokens by default.",
		Subcommands: []*cli.Command{
			oauthConnectCommand(appCtx),
			oauthLoginURLCommand(appCtx),
			oauthTokensCommand(appCtx),
		},
	}
}

func oauthConnectCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "connect",
		Usage: "Manage oauth_connect provider rows",
		Subcommands: []*cli.Command{
			oauthConnectCreateCommand(appCtx),
			oauthConnectListCommand(appCtx),
		},
	}
}

func oauthConnectCreateCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create an oauth_connect provider row",
		ArgsUsage: "<provider>",
		UsageText: `daptin oauth connect create <provider> [flags]
   daptin oauth connect create asana.com --client-id <id> --client-secret-env ASANA_CLIENT_SECRET --auth-url https://app.asana.com/-/oauth_authorize --token-url https://app.asana.com/-/oauth_token --profile-url https://app.asana.com/api/1.0/users/me --scope default`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "provider", Usage: "Provider name; same as positional <provider>"},
			&cli.StringFlag{Name: "client-id", Usage: "OAuth client id"},
			&cli.StringFlag{Name: "client-secret", Usage: "OAuth client secret (prefer --client-secret-env or --client-secret-file)"},
			&cli.StringFlag{Name: "client-secret-env", Usage: "Read OAuth client secret from environment variable"},
			&cli.StringFlag{Name: "client-secret-file", Usage: "Read OAuth client secret from file"},
			&cli.StringFlag{Name: "scope", Usage: "Comma-separated provider scopes"},
			&cli.StringFlag{Name: "response-type", Value: "code", Usage: "OAuth response type"},
			&cli.StringFlag{Name: "redirect-uri", Value: "/oauth/response", Usage: "OAuth callback URI"},
			&cli.StringFlag{Name: "auth-url", Usage: "Provider authorization URL"},
			&cli.StringFlag{Name: "token-url", Usage: "Provider token URL"},
			&cli.StringFlag{Name: "profile-url", Usage: "Provider profile URL"},
			&cli.StringFlag{Name: "profile-email-path", Value: "email", Usage: "JSON path to email in provider profile"},
			&cli.BoolFlag{Name: "allow-login", Usage: "Allow this provider for Daptin user login"},
			&cli.BoolFlag{Name: "access-type-offline", Usage: "Request refresh token/offline access when supported"},
			&cli.BoolFlag{Name: "pkce", Usage: "Enable PKCE for this provider"},
			&cli.StringFlag{Name: "pkce-challenge-method", Value: "S256", Usage: "PKCE challenge method"},
			&cli.BoolFlag{Name: "update", Usage: "Update existing oauth_connect row with the same provider name"},
		},
		Action: func(c *cli.Context) error {
			provider := firstNonEmpty(c.String("provider"), c.Args().Get(0))
			if provider == "" {
				return fmt.Errorf("usage: oauth connect create <provider>")
			}
			secret, err := oauthClientSecret(c.String("client-secret"), c.String("client-secret-env"), c.String("client-secret-file"))
			if err != nil {
				return err
			}
			required := map[string]string{
				"--client-id": c.String("client-id"),
				"--auth-url":  c.String("auth-url"),
				"--token-url": c.String("token-url"),
			}
			for flag, value := range required {
				if value == "" {
					return fmt.Errorf("%s is required", flag)
				}
			}
			attrs := map[string]interface{}{
				"name":                  provider,
				"client_id":             c.String("client-id"),
				"client_secret":         secret,
				"scope":                 c.String("scope"),
				"response_type":         c.String("response-type"),
				"redirect_uri":          c.String("redirect-uri"),
				"auth_url":              c.String("auth-url"),
				"token_url":             c.String("token-url"),
				"profile_url":           c.String("profile-url"),
				"profile_email_path":    c.String("profile-email-path"),
				"allow_login":           c.Bool("allow-login"),
				"access_type_offline":   c.Bool("access-type-offline"),
				"pkce_enabled":          c.Bool("pkce"),
				"pkce_challenge_method": c.String("pkce-challenge-method"),
			}

			if c.Bool("update") {
				existing, err := findOneByName(appCtx, "oauth_connect", provider)
				if err != nil {
					created, createErr := appCtx.Client.Create("oauth_connect", jsonAPIObject("oauth_connect", attrs, ""))
					if createErr != nil {
						return createErr
					}
					return renderSingleAPIObject(appCtx, created)
				}
				ref := refID(existing)
				if ref == "" {
					return fmt.Errorf("oauth_connect %q has no reference_id", provider)
				}
				updated, err := appCtx.Client.Update("oauth_connect", ref, jsonAPIObject("oauth_connect", attrs, ref))
				if err != nil {
					return err
				}
				return renderSingleAPIObject(appCtx, updated)
			}

			created, err := appCtx.Client.Create("oauth_connect", jsonAPIObject("oauth_connect", attrs, ""))
			if err != nil {
				return err
			}
			return renderSingleAPIObject(appCtx, created)
		},
	}
}

func oauthConnectListCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List OAuth provider connections without secrets",
		Action: func(c *cli.Context) error {
			result, err := appCtx.Client.FindAll("oauth_connect", daptinClient.DaptinQueryParameters{"page[size]": 100})
			if err != nil {
				return err
			}
			rows := client.MapArray(result, "attributes")
			rows = render.FilterColumns(rows, []string{"name", "client_id", "scope", "allow_login", "access_type_offline", "pkce_enabled", "reference_id"})
			if appCtx.Quiet {
				return printRefs(rows)
			}
			return appCtx.Renderer.RenderArray(rows)
		},
	}
}

func oauthLoginURLCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "login-url",
		Usage:     "Generate and print the provider authorization URL",
		ArgsUsage: "<provider-or-reference-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "open", Usage: "Open the authorization URL in the default browser"},
		},
		Action: func(c *cli.Context) error {
			ref, err := oauthConnectRef(appCtx, c.Args().Get(0))
			if err != nil {
				return err
			}
			responses, err := appCtx.Client.Execute("oauth_login_begin", "oauth_connect", daptinClient.JsonApiObject{
				"oauth_connect_id": ref,
			})
			if err != nil {
				return err
			}
			url := redirectURLFromResponses(responses)
			if url == "" {
				return applyEffects(ProcessResponses(responses), appCtx)
			}
			if c.Bool("open") {
				if err := openBrowser(url); err != nil {
					return err
				}
			}
			if appCtx.Quiet {
				fmt.Println(url)
				return nil
			}
			return appCtx.Renderer.RenderObject(map[string]interface{}{"authorization_url": url})
		},
	}
}

func oauthTokensCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "tokens",
		Usage: "Discover oauth_token rows without printing token secrets",
		Subcommands: []*cli.Command{
			oauthTokensListCommand(appCtx),
		},
	}
}

func oauthTokensListCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List OAuth token references",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "provider", Usage: "Filter by oauth_connect provider name or reference_id"},
		},
		Action: func(c *cli.Context) error {
			params := daptinClient.DaptinQueryParameters{"page[size]": 100}
			if provider := c.String("provider"); provider != "" {
				ref, err := oauthConnectRef(appCtx, provider)
				if err != nil {
					return err
				}
				clauses, err := ParseFilter("oauth_connect_id=" + ref)
				if err != nil {
					return err
				}
				params["query"] = FilterToJSON(clauses)
			}
			result, err := appCtx.Client.FindAll("oauth_token", params)
			if err != nil {
				return err
			}
			rows := client.MapArray(result, "attributes")
			rows = render.FilterColumns(rows, []string{"reference_id", "oauth_connect_id", "user_account_id", "token_type", "expires_in", "created_at", "updated_at"})
			if appCtx.Quiet {
				return printRefs(rows)
			}
			return appCtx.Renderer.RenderArray(rows)
		},
	}
}

func oauthConnectRef(appCtx *AppContext, nameOrRef string) (string, error) {
	if nameOrRef == "" {
		return "", fmt.Errorf("oauth_connect provider name or reference_id required")
	}
	if strings.Contains(nameOrRef, "-") {
		row, err := appCtx.Client.FindOne("oauth_connect", nameOrRef, nil)
		if err == nil {
			if ref := refID(row); ref != "" {
				return ref, nil
			}
			return nameOrRef, nil
		}
	}
	row, err := findOneByName(appCtx, "oauth_connect", nameOrRef)
	if err != nil {
		return "", err
	}
	if ref := refID(row); ref != "" {
		return ref, nil
	}
	return "", fmt.Errorf("oauth_connect %q has no reference_id", nameOrRef)
}

func oauthClientSecret(direct, envName, filePath string) (string, error) {
	sources := 0
	for _, value := range []string{direct, envName, filePath} {
		if value != "" {
			sources++
		}
	}
	if sources != 1 {
		return "", fmt.Errorf("provide exactly one of --client-secret, --client-secret-env, or --client-secret-file")
	}
	switch {
	case envName != "":
		value := os.Getenv(envName)
		if value == "" {
			return "", fmt.Errorf("environment variable %s is empty", envName)
		}
		return value, nil
	case filePath != "":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return direct, nil
	}
}

func redirectURLFromResponses(responses []daptinClient.DaptinActionResponse) string {
	for _, response := range responses {
		if response.ResponseType != "client.redirect" {
			continue
		}
		if location, ok := response.Attributes["location"].(string); ok {
			return location
		}
	}
	return ""
}

func openBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{url}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		command = "xdg-open"
		args = []string{url}
	}
	return exec.Command(command, args...).Start()
}
