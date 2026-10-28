// Package integration manages project-scoped MCP client configuration for the
// bundled godot-mcp-host.
package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/davidherring123/godot-mcp/internal/addon"
	"github.com/davidherring123/godot-mcp/internal/agent/skill"
	"github.com/davidherring123/godot-mcp/internal/release"
)

// ServerName is the MCP server name written to client configuration.
const ServerName = "godot"

// Client identifies a supported AI client.
type Client string

const (
	ClientOpenCode Client = "opencode"
	ClientClaude   Client = "claude"
	ClientCodex    Client = "codex"
)

var supportedClients = []Client{ClientOpenCode, ClientClaude, ClientCodex}

// Options configures integration inspection or installation. HostPath is
// optional; when empty it resolves the running host or the project's bundled
// host.
type Options struct {
	ProjectRoot string
	HostPath    string
}

// ClientStatus describes one client's project configuration and skill.
type ClientStatus struct {
	Client         Client   `json:"client"`
	ConfigPath     string   `json:"config_path"`
	ConfigExists   bool     `json:"config_exists"`
	Configured     bool     `json:"configured"`
	UpToDate       bool     `json:"up_to_date"`
	Command        []string `json:"command,omitempty"`
	SkillPath      string   `json:"skill_path"`
	SkillInstalled bool     `json:"skill_installed"`
	SkillCurrent   bool     `json:"skill_current"`
}

// StatusReport is the machine-readable result of integration inspection.
type StatusReport struct {
	ProjectRoot string         `json:"project_root"`
	HostPath    string         `json:"host_path"`
	Clients     []ClientStatus `json:"clients"`
}

// ClientInstall describes the files managed for one client installation.
type ClientInstall struct {
	Client        Client `json:"client"`
	ConfigPath    string `json:"config_path"`
	ConfigChanged bool   `json:"config_changed"`
	SkillPath     string `json:"skill_path"`
	SkillChanged  bool   `json:"skill_changed"`
}

// InstallReport is the machine-readable result of integration installation.
type InstallReport struct {
	ProjectRoot string          `json:"project_root"`
	HostPath    string          `json:"host_path"`
	Clients     []ClientInstall `json:"clients"`
}

// SupportedClients returns the clients managed by this package.
func SupportedClients() []Client {
	return append([]Client(nil), supportedClients...)
}

// ParseClients validates an install target. "all" selects every client.
func ParseClients(value string) ([]Client, error) {
	switch Client(strings.ToLower(strings.TrimSpace(value))) {
	case "all":
		return SupportedClients(), nil
	case ClientOpenCode:
		return []Client{ClientOpenCode}, nil
	case ClientClaude:
		return []Client{ClientClaude}, nil
	case ClientCodex:
		return []Client{ClientCodex}, nil
	default:
		return nil, fmt.Errorf(
			"unsupported client %q (supported: opencode, claude, codex, all)",
			value,
		)
	}
}

// ResolveHostPath returns the absolute executable that clients should launch.
// A running godot-mcp-host is used directly; a regular CLI prefers the
// project's bundled host, then falls back to the running executable so the
// connect entrypoint remains usable in development builds.
func ResolveHostPath(projectRoot string) (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating the godot-mcp executable: %w", err)
	}

	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return "", fmt.Errorf("resolving the godot-mcp executable: %w", err)
	}

	if isHostExecutable(executablePath) {
		return executablePath, nil
	}

	bundledPath := bundledHostPath(projectRoot)

	if info, err := os.Stat(bundledPath); err == nil && info.Mode().IsRegular() {
		return bundledPath, nil
	}

	return executablePath, nil
}

// Status inspects the configured clients without modifying files.
func Status(options Options) (StatusReport, error) {
	projectRoot, hostPath, err := resolveOptions(options)
	if err != nil {
		return StatusReport{}, err
	}

	report := StatusReport{
		ProjectRoot: projectRoot,
		HostPath:    hostPath,
		Clients:     make([]ClientStatus, 0, len(supportedClients)),
	}

	for _, client := range supportedClients {
		status, err := inspectClient(projectRoot, hostPath, client)
		if err != nil {
			return StatusReport{}, err
		}

		report.Clients = append(report.Clients, status)
	}

	return report, nil
}

// Install merges the selected clients' project configuration and installs the
// portable skill. Existing unrelated configuration is preserved.
func Install(options Options, clients []Client) (InstallReport, error) {
	if len(clients) == 0 {
		return InstallReport{}, fmt.Errorf("at least one client is required")
	}

	projectRoot, hostPath, err := resolveOptions(options)
	if err != nil {
		return InstallReport{}, err
	}

	report := InstallReport{
		ProjectRoot: projectRoot,
		HostPath:    hostPath,
		Clients:     make([]ClientInstall, 0, len(clients)),
	}

	seen := make(map[Client]bool, len(clients))

	for _, client := range clients {
		if seen[client] {
			continue
		}

		seen[client] = true

		install, err := installClient(projectRoot, hostPath, client)
		if err != nil {
			return InstallReport{}, err
		}

		report.Clients = append(report.Clients, install)
	}

	return report, nil
}

func resolveOptions(options Options) (string, string, error) {
	if strings.TrimSpace(options.ProjectRoot) == "" {
		return "", "", fmt.Errorf("project root is required")
	}

	projectRoot, err := filepath.Abs(options.ProjectRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolving project root: %w", err)
	}

	info, err := os.Stat(projectRoot)
	if err != nil {
		return "", "", fmt.Errorf("reading project root %q: %w", projectRoot, err)
	}

	if !info.IsDir() {
		return "", "", fmt.Errorf("project root %q is not a directory", projectRoot)
	}

	hostPath := options.HostPath
	if hostPath == "" {
		hostPath, err = ResolveHostPath(projectRoot)
		if err != nil {
			return "", "", err
		}
	} else {
		hostPath, err = filepath.Abs(hostPath)
		if err != nil {
			return "", "", fmt.Errorf("resolving host path: %w", err)
		}
	}

	return projectRoot, hostPath, nil
}

func inspectClient(projectRoot, hostPath string, client Client) (ClientStatus, error) {
	switch client {
	case ClientOpenCode:
		return inspectOpenCode(projectRoot, hostPath)
	case ClientClaude:
		return inspectClaude(projectRoot, hostPath)
	case ClientCodex:
		return inspectCodex(projectRoot, hostPath)
	default:
		return ClientStatus{}, fmt.Errorf("unsupported client %q", client)
	}
}

func installClient(projectRoot, hostPath string, client Client) (ClientInstall, error) {
	var (
		configPath    string
		configChanged bool
		skillPath     string
		skillChanged  bool
		err           error
	)

	switch client {
	case ClientOpenCode:
		configPath, configChanged, err = installOpenCode(projectRoot, hostPath)
		skillPath = skill.PortablePath(projectRoot)
	case ClientClaude:
		configPath, configChanged, err = installClaude(projectRoot, hostPath)
		skillPath = skill.ClaudePath(projectRoot)
	case ClientCodex:
		configPath, configChanged, err = installCodex(projectRoot, hostPath)
		skillPath = skill.PortablePath(projectRoot)
	default:
		return ClientInstall{}, fmt.Errorf("unsupported client %q", client)
	}

	if err != nil {
		return ClientInstall{}, err
	}

	skillChanged, err = skill.InstallAt(skillPath)
	if err != nil {
		return ClientInstall{}, fmt.Errorf("installing skill: %w", err)
	}

	return ClientInstall{
		Client:        client,
		ConfigPath:    configPath,
		ConfigChanged: configChanged,
		SkillPath:     skillPath,
		SkillChanged:  skillChanged,
	}, nil
}

func skillStatus(projectRoot string, client Client) (string, bool, bool, error) {
	skillPath := skill.PortablePath(projectRoot)

	if client == ClientClaude {
		skillPath = skill.ClaudePath(projectRoot)
	}

	content, err := os.ReadFile(skillPath)
	if os.IsNotExist(err) {
		return skillPath, false, false, nil
	}

	if err != nil {
		return skillPath, false, false, fmt.Errorf("reading skill %q: %w", skillPath, err)
	}

	return skillPath, true, string(content) == string(skill.Content()), nil
}

func expectedCommand(projectRoot, hostPath string) []string {
	return []string{hostPath, "connect", "--project", projectRoot}
}

func bundledHostPath(projectRoot string) string {
	target := release.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}

	return filepath.Join(
		addon.Path(projectRoot),
		"bin",
		target.BundleString(),
		target.HostBinaryName(),
	)
}

func isHostExecutable(executablePath string) bool {
	name := strings.ToLower(filepath.Base(executablePath))

	return name == "godot-mcp-host" || name == "godot-mcp-host.exe"
}
