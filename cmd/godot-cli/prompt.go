package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func confirm(message string) bool {
	fmt.Printf("%s [Y/n] ", message)

	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))

	return input == "" || input == "y" || input == "yes"
}
