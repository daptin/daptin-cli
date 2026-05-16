package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/daptin/daptin-cli/client"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
)

func tableCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "table",
		Usage: "Manage table-level metadata",
		Subcommands: []*cli.Command{
			tableDefaultsCommand(appCtx),
		},
	}
}

func tableDefaultsCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "defaults",
		Usage: "Inspect and manage DefaultPermission and DefaultGroups",
		Subcommands: []*cli.Command{
			tableDefaultsGetCommand(appCtx),
			tableDefaultsSetCommand(appCtx),
			tableDefaultsGroupCommand(appCtx),
			tableDefaultsEnsureCommand(appCtx),
		},
	}
}

func tableDefaultsGetCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Inspect defaults for a table",
		ArgsUsage: "<entity>",
		Action: func(c *cli.Context) error {
			defaults, err := loadTableDefaults(appCtx, c.Args().Get(0))
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(defaults.render(false))
		},
	}
}

func tableDefaultsSetCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     "Set default row permission for a table",
		ArgsUsage: "<entity>",
		Flags: []cli.Flag{
			&cli.Int64Flag{Name: "permission", Usage: "DefaultPermission value"},
		},
		Action: func(c *cli.Context) error {
			if !c.IsSet("permission") {
				return fmt.Errorf("--permission is required")
			}
			defaults, err := loadTableDefaults(appCtx, c.Args().Get(0))
			if err != nil {
				return err
			}
			changed := defaults.setPermission(c.Int64("permission"))
			if changed {
				if err := saveTableDefaults(appCtx, defaults); err != nil {
					return err
				}
			}
			return appCtx.Renderer.RenderObject(defaults.render(changed))
		},
	}
}

func tableDefaultsGroupCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "group",
		Usage: "Manage default group bindings",
		Subcommands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "Ensure a default group binding",
				ArgsUsage: "<entity> <group>",
				Flags: []cli.Flag{
					&cli.Int64Flag{Name: "permission", Usage: "Relation permission for this group"},
				},
				Action: func(c *cli.Context) error {
					entityName := c.Args().Get(0)
					groupName := c.Args().Get(1)
					if groupName == "" {
						return fmt.Errorf("usage: table defaults group add <entity> <group>")
					}
					defaults, err := loadTableDefaults(appCtx, entityName)
					if err != nil {
						return err
					}
					var permission *int64
					if c.IsSet("permission") {
						value := c.Int64("permission")
						permission = &value
					}
					changed := defaults.ensureGroup(groupName, permission)
					if changed {
						if err := saveTableDefaults(appCtx, defaults); err != nil {
							return err
						}
					}
					return appCtx.Renderer.RenderObject(defaults.render(changed))
				},
			},
		},
	}
}

func tableDefaultsEnsureCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "ensure",
		Usage:     "Idempotently ensure table defaults",
		ArgsUsage: "<entity>",
		Flags: []cli.Flag{
			&cli.Int64Flag{Name: "permission", Usage: "DefaultPermission value"},
			&cli.StringSliceFlag{Name: "group", Usage: "Default group as name or name:permission"},
		},
		Action: func(c *cli.Context) error {
			defaults, err := loadTableDefaults(appCtx, c.Args().Get(0))
			if err != nil {
				return err
			}
			changed := false
			if c.IsSet("permission") {
				changed = defaults.setPermission(c.Int64("permission")) || changed
			}
			for _, group := range c.StringSlice("group") {
				name, permission, err := parseDefaultGroupArg(group)
				if err != nil {
					return err
				}
				changed = defaults.ensureGroup(name, permission) || changed
			}
			if changed {
				if err := saveTableDefaults(appCtx, defaults); err != nil {
					return err
				}
			}
			return appCtx.Renderer.RenderObject(defaults.render(changed))
		},
	}
}

type tableDefaults struct {
	EntityName string
	RefID      string
	Schema     map[string]interface{}
}

func loadTableDefaults(appCtx *AppContext, entityName string) (*tableDefaults, error) {
	if entityName == "" {
		return nil, fmt.Errorf("entity name required")
	}
	worlds, err := appCtx.Client.FindAll("world", daptinClient.DaptinQueryParameters{"page[size]": 500})
	if err != nil {
		return nil, err
	}
	for _, row := range client.MapArray(worlds, "attributes") {
		if row["table_name"] != entityName {
			continue
		}
		schemaJSON, _ := row["world_schema_json"].(string)
		if schemaJSON == "" {
			return nil, fmt.Errorf("world_schema_json for %q is empty", entityName)
		}
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
			return nil, err
		}
		ref, _ := row["reference_id"].(string)
		if ref == "" {
			return nil, fmt.Errorf("world row for %q has no reference_id", entityName)
		}
		return &tableDefaults{EntityName: entityName, RefID: ref, Schema: schema}, nil
	}
	return nil, fmt.Errorf("entity %q not found", entityName)
}

func saveTableDefaults(appCtx *AppContext, defaults *tableDefaults) error {
	schemaJSON, err := json.Marshal(defaults.Schema)
	if err != nil {
		return err
	}
	attrs := map[string]interface{}{
		"world_schema_json": string(schemaJSON),
	}
	if permission, ok := defaults.defaultPermission(); ok {
		attrs["default_permission"] = permission
	}
	_, err = appCtx.Client.Update("world", defaults.RefID, jsonAPIObject("world", attrs, defaults.RefID))
	return err
}

func (d *tableDefaults) render(changed bool) map[string]interface{} {
	permission, _ := d.defaultPermission()
	return map[string]interface{}{
		"table_name":          d.EntityName,
		"reference_id":        d.RefID,
		"DefaultPermission":   permission,
		"DefaultGroups":       d.defaultGroups(),
		"Changed":             changed,
		"restart_recommended": true,
	}
}

func (d *tableDefaults) defaultPermission() (int64, bool) {
	switch value := d.Schema["DefaultPermission"].(type) {
	case float64:
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (d *tableDefaults) setPermission(permission int64) bool {
	current, ok := d.defaultPermission()
	if ok && current == permission {
		return false
	}
	d.Schema["DefaultPermission"] = permission
	return true
}

func (d *tableDefaults) defaultGroups() []map[string]interface{} {
	rawGroups, _ := d.Schema["DefaultGroups"].([]interface{})
	groups := make([]map[string]interface{}, 0, len(rawGroups))
	for _, raw := range rawGroups {
		switch group := raw.(type) {
		case string:
			groups = append(groups, map[string]interface{}{"Name": group})
		case map[string]interface{}:
			normalized := map[string]interface{}{}
			if name, ok := group["Name"]; ok {
				normalized["Name"] = name
			}
			if permission, ok := group["Permission"]; ok {
				normalized["Permission"] = permission
			}
			groups = append(groups, normalized)
		}
	}
	return groups
}

func (d *tableDefaults) ensureGroup(name string, permission *int64) bool {
	if name == "" {
		return false
	}
	rawGroups, _ := d.Schema["DefaultGroups"].([]interface{})
	for i, raw := range rawGroups {
		groupName, _ := defaultGroupName(raw)
		if groupName != name {
			continue
		}
		if permission == nil {
			return false
		}
		groupMap := defaultGroupMap(raw)
		current, ok := defaultGroupPermission(groupMap)
		if ok && current == *permission {
			return false
		}
		groupMap["Permission"] = *permission
		rawGroups[i] = groupMap
		d.Schema["DefaultGroups"] = rawGroups
		return true
	}
	newGroup := map[string]interface{}{"Name": name}
	if permission != nil {
		newGroup["Permission"] = *permission
	}
	d.Schema["DefaultGroups"] = append(rawGroups, newGroup)
	return true
}

func defaultGroupName(raw interface{}) (string, bool) {
	switch group := raw.(type) {
	case string:
		return group, true
	case map[string]interface{}:
		name, ok := group["Name"].(string)
		return name, ok
	default:
		return "", false
	}
}

func defaultGroupMap(raw interface{}) map[string]interface{} {
	if group, ok := raw.(map[string]interface{}); ok {
		result := map[string]interface{}{}
		for key, value := range group {
			result[key] = value
		}
		return result
	}
	name, _ := defaultGroupName(raw)
	return map[string]interface{}{"Name": name}
}

func defaultGroupPermission(group map[string]interface{}) (int64, bool) {
	switch value := group["Permission"].(type) {
	case float64:
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	default:
		return 0, false
	}
}

func parseDefaultGroupArg(value string) (string, *int64, error) {
	name, permissionText, hasPermission := strings.Cut(value, ":")
	if name == "" {
		return "", nil, fmt.Errorf("group name is required")
	}
	if !hasPermission {
		return name, nil, nil
	}
	permission, err := strconv.ParseInt(permissionText, 10, 64)
	if err != nil {
		return "", nil, fmt.Errorf("invalid group permission %q: %w", permissionText, err)
	}
	return name, &permission, nil
}
