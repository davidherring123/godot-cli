package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	// AddonPrefix is the path where Godot projects install the addon.
	AddonPrefix = "addons/godot_cli"

	addonFileMode      = fs.FileMode(0o644)
	executableFileMode = fs.FileMode(0o755)
)

// earliestArchiveTime keeps ZIP's MS-DOS timestamp fields in range.
var earliestArchiveTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// ArchiveEntry describes one regular file in a release archive. Exactly one of
// Data or Source should be set; Source streams a file from disk.
type ArchiveEntry struct {
	Name   string
	Mode   fs.FileMode
	Data   []byte
	Source string
}

// WriteTarGz writes a deterministic gzip-compressed tar archive. Entry names
// are sorted so callers can supply files in any order.
func WriteTarGz(w io.Writer, entries []ArchiveEntry, modified time.Time) error {
	normalized, err := normalizedEntries(entries)
	if err != nil {
		return err
	}

	modified = normalizeArchiveTime(modified)

	gzipWriter, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}

	gzipWriter.ModTime = modified
	gzipWriter.OS = 255

	tarWriter := tar.NewWriter(gzipWriter)

	for _, entry := range normalized {
		size, err := entry.size()
		if err != nil {
			return fmt.Errorf("sizing %q: %w", entry.Name, err)
		}

		header := &tar.Header{
			Name:     entry.Name,
			Mode:     int64(entry.Mode.Perm()),
			Size:     size,
			ModTime:  modified,
			Typeflag: tar.TypeReg,
			Format:   tar.FormatUSTAR,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("writing header for %q: %w", entry.Name, err)
		}

		if err := copyEntry(tarWriter, entry); err != nil {
			return fmt.Errorf("writing %q: %w", entry.Name, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		return err
	}

	return gzipWriter.Close()
}

// WriteZip writes a deterministic ZIP archive. Entry modes are stored as Unix
// permissions so executable host binaries keep their executable bit.
func WriteZip(w io.Writer, entries []ArchiveEntry, modified time.Time) error {
	normalized, err := normalizedEntries(entries)
	if err != nil {
		return err
	}

	modified = normalizeArchiveTime(modified)

	zipWriter := zip.NewWriter(w)

	for _, entry := range normalized {
		header := &zip.FileHeader{
			Name:     entry.Name,
			Method:   zip.Deflate,
			Modified: modified,
		}
		header.SetMode(entry.Mode.Perm())

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("creating %q: %w", entry.Name, err)
		}

		if err := copyEntry(writer, entry); err != nil {
			return fmt.Errorf("writing %q: %w", entry.Name, err)
		}
	}

	return zipWriter.Close()
}

func normalizedEntries(entries []ArchiveEntry) ([]ArchiveEntry, error) {
	if len(entries) == 0 {
		return nil, errors.New("archive has no files")
	}

	normalized := append([]ArchiveEntry(nil), entries...)

	sort.Slice(normalized, func(left, right int) bool {
		return normalized[left].Name < normalized[right].Name
	})

	for index, entry := range normalized {
		if err := validateArchiveName(entry.Name); err != nil {
			return nil, err
		}

		if index > 0 && entry.Name == normalized[index-1].Name {
			return nil, fmt.Errorf("duplicate archive path %q", entry.Name)
		}
	}

	return normalized, nil
}

func validateArchiveName(name string) error {
	if name == "" {
		return errors.New("archive entry has an empty name")
	}

	if strings.Contains(name, "\\") {
		return fmt.Errorf("archive entry %q must use forward slashes", name)
	}

	if path.IsAbs(name) {
		return fmt.Errorf("archive entry %q must be relative", name)
	}

	cleaned := path.Clean(name)
	if cleaned != name || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("archive entry %q is not a clean relative path", name)
	}

	return nil
}

func normalizeArchiveTime(modified time.Time) time.Time {
	if modified.IsZero() || modified.Before(earliestArchiveTime) {
		return earliestArchiveTime
	}

	return modified.UTC()
}

func (entry ArchiveEntry) size() (int64, error) {
	if entry.Source == "" {
		return int64(len(entry.Data)), nil
	}

	info, err := os.Stat(entry.Source)
	if err != nil {
		return 0, err
	}

	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("%q is not a regular file", entry.Source)
	}

	return info.Size(), nil
}

func (entry ArchiveEntry) open() (io.ReadCloser, error) {
	if entry.Source == "" {
		return io.NopCloser(bytes.NewReader(entry.Data)), nil
	}

	return os.Open(entry.Source)
}

func copyEntry(w io.Writer, entry ArchiveEntry) error {
	reader, err := entry.open()
	if err != nil {
		return err
	}

	if _, err := io.Copy(w, reader); err != nil {
		_ = reader.Close()

		return err
	}

	return reader.Close()
}
