package cmd

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

func relateCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "relate",
		Usage:     "Add a relationship association between entities",
		ArgsUsage: "<entity> <reference_id> <relation_column> <target_ref_id>",
		Action: func(c *cli.Context) error {
			entity := c.Args().Get(0)
			refId := c.Args().Get(1)
			relation := c.Args().Get(2)
			targetRefId := c.Args().Get(3)
			if entity == "" || refId == "" || relation == "" || targetRefId == "" {
				return fmt.Errorf("usage: relate <entity> <reference_id> <relation_column> <target_ref_id>")
			}

			// Derive target type from relation column name (e.g., "usergroup_id" → "usergroup")
			targetType := strings.TrimSuffix(relation, "_id")

			err := appCtx.Client.AddRelation(entity, refId, relation, targetType, targetRefId)
			if err != nil {
				return err
			}
			fmt.Println("Related")
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

			targetType := strings.TrimSuffix(relation, "_id")
			err := appCtx.Client.RemoveRelation(entity, refId, relation, targetType, targetRefId)
			if err != nil {
				return err
			}
			fmt.Println("Unrelated")
			return nil
		},
	}
}
