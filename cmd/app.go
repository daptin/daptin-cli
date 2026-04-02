package cmd

import (
	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/config"
	"github.com/daptin/daptin-cli/render"
	"github.com/urfave/cli/v2"
)

// AppContext holds the shared dependencies for all commands.
type AppContext struct {
	Client   *client.ExtendedClient
	Config   *config.Config
	Renderer render.Renderer
}

func NewApp(cfg *config.Config, version string) *cli.App {
	appCtx := &AppContext{Config: cfg}

	app := &cli.App{
		Name:    "daptin",
		Usage:   "CLI client for Daptin API server",
		Version: version,
		Before: func(c *cli.Context) error {
			// Resolve endpoint: config context > flag > default
			endpoint := c.String("endpoint")
			authToken := ""
			if cfg.CurrentContext != "" {
				if host, err := cfg.ActiveHost(); err == nil {
					endpoint = host.Endpoint
					authToken = host.Token
				}
			}
			if ep := c.String("endpoint"); ep != "http://localhost:6336" {
				endpoint = ep
			}

			appCtx.Client = client.New(endpoint, authToken, c.Bool("debug"))

			switch c.String("output") {
			case "json":
				appCtx.Renderer = render.NewJsonRenderer()
			default:
				appCtx.Renderer = render.NewTableRenderer()
			}

			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Usage:       "Load configuration from `FILE`",
				DefaultText: "~/.daptin/config.yaml",
				EnvVars:     []string{"DAPTIN_CLI_CONFIG"},
			},
			&cli.StringFlag{
				Name:        "output",
				Aliases:     []string{"o"},
				Usage:       "Output format: table or json",
				DefaultText: "table",
				Value:       "table",
				EnvVars:     []string{"DAPTIN_CLI_OUTPUT"},
			},
			&cli.StringFlag{
				Name:        "endpoint",
				Usage:       "Daptin server endpoint",
				DefaultText: "http://localhost:6336",
				Value:       "http://localhost:6336",
				EnvVars:     []string{"DAPTIN_ENDPOINT"},
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug output",
			},
		},
		Commands: []*cli.Command{
			contextCommand(appCtx),
			listCommand(appCtx),
			getCommand(appCtx),
			createCommand(appCtx),
			updateCommand(appCtx),
			deleteCommand(appCtx),
			relatedCommand(appCtx),
			describeCommand(appCtx),
			executeCommand(appCtx),
		},
	}

	return app
}
