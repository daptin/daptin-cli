package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/daptin/daptin-cli/client"
	"github.com/urfave/cli/v2"
)

func wsCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "ws",
		Usage: "WebSocket pub/sub commands",
		Subcommands: []*cli.Command{
			wsListenCommand(appCtx),
			wsPublishCommand(appCtx),
			wsTopicCommand(appCtx),
			wsVerifyCommand(appCtx),
		},
	}
}

func wsListenCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "listen",
		Usage:     "Subscribe to a topic and stream events",
		ArgsUsage: "<topic>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "filter",
				Usage: "Filter expression key=value (repeatable)",
			},
		},
		Action: func(c *cli.Context) error {
			topic := c.Args().Get(0)
			if topic == "" {
				return fmt.Errorf("topic name required")
			}

			ws, err := client.DialWebSocket(appCtx.Client.Endpoint, appCtx.Client.AuthToken)
			if err != nil {
				return err
			}
			defer ws.Close()

			if !appCtx.Quiet {
				fmt.Fprintf(os.Stderr, "Connected (session open)\n")
			}

			// Build subscribe attributes
			attrs := map[string]interface{}{
				"topicName": topic,
			}
			if filters := c.StringSlice("filter"); len(filters) > 0 {
				filterMap := make(map[string]interface{}, len(filters))
				for _, f := range filters {
					parts := strings.SplitN(f, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid filter %q, expected key=value", f)
					}
					filterMap[parts[0]] = parts[1]
				}
				attrs["filters"] = filterMap
			}

			id, err := ws.Send("subscribe", attrs)
			if err != nil {
				return err
			}

			_, err = ws.WaitResponse(id, nil)
			if err != nil {
				return fmt.Errorf("subscribe failed: %w", err)
			}

			if !appCtx.Quiet {
				fmt.Fprintf(os.Stderr, "Subscribed to %s\n", topic)
			}

			// Handle SIGINT for clean shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt)

			go func() {
				<-sigCh
				ws.Send("unsubscribe", map[string]interface{}{"topicName": topic})
				ws.Close()
				os.Exit(0)
			}()

			// Stream events
			for {
				msg, err := ws.ReadMessage()
				if err != nil {
					return nil // connection closed
				}
				if msg["type"] == "pong" {
					continue
				}
				line, err := client.EventToJSONLine(msg)
				if err != nil {
					continue
				}
				fmt.Println(line)
			}
		},
	}
}

func wsPublishCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "publish",
		Usage:     "Publish a message to a topic",
		ArgsUsage: "<topic> <json-message>",
		Action: func(c *cli.Context) error {
			topic := c.Args().Get(0)
			msgStr := c.Args().Get(1)
			if topic == "" || msgStr == "" {
				return fmt.Errorf("usage: ws publish <topic> <json-message>")
			}

			var msgPayload map[string]interface{}
			if err := json.Unmarshal([]byte(msgStr), &msgPayload); err != nil {
				return fmt.Errorf("invalid JSON message: %w", err)
			}

			ws, err := client.DialWebSocket(appCtx.Client.Endpoint, appCtx.Client.AuthToken)
			if err != nil {
				return err
			}
			defer ws.Close()

			id, err := ws.Send("new-message", map[string]interface{}{
				"topicName": topic,
				"message":   msgPayload,
			})
			if err != nil {
				return err
			}

			// Server only responds on error; timeout means success
			_, err = ws.WaitResponseTimeout(id, 2*time.Second)
			if err != nil {
				return fmt.Errorf("publish failed: %w", err)
			}

			if !appCtx.Quiet {
				fmt.Fprintf(os.Stderr, "Published to %s\n", topic)
			}
			return nil
		},
	}
}

func wsTopicCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "topic",
		Usage: "Manage user-created topics",
		Subcommands: []*cli.Command{
			{
				Name:      "create",
				Usage:     "Create a user topic",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					name := c.Args().Get(0)
					if name == "" {
						return fmt.Errorf("topic name required")
					}

					ws, err := client.DialWebSocket(appCtx.Client.Endpoint, appCtx.Client.AuthToken)
					if err != nil {
						return err
					}
					defer ws.Close()

					id, err := ws.Send("create-topicName", map[string]interface{}{
						"name": name,
					})
					if err != nil {
						return err
					}

					_, err = ws.WaitResponse(id, nil)
					if err != nil {
						return fmt.Errorf("create topic failed: %w", err)
					}

					if !appCtx.Quiet {
						fmt.Fprintf(os.Stderr, "Created topic %s\n", name)
					}
					return nil
				},
			},
			{
				Name:      "destroy",
				Usage:     "Destroy a user topic",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					name := c.Args().Get(0)
					if name == "" {
						return fmt.Errorf("topic name required")
					}

					ws, err := client.DialWebSocket(appCtx.Client.Endpoint, appCtx.Client.AuthToken)
					if err != nil {
						return err
					}
					defer ws.Close()

					id, err := ws.Send("destroy-topicName", map[string]interface{}{
						"name": name,
					})
					if err != nil {
						return err
					}

					_, err = ws.WaitResponse(id, nil)
					if err != nil {
						return fmt.Errorf("destroy topic failed: %w", err)
					}

					if !appCtx.Quiet {
						fmt.Fprintf(os.Stderr, "Destroyed topic %s\n", name)
					}
					return nil
				},
			},
			{
				Name:      "set-permission",
				Usage:     "Set permission on a user topic",
				ArgsUsage: "<name> <permission>",
				Action: func(c *cli.Context) error {
					name := c.Args().Get(0)
					permStr := c.Args().Get(1)
					if name == "" || permStr == "" {
						return fmt.Errorf("usage: ws topic set-permission <name> <permission>")
					}

					perm, err := strconv.ParseInt(permStr, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid permission value: %w", err)
					}

					ws, err := client.DialWebSocket(appCtx.Client.Endpoint, appCtx.Client.AuthToken)
					if err != nil {
						return err
					}
					defer ws.Close()

					id, err := ws.Send("set-topic-permission", map[string]interface{}{
						"topicName":  name,
						"permission": perm,
					})
					if err != nil {
						return err
					}

					_, err = ws.WaitResponse(id, nil)
					if err != nil {
						return fmt.Errorf("set-permission failed: %w", err)
					}

					if !appCtx.Quiet {
						fmt.Fprintf(os.Stderr, "Set permission %d on topic %s\n", perm, name)
					}
					return nil
				},
			},
		},
	}
}

func wsVerifyCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "verify",
		Usage: "Verify cross-node PubSub delivery between two endpoints",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "endpoints",
				Usage:    "Comma-separated endpoints (exactly 2)",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			parts := strings.Split(c.String("endpoints"), ",")
			if len(parts) != 2 {
				return fmt.Errorf("exactly 2 endpoints required, got %d", len(parts))
			}
			epA := strings.TrimSpace(parts[0])
			epB := strings.TrimSpace(parts[1])

			topicName := fmt.Sprintf("cluster-verify-%d", time.Now().UnixMilli())

			// Connect to both endpoints
			fmt.Fprintf(os.Stderr, "Connecting to %s... ", epA)
			wsA, err := client.DialWebSocket(epA, appCtx.Client.AuthToken)
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL\n")
				return err
			}
			defer wsA.Close()
			fmt.Fprintf(os.Stderr, "OK (session open)\n")

			fmt.Fprintf(os.Stderr, "Connecting to %s... ", epB)
			wsB, err := client.DialWebSocket(epB, appCtx.Client.AuthToken)
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL\n")
				return err
			}
			defer wsB.Close()
			fmt.Fprintf(os.Stderr, "OK (session open)\n")

			// Create topic on A
			fmt.Fprintf(os.Stderr, "Creating topic %q on %s... ", topicName, epA)
			id, err := wsA.Send("create-topicName", map[string]interface{}{
				"name": topicName,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL\n")
				return err
			}
			_, err = wsA.WaitResponse(id, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL\n")
				return err
			}
			fmt.Fprintf(os.Stderr, "OK\n")

			// Subscribe on B
			fmt.Fprintf(os.Stderr, "Subscribing on %s... ", epB)
			id, err = wsB.Send("subscribe", map[string]interface{}{
				"topicName": topicName,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL\n")
				return err
			}
			_, err = wsB.WaitResponse(id, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL\n")
				return err
			}
			fmt.Fprintf(os.Stderr, "OK (confirmed)\n")

			// Publish from A (fire-and-forget, server doesn't ack success)
			publishTime := time.Now()
			fmt.Fprintf(os.Stderr, "Publishing from %s... ", epA)
			_, err = wsA.Send("new-message", map[string]interface{}{
				"topicName": topicName,
				"message":   map[string]interface{}{"verify": true, "ts": publishTime.UnixMilli()},
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL\n")
				return err
			}
			fmt.Fprintf(os.Stderr, "OK\n")

			// Wait for delivery on B
			fmt.Fprintf(os.Stderr, "Waiting for delivery on %s... ", epB)
			msg, err := wsB.ReadMessageTimeout(5 * time.Second)
			if msg == nil {
				fmt.Fprintf(os.Stderr, "TIMEOUT (5s)\n")
				fmt.Fprintf(os.Stderr, "Cross-node PubSub: FAIL\n")
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				fmt.Fprintf(os.Stderr, "Cross-node PubSub: FAIL\n")
			} else {
				d := time.Since(publishTime)
				fmt.Fprintf(os.Stderr, "OK (received in %dms)\n", d.Milliseconds())
				fmt.Fprintf(os.Stderr, "Cross-node PubSub: PASS\n")
			}

			// Cleanup: destroy topic
			wsA.Send("destroy-topicName", map[string]interface{}{"name": topicName})

			return nil
		},
	}
}
