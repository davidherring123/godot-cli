package release_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/davidherring123/godot-cli/internal/release"
)

const canonicalAddonDir = "../../internal/addon/godot_cli"

var fixedTime = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

func TestAddonEntriesMatchCanonicalTree(t *testing.T) {
	entries, err := release.AddonEntries(canonicalAddonDir)
	if err != nil {
		t.Fatalf("listing addon entries: %v", err)
	}

	got := make([]string, 0, len(entries))

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name, release.AddonPrefix+"/") {
			t.Fatalf("entry %q is outside %q", entry.Name, release.AddonPrefix)
		}

		if entry.Mode.Perm() != 0o644 {
			t.Fatalf("entry %q has mode %o; want 644", entry.Name, entry.Mode.Perm())
		}

		if entry.Source == "" {
			t.Fatalf("entry %q has no source", entry.Name)
		}

		got = append(got, entry.Name)
	}

	want := canonicalNames(t, canonicalAddonDir)

	sort.Strings(got)

	if !slices.Equal(got, want) {
		t.Fatalf("addon entries differ from canonical files:\ngot  %v\nwant %v", got, want)
	}

	if !slices.Contains(got, release.AddonPrefix+"/plugin.cfg") {
		t.Fatal("plugin.cfg is missing from addon entries")
	}
}

func TestHostBinaryLayout(t *testing.T) {
	cases := []struct {
		target release.Target
		want   string
	}{
		{release.Target{OS: "darwin", Arch: "arm64"}, "addons/godot_cli/bin/macos-arm64/godot-mcp-host"},
		{release.Target{OS: "darwin", Arch: "amd64"}, "addons/godot_cli/bin/macos-x86_64/godot-mcp-host"},
		{release.Target{OS: "linux", Arch: "amd64"}, "addons/godot_cli/bin/linux-x86_64/godot-mcp-host"},
		{release.Target{OS: "linux", Arch: "arm64"}, "addons/godot_cli/bin/linux-arm64/godot-mcp-host"},
		{release.Target{OS: "windows", Arch: "amd64"}, "addons/godot_cli/bin/windows-x86_64/godot-mcp-host.exe"},
	}

	if len(release.DefaultTargets) != len(cases) {
		t.Fatalf("default target count is %d; want %d", len(release.DefaultTargets), len(cases))
	}

	for _, testCase := range cases {
		if got := testCase.target.HostEntryName(); got != testCase.want {
			t.Fatalf("%s host path is %q; want %q", testCase.target, got, testCase.want)
		}
	}

	seen := make(map[string]bool)

	for _, target := range release.DefaultTargets {
		if seen[target.String()] {
			t.Fatalf("duplicate default target %s", target)
		}

		seen[target.String()] = true
	}
}

func TestZipArchiveLayoutAndModes(t *testing.T) {
	entries := releaseEntries(t)
	data := writeZip(t, entries, fixedTime)

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading zip: %v", err)
	}

	modes := make(map[string]fs.FileMode, len(reader.File))

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, release.AddonPrefix+"/") {
			t.Fatalf("zip entry %q is outside %q", file.Name, release.AddonPrefix)
		}

		modes[file.Name] = file.Mode().Perm()
	}

	assertLayoutModes(t, modes)

	for _, target := range release.DefaultTargets {
		if mode, ok := modes[target.HostEntryName()]; !ok || mode != 0o755 {
			t.Fatalf("zip host %q has mode %o; want 755", target.HostEntryName(), mode)
		}
	}
}

func TestTarArchiveLayoutAndModes(t *testing.T) {
	entries := releaseEntries(t)
	data := writeTarGz(t, entries, fixedTime)

	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("reading gzip: %v", err)
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	reader := tar.NewReader(gzipReader)
	modes := make(map[string]fs.FileMode)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}

		if !strings.HasPrefix(header.Name, release.AddonPrefix+"/") {
			t.Fatalf("tar entry %q is outside %q", header.Name, release.AddonPrefix)
		}

		if header.Typeflag != tar.TypeReg {
			t.Fatalf("tar entry %q is not a regular file", header.Name)
		}

		modes[header.Name] = fs.FileMode(header.Mode).Perm()
	}

	assertLayoutModes(t, modes)

	for _, target := range release.DefaultTargets {
		if mode, ok := modes[target.HostEntryName()]; !ok || mode != 0o755 {
			t.Fatalf("tar host %q has mode %o; want 755", target.HostEntryName(), mode)
		}
	}
}

func TestArchivesAreDeterministic(t *testing.T) {
	entries := releaseEntries(t)
	reversed := append([]release.ArchiveEntry(nil), entries...)
	slices.Reverse(reversed)

	firstTar := writeTarGz(t, entries, fixedTime)
	secondTar := writeTarGz(t, reversed, fixedTime)

	if !bytes.Equal(firstTar, secondTar) {
		t.Fatal("tar.gz archives differ for the same inputs")
	}

	firstZip := writeZip(t, entries, fixedTime)
	secondZip := writeZip(t, reversed, fixedTime)

	if !bytes.Equal(firstZip, secondZip) {
		t.Fatal("zip archives differ for the same inputs")
	}
}

func releaseEntries(t *testing.T) []release.ArchiveEntry {
	t.Helper()

	entries, err := release.AddonEntries(canonicalAddonDir)
	if err != nil {
		t.Fatalf("listing addon entries: %v", err)
	}

	for _, target := range release.DefaultTargets {
		entries = append(entries, release.ArchiveEntry{
			Name: target.HostEntryName(),
			Mode: 0o755,
			Data: []byte("host binary for " + target.String()),
		})
	}

	return entries
}

func assertLayoutModes(t *testing.T, modes map[string]fs.FileMode) {
	t.Helper()

	if mode := modes[release.AddonPrefix+"/plugin.cfg"]; mode != 0o644 {
		t.Fatalf("plugin.cfg has mode %o; want 644", mode)
	}

	for _, target := range release.DefaultTargets {
		if _, ok := modes[target.HostEntryName()]; !ok {
			t.Fatalf("archive is missing host binary %q", target.HostEntryName())
		}
	}
}

func canonicalNames(t *testing.T, directory string) []string {
	t.Helper()

	var names []string

	err := filepath.WalkDir(directory, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		switch entry.Name() {
		case ".DS_Store", "Thumbs.db", ".godot-cli-managed":
			return nil
		}

		relative, err := filepath.Rel(directory, filePath)
		if err != nil {
			return err
		}

		names = append(names, path.Join(release.AddonPrefix, filepath.ToSlash(relative)))

		return nil
	})
	if err != nil {
		t.Fatalf("walking canonical addon: %v", err)
	}

	sort.Strings(names)

	return names
}

func writeTarGz(t *testing.T, entries []release.ArchiveEntry, modified time.Time) []byte {
	t.Helper()

	var buffer bytes.Buffer

	if err := release.WriteTarGz(&buffer, entries, modified); err != nil {
		t.Fatalf("writing tar.gz: %v", err)
	}

	return buffer.Bytes()
}

func writeZip(t *testing.T, entries []release.ArchiveEntry, modified time.Time) []byte {
	t.Helper()

	var buffer bytes.Buffer

	if err := release.WriteZip(&buffer, entries, modified); err != nil {
		t.Fatalf("writing zip: %v", err)
	}

	return buffer.Bytes()
}

func TestAddonEntriesRejectsMissingAddon(t *testing.T) {
	_, err := release.AddonEntries(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("AddonEntries succeeded for a missing addon")
	}
}

func TestAddonEntriesReadsSourceFiles(t *testing.T) {
	root := t.TempDir()
	addonDir := filepath.Join(root, "godot_cli")

	if err := os.MkdirAll(filepath.Join(addonDir, "core"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(addonDir, "plugin.cfg"), "plugin")
	writeFile(t, filepath.Join(addonDir, "core", "service.gd"), "script")
	writeFile(t, filepath.Join(addonDir, ".DS_Store"), "junk")

	entries, err := release.AddonEntries(addonDir)
	if err != nil {
		t.Fatalf("listing addon entries: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}

	want := []string{
		release.AddonPrefix + "/core/service.gd",
		release.AddonPrefix + "/plugin.cfg",
	}

	if !slices.Equal(names, want) {
		t.Fatalf("addon names are %v; want %v", names, want)
	}
}

func writeFile(t *testing.T, filePath, contents string) {
	t.Helper()

	if err := os.WriteFile(filePath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
