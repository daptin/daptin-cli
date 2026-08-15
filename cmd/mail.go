package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/daptin/daptin-cli/mailprobe"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func mailCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "mail",
		Usage: "Qualify Daptin-owned SMTP and IMAP transports",
		Subcommands: []*cli.Command{
			smtpCheckCommand(appCtx),
			smtpDeliveryCheckCommand(appCtx),
			imapCheckCommand(appCtx),
			mailQualifyCommand(appCtx),
		},
	}
}

func smtpCheckCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "smtp-check",
		Usage: "Check SMTP listener and verified TLS readiness without sending mail",
		Flags: connectionFlags(false),
		Action: func(c *cli.Context) error {
			if err := requireExplicitMailContext(appCtx); err != nil {
				return err
			}
			connection, err := connectionFromCLI(c, "address", "tls")
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(c.Context, connection.Timeout)
			defer cancel()
			result, err := mailprobe.CheckSMTP(ctx, connection)
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(smtpCheckForRender("smtp_check", result))
		},
	}
}

func smtpDeliveryCheckCommand(appCtx *AppContext) *cli.Command {
	flags := connectionFlags(false)
	flags = append(flags,
		&cli.StringFlag{Name: "username", Usage: "SMTP account identity (required unless --dry-run)"},
		&cli.StringFlag{Name: "from", Usage: "Envelope sender (required unless --dry-run)"},
		&cli.StringFlag{Name: "to", Usage: "Envelope recipient (required unless --dry-run)"},
		&cli.StringFlag{Name: "correlation-id", Usage: "Opaque Message-ID correlation value (required unless --dry-run)"},
		&cli.BoolFlag{Name: "credential-stdin", Usage: "Read the SMTP password from stdin"},
		&cli.StringFlag{Name: "credential-file", Usage: "Read the SMTP password from a mode-0600 file"},
		&cli.BoolFlag{Name: "dry-run", Usage: "Check TCP and TLS without authenticating or sending"},
		&cli.BoolFlag{Name: "yes", Usage: "Confirm submission without an interactive prompt"},
	)
	return &cli.Command{
		Name:  "smtp-delivery-check",
		Usage: "Authenticate and submit one correlation-marked synthetic message",
		Flags: flags,
		Action: func(c *cli.Context) error {
			if err := requireExplicitMailContext(appCtx); err != nil {
				return err
			}
			connection, err := connectionFromCLI(c, "address", "tls")
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(c.Context, connection.Timeout)
			defer cancel()
			if c.Bool("dry-run") {
				result, err := mailprobe.CheckSMTP(ctx, connection)
				if err != nil {
					return err
				}
				return appCtx.Renderer.RenderObject(smtpCheckForRender("smtp_delivery_dry_run", result))
			}
			if err := validateDeliveryCLI(c); err != nil {
				return err
			}
			if err := confirmMailSubmission(c.Bool("yes")); err != nil {
				return err
			}
			password, err := mailprobe.ReadCredential(os.Stdin, c.Bool("credential-stdin"), c.String("credential-file"))
			if err != nil {
				return err
			}
			defer clearBytes(password)
			result, err := mailprobe.DeliverSMTP(ctx, mailprobe.SMTPDeliveryRequest{
				Connection: connection, Username: c.String("username"), Password: password,
				From: c.String("from"), To: c.String("to"), CorrelationID: c.String("correlation-id"),
			})
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(smtpDeliveryForRender(result, c.String("correlation-id")))
		},
	}
}

func imapCheckCommand(appCtx *AppContext) *cli.Command {
	flags := connectionFlags(false)
	flags = append(flags,
		&cli.StringFlag{Name: "username", Usage: "IMAP account identity"},
		&cli.StringFlag{Name: "folder", Usage: "Mailbox folder to select"},
		&cli.StringFlag{Name: "correlation-id", Usage: "Opaque Message-ID correlation value"},
		&cli.BoolFlag{Name: "credential-stdin", Usage: "Read the IMAP password from stdin"},
		&cli.StringFlag{Name: "credential-file", Usage: "Read the IMAP password from a mode-0600 file"},
		&cli.DurationFlag{Name: "poll-timeout", Value: 30 * time.Second, Usage: "Maximum correlation search duration"},
		&cli.DurationFlag{Name: "poll-interval", Value: 500 * time.Millisecond, Usage: "Correlation search interval"},
	)
	return &cli.Command{
		Name:  "imap-check",
		Usage: "Check IMAP TLS readiness or authenticate and find a correlated message",
		Flags: flags,
		Action: func(c *cli.Context) error {
			if err := requireExplicitMailContext(appCtx); err != nil {
				return err
			}
			connection, err := connectionFromCLI(c, "address", "tls")
			if err != nil {
				return err
			}
			authenticated := c.String("username") != "" || c.String("folder") != "" || c.String("correlation-id") != "" || c.Bool("credential-stdin") || c.String("credential-file") != ""
			if !authenticated {
				ctx, cancel := context.WithTimeout(c.Context, connection.Timeout)
				defer cancel()
				result, err := mailprobe.CheckIMAP(ctx, connection)
				if err != nil {
					return err
				}
				return appCtx.Renderer.RenderObject(imapForRender("imap_check", result, ""))
			}
			if c.String("username") == "" || c.String("folder") == "" || c.String("correlation-id") == "" {
				return fmt.Errorf("authenticated IMAP check requires username, folder, correlation ID, and one credential source")
			}
			password, err := mailprobe.ReadCredential(os.Stdin, c.Bool("credential-stdin"), c.String("credential-file"))
			if err != nil {
				return err
			}
			defer clearBytes(password)
			ctx, cancel := context.WithTimeout(c.Context, connection.Timeout+c.Duration("poll-timeout"))
			defer cancel()
			result, err := mailprobe.SearchIMAP(ctx, mailprobe.IMAPSearchRequest{
				Connection: connection, Username: c.String("username"), Password: password,
				Folder: c.String("folder"), CorrelationID: c.String("correlation-id"),
				PollTimeout: c.Duration("poll-timeout"), PollInterval: c.Duration("poll-interval"),
			})
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(imapForRender("imap_check", result, c.String("correlation-id")))
		},
	}
}

func mailQualifyCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "qualify",
		Usage: "Submit through Daptin SMTP and prove receipt through Daptin IMAP",
		Flags: qualificationFlags(),
		Action: func(c *cli.Context) error {
			if err := requireExplicitMailContext(appCtx); err != nil {
				return err
			}
			smtpConnection, err := connectionFromCLI(c, "smtp-address", "smtp-tls")
			if err != nil {
				return err
			}
			imapConnection, err := connectionFromCLI(c, "imap-address", "imap-tls")
			if err != nil {
				return err
			}
			if c.Bool("dry-run") {
				ctx, cancel := context.WithTimeout(c.Context, smtpConnection.Timeout+imapConnection.Timeout)
				defer cancel()
				smtpResult, err := mailprobe.CheckSMTP(ctx, smtpConnection)
				if err != nil {
					return err
				}
				imapResult, err := mailprobe.CheckIMAP(ctx, imapConnection)
				if err != nil {
					return err
				}
				return appCtx.Renderer.RenderObject(qualificationForRender(smtpResult, nil, imapResult, "", true))
			}
			if err := validateQualificationCLI(c); err != nil {
				return err
			}
			if err := confirmMailSubmission(c.Bool("yes")); err != nil {
				return err
			}
			credentials, err := mailprobe.ReadQualificationCredentials(os.Stdin, c.Bool("credentials-stdin"), c.String("credentials-file"))
			if err != nil {
				return err
			}
			smtpPassword := []byte(credentials.SMTPPassword)
			imapPassword := []byte(credentials.IMAPPassword)
			defer clearBytes(smtpPassword)
			defer clearBytes(imapPassword)
			ctx, cancel := context.WithTimeout(c.Context, smtpConnection.Timeout+imapConnection.Timeout+c.Duration("poll-timeout"))
			defer cancel()
			smtpResult, err := mailprobe.DeliverSMTP(ctx, mailprobe.SMTPDeliveryRequest{
				Connection: smtpConnection, Username: c.String("smtp-username"), Password: smtpPassword,
				From: c.String("from"), To: c.String("to"), CorrelationID: c.String("correlation-id"),
			})
			if err != nil {
				return err
			}
			imapResult, err := mailprobe.SearchIMAP(ctx, mailprobe.IMAPSearchRequest{
				Connection: imapConnection, Username: c.String("imap-username"), Password: imapPassword,
				Folder: c.String("folder"), CorrelationID: c.String("correlation-id"),
				PollTimeout: c.Duration("poll-timeout"), PollInterval: c.Duration("poll-interval"),
			})
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(qualificationForRender(smtpResult.SMTPCheckResult, &smtpResult, imapResult, c.String("correlation-id"), false))
		},
	}
}

func connectionFlags(prefixed bool) []cli.Flag {
	address, tlsName := "address", "tls"
	if prefixed {
		address, tlsName = "smtp-address", "smtp-tls"
	}
	return []cli.Flag{
		&cli.StringFlag{Name: address, Required: true, Usage: "Listener address as host:port"},
		&cli.StringFlag{Name: tlsName, Required: true, Usage: "TLS mode: implicit or starttls"},
		&cli.StringFlag{Name: "server-name", Required: true, Usage: "TLS certificate hostname"},
		&cli.StringFlag{Name: "ca-file", Usage: "Additional public CA certificate file"},
		&cli.DurationFlag{Name: "timeout", Value: 10 * time.Second, Usage: "Connection and protocol timeout"},
	}
}

func qualificationFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "smtp-address", Required: true}, &cli.StringFlag{Name: "smtp-tls", Required: true},
		&cli.StringFlag{Name: "imap-address", Required: true}, &cli.StringFlag{Name: "imap-tls", Required: true},
		&cli.StringFlag{Name: "server-name", Required: true}, &cli.StringFlag{Name: "ca-file"},
		&cli.DurationFlag{Name: "timeout", Value: 10 * time.Second},
		&cli.StringFlag{Name: "smtp-username", Usage: "Required unless --dry-run"}, &cli.StringFlag{Name: "imap-username", Usage: "Required unless --dry-run"},
		&cli.StringFlag{Name: "from", Usage: "Required unless --dry-run"}, &cli.StringFlag{Name: "to", Usage: "Required unless --dry-run"},
		&cli.StringFlag{Name: "folder", Value: "INBOX"}, &cli.StringFlag{Name: "correlation-id", Usage: "Required unless --dry-run"},
		&cli.BoolFlag{Name: "credentials-stdin"}, &cli.StringFlag{Name: "credentials-file"},
		&cli.DurationFlag{Name: "poll-timeout", Value: 30 * time.Second}, &cli.DurationFlag{Name: "poll-interval", Value: 500 * time.Millisecond},
		&cli.BoolFlag{Name: "dry-run"}, &cli.BoolFlag{Name: "yes"},
	}
}

func connectionFromCLI(c *cli.Context, addressFlag, tlsFlag string) (mailprobe.Connection, error) {
	mode, err := mailprobe.ParseTLSMode(c.String(tlsFlag))
	if err != nil {
		return mailprobe.Connection{}, err
	}
	connection := mailprobe.Connection{Address: c.String(addressFlag), ServerName: c.String("server-name"), CAFile: c.String("ca-file"), TLSMode: mode, Timeout: c.Duration("timeout")}
	return connection, mailprobe.ValidateConnection(connection)
}

func requireExplicitMailContext(appCtx *AppContext) error {
	if !appCtx.ConfigExplicit {
		return fmt.Errorf("mail commands require an explicit --config file or DAPTIN_CLI_CONFIG")
	}
	if appCtx.EndpointExplicit {
		return fmt.Errorf("mail commands do not allow --endpoint overrides; select a saved context in the explicit config")
	}
	if appCtx.ContextName == "" {
		return fmt.Errorf("mail commands require an active saved context")
	}
	if _, err := appCtx.Config.ActiveHost(); err != nil {
		return fmt.Errorf("mail commands require an active saved context: %w", err)
	}
	return nil
}

func validateDeliveryCLI(c *cli.Context) error {
	for name, value := range map[string]string{"SMTP username": c.String("username"), "sender": c.String("from"), "recipient": c.String("to")} {
		if err := mailprobe.ValidateProtocolValue(name, value, 255); err != nil {
			return err
		}
	}
	return mailprobe.ValidateCorrelationID(c.String("correlation-id"))
}

func validateQualificationCLI(c *cli.Context) error {
	for name, value := range map[string]string{"SMTP username": c.String("smtp-username"), "IMAP username": c.String("imap-username"), "sender": c.String("from"), "recipient": c.String("to"), "folder": c.String("folder")} {
		if err := mailprobe.ValidateProtocolValue(name, value, 255); err != nil {
			return err
		}
	}
	return mailprobe.ValidateCorrelationID(c.String("correlation-id"))
}

func confirmMailSubmission(yes bool) error {
	if yes {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("mail submission requires --yes in non-interactive use")
	}
	fmt.Fprint(os.Stderr, "Submit one synthetic mail qualification message? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(line)) != "y" && strings.ToLower(strings.TrimSpace(line)) != "yes" {
		return fmt.Errorf("mail submission cancelled")
	}
	return nil
}

func clearBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func tlsForRender(result mailprobe.TLSResult) map[string]interface{} {
	out := map[string]interface{}{"tls_verified": result.Verified, "tls_version": result.Version, "certificate_hostname_valid": result.Verified}
	if !result.ExpiresAt.IsZero() {
		out["certificate_expires_at"] = result.ExpiresAt.Format(time.RFC3339)
	}
	return out
}

func smtpCheckForRender(operation string, result mailprobe.SMTPCheckResult) map[string]interface{} {
	out := tlsForRender(result.TLS)
	out["operation"] = operation
	out["connected"] = result.Connected
	out["smtp_ready"] = result.SMTPReady
	out["starttls_advertised"] = result.StartTLSAdvertised
	out["auth_login_advertised"] = result.AuthLoginAdvertised
	out["duration_ms"] = result.Duration.Milliseconds()
	return out
}

func smtpDeliveryForRender(result mailprobe.SMTPDeliveryResult, correlation string) map[string]interface{} {
	out := smtpCheckForRender("smtp_delivery_check", result.SMTPCheckResult)
	out["authenticated"] = result.Authenticated
	out["sender_accepted"] = result.SenderAccepted
	out["recipient_accepted"] = result.RecipientAccepted
	out["message_accepted"] = result.MessageAccepted
	out["correlation_id"] = correlation
	return out
}

func imapForRender(operation string, result mailprobe.IMAPResult, correlation string) map[string]interface{} {
	out := tlsForRender(result.TLS)
	out["operation"] = operation
	out["connected"] = result.Connected
	out["authenticated"] = result.Authenticated
	out["folder_selected"] = result.FolderSelected
	out["correlated_message_found"] = result.MessageFound
	out["poll_attempts"] = result.PollAttempts
	out["duration_ms"] = result.Duration.Milliseconds()
	if correlation != "" {
		out["correlation_id"] = correlation
	}
	return out
}

func qualificationForRender(smtpCheck mailprobe.SMTPCheckResult, delivery *mailprobe.SMTPDeliveryResult, imap mailprobe.IMAPResult, correlation string, dryRun bool) map[string]interface{} {
	out := map[string]interface{}{"operation": "mail_qualify", "dry_run": dryRun, "smtp": smtpCheckForRender("smtp", smtpCheck), "imap": imapForRender("imap", imap, correlation)}
	if delivery != nil {
		out["smtp"] = smtpDeliveryForRender(*delivery, correlation)
		out["correlation_id"] = correlation
	}
	return out
}
