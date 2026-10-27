// Command package-addon builds reproducible godot-cli addon release archives.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/davidherring123/godot-cli/internal/release"
)

func main() {
	version := flag.String("version", "dev", "version embedded in binaries and archive names")
	root := flag.String("root", ".", "repository root")
	output := flag.String("out", "dist", "output directory for archives")
	flag.Parse()

	epoch, err := sourceDateEpoch()
	if err != nil {
		fail(err)
	}

	artifacts, err := release.Build(release.Options{
		Root:            *root,
		OutputDir:       *output,
		Version:         *version,
		SourceDateEpoch: epoch,
	})
	if err != nil {
		fail(err)
	}

	for _, artifact := range artifacts {
		fmt.Println(artifact)
	}
}

func sourceDateEpoch() (time.Time, error) {
	raw := os.Getenv("SOURCE_DATE_EPOCH")
	if raw == "" {
		return time.Time{}, nil
	}

	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds < 0 {
		return time.Time{}, fmt.Errorf("invalid SOURCE_DATE_EPOCH %q", raw)
	}

	return time.Unix(seconds, 0).UTC(), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "package-addon:", err)
	os.Exit(1)
}
