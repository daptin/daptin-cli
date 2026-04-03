package cmd

import "strings"

// commands and subcommands known to the app — used to identify the boundary
// between global flags and command args.
var knownCommands = map[string]bool{
	"context": true, "list": true, "get": true, "create": true,
	"update": true, "delete": true, "related": true, "describe": true,
	"execute": true, "help": true,
}

// Only commands that actually have subcommands, mapped to their subcommand names.
var commandSubcommands = map[string]map[string]bool{
	"context":    {"set": true, "add": true, "list": true},
	"describe":   {"table": true, "action": true},
	"permission": {"decode": true, "encode": true},
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

	// Start of the command's own args (after command + optional subcommand)
	argsStart := cmdIdx + 1
	cmdName := args[cmdIdx]
	if subs, ok := commandSubcommands[cmdName]; ok && argsStart < len(args) && subs[args[argsStart]] {
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
			if !strings.Contains(arg, "=") && i+1 < len(commandArgs) && !strings.HasPrefix(commandArgs[i+1], "-") && !strings.Contains(commandArgs[i+1], "=") {
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
