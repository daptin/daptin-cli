package cmd

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/urfave/cli/v2"
)

func relateCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "relate",
		Usage:     "Add a relationship association between entities",
		ArgsUsage: "<entity> <reference_id> <relation_column> <target_ref_id> [key=val ...]",
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()
			entity := c.Args().Get(0)
			refId := c.Args().Get(1)
			relation := c.Args().Get(2)
			targetRefId := c.Args().Get(3)
			if entity == "" || refId == "" || relation == "" || targetRefId == "" {
				return fmt.Errorf("usage: relate <entity> <reference_id> <relation_column> <target_ref_id> [key=val ...]")
			}
			attrs, err := parseRelationAttributes(args[4:])
			if err != nil {
				return err
			}

			slog.Info("relate", "entity", entity, "reference_id", refId, "relation", relation)
			// Derive target type from relation column name (e.g., "usergroup_id" -> "usergroup")
			targetType := client.RelationTargetType(relation)
			slog.Debug("derived target type", "target_type", targetType)

			if err := appCtx.Client.AddRelation(entity, refId, relation, targetType, targetRefId); err != nil {
				return err
			}
			if len(attrs) > 0 {
				if _, err := appCtx.Client.UpdateRelationAttributes(entity, refId, relation, targetRefId, attrs); err != nil {
					return err
				}
			}
			fmt.Println("Related")
			return nil
		},
	}
}

func updateRelationCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "update-relation",
		Usage:     "Update attributes on an existing relationship association",
		ArgsUsage: "<entity> <reference_id> <relation_column> <target_ref_id> [key=val ...]",
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()
			entity := c.Args().Get(0)
			refId := c.Args().Get(1)
			relation := c.Args().Get(2)
			targetRefId := c.Args().Get(3)
			if entity == "" || refId == "" || relation == "" || targetRefId == "" || len(args) <= 4 {
				return fmt.Errorf("usage: update-relation <entity> <reference_id> <relation_column> <target_ref_id> [key=val ...]")
			}

			attrs, err := parseRelationAttributes(args[4:])
			if err != nil {
				return err
			}

			slog.Info("update-relation", "entity", entity, "reference_id", refId, "relation", relation)
			if _, err := appCtx.Client.UpdateRelationAttributes(entity, refId, relation, targetRefId, attrs); err != nil {
				return err
			}
			fmt.Println("Relation updated")
			return nil
		},
	}
}

func unrelateCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "unrelate",
		Usage:     "Remove a relationship association between entities",
		ArgsUsage: "<entity> <reference_id> <relation_column> <target_ref_id>",
		Action: func(c *cli.Context) error {
			entity := c.Args().Get(0)
			refId := c.Args().Get(1)
			relation := c.Args().Get(2)
			targetRefId := c.Args().Get(3)
			if entity == "" || refId == "" || relation == "" || targetRefId == "" {
				return fmt.Errorf("usage: unrelate <entity> <reference_id> <relation_column> <target_ref_id>")
			}

			slog.Info("unrelate", "entity", entity, "reference_id", refId, "relation", relation)
			targetType := strings.TrimSuffix(relation, "_id")
			slog.Debug("derived target type", "target_type", targetType)
			err := appCtx.Client.RemoveRelation(entity, refId, relation, targetType, targetRefId)
			if err != nil {
				return err
			}
			fmt.Println("Unrelated")
			return nil
		},
	}
}

func parseRelationAttributes(args []string) (map[string]interface{}, error) {
	attrs, err := parseAttributes(args)
	if err != nil {
		return nil, err
	}
	for key, value := range attrs {
		if stringValue, ok := value.(string); ok {
			attrs[key] = parseRelationAttributeValue(stringValue)
		}
	}
	return attrs, nil
}

func parseRelationAttributeValue(value string) interface{} {
	if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
		return intValue
	}
	if boolValue, err := strconv.ParseBool(value); err == nil {
		return boolValue
	}
	if strings.ContainsAny(value, ".eE") {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return value
}
