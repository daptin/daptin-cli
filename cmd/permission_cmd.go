package cmd

import (
	"fmt"
	"strconv"

	"github.com/urfave/cli/v2"
)

func permissionCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "permission",
		Usage: "Decode and encode Daptin permission values",
		Subcommands: []*cli.Command{
			{
				Name:      "decode",
				Usage:     "Show human-readable breakdown of a permission value",
				ArgsUsage: "<value>",
				Action: func(c *cli.Context) error {
					valStr := c.Args().Get(0)
					if valStr == "" {
						return fmt.Errorf("usage: permission decode <value>")
					}
					val, err := strconv.ParseInt(valStr, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid permission value %q: %w", valStr, err)
					}
					fmt.Printf("%d:\n%s", val, FormatPermission(val))
					return nil
				},
			},
			{
				Name:      "encode",
				Usage:     "Compute permission value from modifiers (e.g., +GuestRead +OwnerUpdate)",
				ArgsUsage: "[+/-TierOp ...]",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:  "base",
						Usage: "Base permission value to modify (default: 0)",
						Value: 0,
					},
				},
				Action: func(c *cli.Context) error {
					value := c.Int64("base")
					for _, mod := range c.Args().Slice() {
						var err error
						value, err = ApplyPermissionModifier(value, mod)
						if err != nil {
							return err
						}
					}
					fmt.Printf("%d:\n%s", value, FormatPermission(value))
					return nil
				},
			},
		},
	}
}
