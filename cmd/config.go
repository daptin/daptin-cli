package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/urfave/cli/v2"
)

var configSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func configCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Inspect and update Daptin runtime configuration",
		Subcommands: []*cli.Command{
			{
				Name:      "get",
				Usage:     "Get a runtime configuration value",
				ArgsUsage: "<type> <key>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "show-secrets", Usage: "Show values for secret-bearing keys"},
				},
				Action: func(c *cli.Context) error {
					if c.Args().Len() != 2 {
						return configCommandError(c, fmt.Errorf("usage: config get <type> <key>"))
					}
					configType, key, err := configKeyArgs(c)
					if err != nil {
						return configCommandError(c, err)
					}
					value, err := appCtx.Client.GetConfigValue(configType, key)
					if err != nil {
						return configCommandError(c, err)
					}
					redacted := isSensitiveConfigKey(key) && !c.Bool("show-secrets")
					if redacted {
						value = "[REDACTED]"
					}
					return appCtx.Renderer.RenderObject(map[string]interface{}{
						"type":     configType,
						"key":      key,
						"value":    value,
						"redacted": redacted,
					})
				},
			},
			{
				Name:      "set",
				Usage:     "Set a runtime configuration value",
				ArgsUsage: "<type> <key> [value]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "value", Usage: "Value to set (supports values beginning with a dash)"},
				},
				Action: func(c *cli.Context) error {
					configType, key, err := configKeyArgs(c)
					if err != nil {
						return configCommandError(c, err)
					}
					flagValueSet := c.IsSet("value")
					if (flagValueSet && c.Args().Len() != 2) || (!flagValueSet && c.Args().Len() != 3) {
						return configCommandError(c, fmt.Errorf("usage: config set <type> <key> <value> or config set --value <value> <type> <key>"))
					}
					value := c.Args().Get(2)
					if flagValueSet {
						value = c.String("value")
					}
					if err := appCtx.Client.SetConfigValue(configType, key, value); err != nil {
						return configCommandError(c, err)
					}
					return appCtx.Renderer.RenderObject(map[string]interface{}{
						"type":    configType,
						"key":     key,
						"updated": true,
					})
				},
			},
		},
	}
}

func configKeyArgs(c *cli.Context) (string, string, error) {
	configType := c.Args().Get(0)
	key := c.Args().Get(1)
	if configType == "" || key == "" {
		return "", "", fmt.Errorf("config type and key are required")
	}
	if !configSegmentPattern.MatchString(configType) {
		return "", "", fmt.Errorf("invalid config type %q", configType)
	}
	if !configSegmentPattern.MatchString(key) {
		return "", "", fmt.Errorf("invalid config key %q", key)
	}
	return configType, key, nil
}

func isSensitiveConfigKey(key string) bool {
	lower := strings.ToLower(key)
	for _, marker := range []string{"secret", "password", "token", "private_key", "private-key", "private.key", "api_key", "apikey", "credential"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func configCommandError(c *cli.Context, err error) error {
	if c.String("output") != "json" {
		return err
	}
	payload := map[string]interface{}{"message": err.Error()}
	var apiErr *client.ConfigAPIError
	if errors.As(err, &apiErr) {
		payload = map[string]interface{}{
			"operation": apiErr.Operation,
			"type":      apiErr.ConfigType,
			"key":       apiErr.Key,
			"status":    apiErr.StatusCode,
			"message":   apiErr.Message,
		}
	}
	_ = json.NewEncoder(os.Stderr).Encode(map[string]interface{}{"error": payload})
	return cli.Exit("", 1)
}
