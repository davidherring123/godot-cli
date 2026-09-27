package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func Confirm(message string) (bool, error) {
	fmt.Printf("%s [Y/n] ", message)

	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(strings.ToLower(input))

	return input == "" || input == "y" || input == "yes", nil
}
