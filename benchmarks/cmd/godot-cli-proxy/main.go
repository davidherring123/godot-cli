package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	recordInvocation()

	if os.Getenv("GODOT_CLI_BLOCK") == "1" {
		fmt.Fprintln(os.Stderr, "godot-cli is disabled for the direct-files benchmark approach")
		os.Exit(126)
	}
	realCLI := os.Getenv("GODOT_CLI_REAL")
	if realCLI == "" {
		fmt.Fprintln(os.Stderr, "benchmark proxy requires GODOT_CLI_REAL")
		os.Exit(127)
	}
	command := exec.Command(realCLI, os.Args[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(127)
	}
}

func recordInvocation() {
	marker := os.Getenv("GODOT_CLI_MARKER")
	if marker == "" {
		return
	}
	file, err := os.OpenFile(marker, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = fmt.Fprintln(file, strings.Join(os.Args[1:], " "))
}
