// Package release builds reproducible, installable godot-cli addon archives.
package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultAddonDir is the canonical addon source, relative to the repository root.
	DefaultAddonDir = "internal/addon/godot_cli"

	hostBinaryName     = "godot-mcp-host"
	hostCommandPackage = "./cmd/godot-cli"
)

// Target is one GOOS/GOARCH pair that receives a bundled host binary.
type Target struct {
	OS   string
	Arch string
}

// DefaultTargets lists the platforms shipped in a release.
var DefaultTargets = []Target{
	{OS: "darwin", Arch: "arm64"},
	{OS: "darwin", Arch: "amd64"},
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "windows", Arch: "amd64"},
}

func (target Target) String() string {
	return target.OS + "-" + target.Arch
}

// HostBinaryName returns the host executable name for the target.
func (target Target) HostBinaryName() string {
	if target.OS == "windows" {
		return hostBinaryName + ".exe"
	}

	return hostBinaryName
}

// HostEntryName returns the host executable path inside an addon archive.
func (target Target) HostEntryName() string {
	return path.Join(AddonPrefix, "bin", target.String(), target.HostBinaryName())
}

// Options configures a release build.
type Options struct {
	// Root is the repository root. It defaults to the current directory.
	Root string
	// OutputDir receives the archives and SHA256SUMS. It defaults to <Root>/dist.
	OutputDir string
	// Version is embedded in the host binaries and used in archive names.
	Version string
	// Targets defaults to DefaultTargets.
	Targets []Target
	// SourceDateEpoch normalizes archive timestamps. Zero uses 1980-01-01 UTC.
	SourceDateEpoch time.Time
	// GoCommand defaults to "go".
	GoCommand string
}

// Build cross-compiles the host for every target and writes per-platform and
// all-platform addon archives plus a SHA256SUMS file. It returns the written paths.
func Build(options Options) ([]string, error) {
	root, err := repositoryRoot(options.Root)
	if err != nil {
		return nil, err
	}

	version, err := normalizeVersion(options.Version)
	if err != nil {
		return nil, err
	}

	targets := options.Targets
	if len(targets) == 0 {
		targets = DefaultTargets
	}

	outputDir, err := outputDirectory(root, options.OutputDir)
	if err != nil {
		return nil, err
	}

	addonEntries, err := AddonEntries(filepath.Join(root, filepath.FromSlash(DefaultAddonDir)))
	if err != nil {
		return nil, err
	}

	scratch, err := os.MkdirTemp("", "godot-cli-addon-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(scratch)
	}()

	goCommand := options.GoCommand
	if goCommand == "" {
		goCommand = "go"
	}

	binaries := make(map[string]string, len(targets))

	for _, target := range targets {
		output := filepath.Join(scratch, target.String(), target.HostBinaryName())

		if err := buildHost(goCommand, root, version, target, output); err != nil {
			return nil, err
		}

		binaries[target.String()] = output
	}

	artifacts := make([]string, 0, len(targets)+2)

	for _, target := range targets {
		entries := append([]ArchiveEntry(nil), addonEntries...)
		entries = append(entries, hostEntry(target, binaries[target.String()]))

		archive := filepath.Join(outputDir, targetArchiveName(version, target))
		if err := writeArchive(archive, target.OS == "windows", entries, options.SourceDateEpoch); err != nil {
			return nil, err
		}

		artifacts = append(artifacts, archive)
	}

	universalEntries := append([]ArchiveEntry(nil), addonEntries...)
	for _, target := range targets {
		universalEntries = append(universalEntries, hostEntry(target, binaries[target.String()]))
	}

	universal := filepath.Join(outputDir, universalArchiveName(version))
	if err := writeArchive(universal, true, universalEntries, options.SourceDateEpoch); err != nil {
		return nil, err
	}

	artifacts = append(artifacts, universal)

	checksums, err := writeChecksums(outputDir, artifacts)
	if err != nil {
		return nil, err
	}

	artifacts = append(artifacts, checksums)
	sort.Strings(artifacts)

	return artifacts, nil
}

// AddonEntries lists the canonical addon files rooted at addons/godot_cli.
func AddonEntries(addonDir string) ([]ArchiveEntry, error) {
	pluginPath := filepath.Join(addonDir, "plugin.cfg")

	info, err := os.Stat(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("canonical addon is missing plugin.cfg: %w", err)
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("canonical addon plugin.cfg is not a regular file: %s", pluginPath)
	}

	var entries []ArchiveEntry

	err = filepath.WalkDir(addonDir, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if filePath == addonDir {
			return nil
		}

		relative, err := filepath.Rel(addonDir, filePath)
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		switch entry.Name() {
		case ".DS_Store", "Thumbs.db", ".godot-cli-managed":
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("addon contains unsupported file %q", filePath)
		}

		entries = append(entries, ArchiveEntry{
			Name:   path.Join(AddonPrefix, filepath.ToSlash(relative)),
			Mode:   addonFileMode,
			Source: filePath,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("canonical addon directory %q is empty", addonDir)
	}

	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name < entries[right].Name
	})

	return entries, nil
}

func repositoryRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(filepath.Join(absolute, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("root %q is not the repository root: %w", absolute, err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("root %q has a go.mod directory", absolute)
	}

	return absolute, nil
}

func outputDirectory(root, output string) (string, error) {
	if output == "" {
		output = filepath.Join(root, "dist")
	}

	absolute, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return "", err
	}

	return absolute, nil
}

func normalizeVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "dev"
	}

	for _, character := range version {
		alphaNumeric := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		if alphaNumeric || strings.ContainsRune("._+-", character) {
			continue
		}

		return "", fmt.Errorf(
			"invalid version %q: use letters, digits, '.', '-', '_' or '+'",
			version,
		)
	}

	return version, nil
}

func buildHost(goCommand, root, version string, target Target, output string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}

	linkerFlags := "-X main.VERSION=" + version + " -buildid="
	arguments := []string{
		"build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags", linkerFlags,
		"-o", output,
		hostCommandPackage,
	}

	command := exec.Command(goCommand, arguments...)
	command.Dir = root
	command.Env = buildEnvironment(os.Environ(), target)

	var stderr bytes.Buffer
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("building %s: %w: %s", target, err, detail)
		}

		return fmt.Errorf("building %s: %w", target, err)
	}

	info, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("built binary for %s is missing: %w", target, err)
	}

	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("built binary for %s is empty", target)
	}

	return nil
}

func buildEnvironment(environment []string, target Target) []string {
	filtered := make([]string, 0, len(environment)+3)

	for _, variable := range environment {
		name, _, _ := strings.Cut(variable, "=")

		switch name {
		case "CGO_ENABLED", "GOARCH", "GOOS":
			continue
		}

		filtered = append(filtered, variable)
	}

	return append(
		filtered,
		"CGO_ENABLED=0",
		"GOOS="+target.OS,
		"GOARCH="+target.Arch,
	)
}

func hostEntry(target Target, source string) ArchiveEntry {
	return ArchiveEntry{
		Name:   target.HostEntryName(),
		Mode:   executableFileMode,
		Source: source,
	}
}

func targetArchiveName(version string, target Target) string {
	name := "godot-cli-addon-" + version + "-" + target.String()
	if target.OS == "windows" {
		return name + ".zip"
	}

	return name + ".tar.gz"
}

func universalArchiveName(version string) string {
	return "godot-cli-addon-" + version + ".zip"
}

func writeArchive(filePath string, compressedZip bool, entries []ArchiveEntry, modified time.Time) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	if compressedZip {
		err = WriteZip(file, entries, modified)
	} else {
		err = WriteTarGz(file, entries, modified)
	}

	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}

	if closeErr != nil {
		return fmt.Errorf("closing %s: %w", filePath, closeErr)
	}

	return nil
}

func writeChecksums(directory string, artifacts []string) (string, error) {
	sort.Strings(artifacts)

	var checksums strings.Builder

	for _, artifact := range artifacts {
		digest, err := fileDigest(artifact)
		if err != nil {
			return "", err
		}

		fmt.Fprintf(&checksums, "%s  %s\n", digest, filepath.Base(artifact))
	}

	checksumPath := filepath.Join(directory, "SHA256SUMS")
	if err := os.WriteFile(checksumPath, []byte(checksums.String()), 0o644); err != nil {
		return "", err
	}

	return checksumPath, nil
}

func fileDigest(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	hash := sha256.New()

	if _, err := io.Copy(hash, file); err != nil {
		_ = file.Close()

		return "", err
	}

	if err := file.Close(); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
