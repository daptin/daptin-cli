package cmd

import "strings"

// commands and subcommands known to the app — used to identify the boundary
// between global flags and command args.
var knownCommands = map[string]bool{
	"context": true, "list": true, "get": true, "create": true,
	"update": true, "delete": true, "related": true, "describe": true,
	"execute": true, "help": true, "relate": true, "unrelate": true,
	"permission": true, "storage": true, "asset": true, "oauth": true,
	"integration": true, "table": true,
}

// Only commands that actually have subcommands, mapped to their subcommand names.
var commandSubcommands = map[string]map[string]bool{
	"context":    {"set": true, "add": true, "list": true},
	"describe":   {"table": true, "action": true},
	"permission": {"decode": true, "encode": true},
	"table":      {"defaults": true},
	"defaults":   {"get": true, "set": true, "group": true, "ensure": true},
	"group":      {"add": true},
	"storage": {
		"add": true, "list": true, "remove": true, "ls": true,
		"upload": true, "download": true, "mv": true, "rm": true, "mkdir": true,
	},
	"asset":   {"upload": true, "list": true},
	"oauth":   {"connect": true, "login-url": true, "tokens": true},
	"connect": {"create": true, "list": true},
	"tokens":  {"list": true},
	"integration": {
		"validate-spec": true, "import": true, "install": true, "list": true,
		"operations": true, "describe": true, "execute": true,
	},
}

var valueFlags = map[string]bool{
	"--config": true, "-c": true,
	"--output": true, "-o": true,
	"--endpoint":                        true,
	"--columns":                         true,
	"--page-size":                       true,
	"--page":                            true,
	"--sort":                            true,
	"--filter":                          true,
	"--include":                         true,
	"--reference-id":                    true,
	"--type":                            true,
	"--provider":                        true,
	"--store-provider":                  true,
	"--access-key":                      true,
	"--secret-key":                      true,
	"--bucket":                          true,
	"--root-path":                       true,
	"--credential":                      true,
	"--param":                           true,
	"--spec-file":                       true,
	"--spec-url":                        true,
	"--spec-format":                     true,
	"--spec-language":                   true,
	"--auth":                            true,
	"--oauth-connect":                   true,
	"--auth-spec-json":                  true,
	"--auth-spec-file":                  true,
	"--set-operation-transport":         true,
	"--set-operation-upstream-path":     true,
	"--set-operation-timeout-ms":        true,
	"--set-graphql-document":            true,
	"--set-graphql-document-file":       true,
	"--set-graphql-operation-name":      true,
	"--set-websocket-message-template":  true,
	"--set-websocket-response-selector": true,
	"--set-grpc-service":                true,
	"--set-grpc-method":                 true,
	"--grpc-descriptor-set":             true,
	"--grpc-proto":                      true,
	"--grpc-proto-path":                 true,
	"--protoc":                          true,
	"--oauth-token-id":                  true,
	"--credential-id":                   true,
	"--input-json":                      true,
	"--input-file":                      true,
	"--client-id":                       true,
	"--client-secret":                   true,
	"--client-secret-env":               true,
	"--client-secret-file":              true,
	"--scope":                           true,
	"--response-type":                   true,
	"--redirect-uri":                    true,
	"--auth-url":                        true,
	"--token-url":                       true,
	"--profile-url":                     true,
	"--profile-email-path":              true,
	"--pkce-challenge-method":           true,
	"--permission":                      true,
	"--group":                           true,
}

var boolFlags = map[string]bool{
	"--debug":       true,
	"--no-truncate": true,
	"--quiet":       true, "-q": true,
	"--interactive":         true,
	"--restart":             true,
	"--recursive":           true,
	"--spec-stdin":          true,
	"--validate":            true,
	"--disable":             true,
	"--update":              true,
	"--allow-login":         true,
	"--access-type-offline": true,
	"--pkce":                true,
	"--open":                true,
	"--help":                true, "-h": true,
	"--version": true, "-v": true,
}

// ReorderArgs moves --flags that appear after positional args to before them,
// so Go's flag.FlagSet.Parse() sees them.
//
// Pure function: args in, reordered args out.
//
// Strategy: find where the command's positional args begin, then partition
// everything after into flags and positional args, with flags first.
func ReorderArgs(args []string) []string {
	if len(args) <= 2 {
		return args
	}

	// Find the command boundary: skip binary name and global flags to find
	// the command name, then optionally a subcommand name.
	cmdIdx := findCommandIndex(args)
	if cmdIdx < 0 {
		return args
	}

	// Start of the command's own args (after command + optional subcommands)
	argsStart := cmdIdx + 1
	cmdName := args[cmdIdx]
	for {
		subs, ok := commandSubcommands[cmdName]
		if !ok || argsStart >= len(args) || !subs[args[argsStart]] {
			break
		}
		cmdName = args[argsStart]
		argsStart++
	}

	if argsStart >= len(args) {
		return args
	}

	// Partition the command args into flags and positional args.
	// A flag is anything starting with "--" (or "-" followed by a letter).
	// A flag's value is the next arg if it doesn't start with "-" and the
	// flag doesn't use "=" syntax.
	commandArgs := args[argsStart:]
	var flags []string
	var positional []string

	for i := 0; i < len(commandArgs); i++ {
		arg := commandArgs[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// If it's --flag=value, the value is included. Otherwise peek next.
			if flagConsumesNextValue(arg, commandArgs, i) {
				flags = append(flags, commandArgs[i+1])
				i++
			}
		} else {
			positional = append(positional, arg)
		}
	}

	// Rebuild: prefix + command [+ subcommand] + flags + positional
	result := make([]string, 0, len(args))
	result = append(result, args[:argsStart]...)
	result = append(result, flags...)
	result = append(result, positional...)
	return result
}

func flagConsumesNextValue(arg string, commandArgs []string, index int) bool {
	if strings.Contains(arg, "=") || index+1 >= len(commandArgs) {
		return false
	}
	if boolFlags[arg] {
		return false
	}

	next := commandArgs[index+1]
	if valueFlags[arg] {
		return !strings.HasPrefix(next, "--")
	}

	return !strings.HasPrefix(next, "-") && !strings.Contains(next, "=")
}

// findCommandIndex returns the index of the first known command in args,
// skipping the binary name and any global flags (--flag value or --flag=value or --bool-flag).
func findCommandIndex(args []string) int {
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			// Skip global flag value if it uses space-separated syntax
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && knownCommands[args[i+1]] == false {
				i++ // skip the value
			}
			continue
		}
		if knownCommands[arg] {
			return i
		}
		// Unknown non-flag arg before a command — bail out
		return -1
	}
	return -1
}
