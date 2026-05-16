package cmd

import (
	"fmt"
	"log/slog"
	"os"

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
	Quiet    bool
}

func NewApp(cfg *config.Config, version string) *cli.App {
	appCtx := &AppContext{Config: cfg}

	app := &cli.App{
		Name:    "daptin",
		Usage:   "CLI client for Daptin API server",
		Version: version,
		ExitErrHandler: func(c *cli.Context, err error) {
			if err == nil {
				return
			}
			if exitErr, ok := err.(cli.ExitCoder); ok {
				if err.Error() != "" {
					fmt.Fprintln(os.Stderr, err)
				}
				cli.OsExiter(exitErr.ExitCode())
				return
			}
			fmt.Fprintln(os.Stderr, err)
			cli.OsExiter(1)
		},
		Before: func(c *cli.Context) error {
			InitLogger(c.Bool("debug"))

			endpoint := c.String("endpoint")
			authToken := ""
			contextName := endpoint

			// Explicit --endpoint flag wins over saved context
			if c.IsSet("endpoint") {
				contextName = endpoint
				slog.Debug("context resolution", "source", "endpoint_flag", "endpoint", endpoint)
			} else if cfg.CurrentContext != "" {
				if host, err := cfg.ActiveHost(); err == nil {
					endpoint = host.Endpoint
					authToken = host.Token
					contextName = host.Name
					slog.Debug("context resolution", "source", "saved_context", "context", contextName, "endpoint", endpoint, "token_present", authToken != "")
				}
			}

			appCtx.Client = client.New(endpoint, authToken, c.Bool("debug"))
			appCtx.Quiet = c.Bool("quiet")

			if !appCtx.Quiet {
				authed := ""
				if authToken != "" {
					authed = ", authenticated"
				}
				fmt.Fprintf(os.Stderr, "Using %s (%s%s)\n", contextName, endpoint, authed)
			}

			outputFmt := c.String("output")
			switch outputFmt {
			case "json":
				appCtx.Renderer = render.NewJsonRenderer()
			default:
				if c.Bool("no-truncate") {
					appCtx.Renderer = render.NewTableRendererNoTruncate()
				} else {
					appCtx.Renderer = render.NewTableRenderer()
				}
			}
			slog.Info("renderer selected", "output", outputFmt)

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
			&cli.BoolFlag{
				Name:  "no-truncate",
				Usage: "Show full values in table output (no 50-char truncation)",
			},
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Usage:   "Output only reference_id (for scripting)",
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
			relateCommand(appCtx),
			unrelateCommand(appCtx),
			describeCommand(appCtx),
			executeCommand(appCtx),
			oauthCommand(appCtx),
			integrationCommand(appCtx),
			storageCommand(appCtx),
			assetCommand(appCtx),
			permissionCommand(appCtx),
			tableCommand(appCtx),
			wsCommand(appCtx),
		},
	}

	return app
}
