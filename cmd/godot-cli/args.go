package main

import "fmt"

type Flags struct {
	JSON bool
}

type ParsedArgs struct {
	Positionals []string
	Flags       Flags
}

func parseArgs(args []string) (ParsedArgs, error) {
	var parsed ParsedArgs

	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			parsed.Flags.JSON = true

		default:
			if len(arg) >= 2 && arg[:2] == "--" {
				return ParsedArgs{}, fmt.Errorf("unknown flag: %s", arg)
			}

			parsed.Positionals = append(parsed.Positionals, arg)
		}
	}

	return parsed, nil
}

func HasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}

	return false
}
