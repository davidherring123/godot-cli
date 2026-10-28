package runtime

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const (
	runtimeDirName    = ".godot/godot-mcp/runtime"
	sessionsDirName   = "sessions"
	heartbeatTimeout  = 5 * time.Second
	readyGrace        = 20 * time.Second
	maxErrors         = 50
	maxErrorRunes     = 500
	maxLogTail        = 1 << 20
	defaultStartWait  = 20 * time.Second
	defaultStopWait   = 5 * time.Second
	captureTimeout    = 10 * time.Second
	killWait          = 2 * time.Second
	maxEmbeddedImage  = 16 << 20
	sessionRetention  = 7 * 24 * time.Hour
	startLockStaleFor = 2 * time.Minute
)

var (
	sessionIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{6}$`)
	errorLinePattern = regexp.MustCompile(`^(SCRIPT ERROR|USER ERROR|ERROR|FATAL):`)
)

type sessionMetadata struct {
	Session         string    `json:"session"`
	PID             int       `json:"pid"`
	ProcessIdentity string    `json:"process_identity,omitempty"`
	Scene           string    `json:"scene"`
	Headless        bool      `json:"headless"`
	StartedAt       time.Time `json:"started_at"`
}

type heartbeatReport struct {
	Frames   int   `json:"frames"`
	UptimeMS int64 `json:"uptime_ms"`
}

type readyReport struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	Scene         string `json:"scene"`
	DisplayServer string `json:"display_server"`
	Headless      bool   `json:"headless"`
	WindowSize    Size   `json:"window_size"`
	Frames        int    `json:"frames"`
	PID           int    `json:"pid"`
}

type exitReport struct {
	Reason   string `json:"reason"`
	Code     int    `json:"code"`
	Frames   int    `json:"frames"`
	UptimeMS int64  `json:"uptime_ms"`
}

type captureResponse struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Bytes   int64  `json:"bytes"`
	Frame   int    `json:"frames"`
}

type sessionInfo struct {
	metadata   sessionMetadata
	directory  string
	state      SessionState
	responsive bool
}

func runtimeRoot(projectRoot string) string {
	return filepath.Join(projectRoot, filepath.FromSlash(runtimeDirName))
}

func sessionsRoot(projectRoot string) string {
	return filepath.Join(runtimeRoot(projectRoot), sessionsDirName)
}

func validSessionID(session string) bool {
	return sessionIDPattern.MatchString(session)
}

func sessionDirectory(projectRoot string, session string) (string, error) {
	if !validSessionID(session) {
		return "", newError(
			string(apierr.CodeInvalidArgument),
			"invalid runtime session ID %q",
			session,
		)
	}

	directory := filepath.Join(sessionsRoot(projectRoot), session)

	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return "", newError(
			string(apierr.CodeNotFound),
			"runtime session %q was not found",
			session,
		)
	}

	return directory, nil
}

func newSessionID() string {
	return time.Now().UTC().Format("20060102T150405Z") + "-" + randomHex(3)
}

func randomHex(size int) string {
	random := make([]byte, size)

	if _, err := rand.Read(random); err != nil {
		now := time.Now().UnixNano()

		for index := range random {
			random[index] = byte(now >> (index * 8))
		}
	}

	return hex.EncodeToString(random)
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	temporary := path + ".tmp"

	if err := os.WriteFile(temporary, append(data, '\n'), 0644); err != nil {
		return err
	}

	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)

		return err
	}

	return nil
}

func readJSON(path string, value any) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	if err := json.Unmarshal(data, value); err != nil {
		return false, err
	}

	return true, nil
}

func resPath(projectRoot string, absolute string) (string, error) {
	relative, err := filepath.Rel(projectRoot, absolute)
	if err != nil {
		return "", err
	}

	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside the project: %s", absolute)
	}

	return "res://" + filepath.ToSlash(relative), nil
}

func inspectSession(projectRoot string, session string) (*sessionInfo, error) {
	directory, err := sessionDirectory(projectRoot, session)
	if err != nil {
		return nil, err
	}

	return inspectDirectory(directory)
}

func inspectDirectory(directory string) (*sessionInfo, error) {
	var metadata sessionMetadata

	found, err := readJSON(filepath.Join(directory, "session.json"), &metadata)
	if err != nil {
		return nil, newError(
			string(apierr.CodeInternalError),
			"reading runtime session metadata: %v",
			err,
		)
	}

	if !found {
		return nil, newError(
			string(apierr.CodeInternalError),
			"runtime session metadata is missing in %s",
			directory,
		)
	}

	info := &sessionInfo{metadata: metadata, directory: directory}

	// An exit report is authoritative: the runner writes it before quitting.
	// Check it before PID liveness so a PID reused by another process is never
	// mistaken for the runtime, which could otherwise lead stop to kill the
	// unrelated process.
	if _, found := readExit(directory); found {
		info.state = StateExited

		return info, nil
	}

	if !sameProcess(metadata.PID, metadata.ProcessIdentity) {
		info.state = StateExited

		return info, nil
	}

	heartbeatPath := filepath.Join(directory, "heartbeat.json")

	if stat, err := os.Stat(heartbeatPath); err == nil &&
		time.Since(stat.ModTime()) <= heartbeatTimeout {
		info.state = StateRunning
		info.responsive = true

		return info, nil
	}

	if time.Since(metadata.StartedAt) <= readyGrace {
		info.state = StateStarting

		return info, nil
	}

	info.state = StateUnresponsive

	return info, nil
}

func listSessions(projectRoot string) ([]*sessionInfo, error) {
	entries, err := os.ReadDir(sessionsRoot(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	sessions := make([]*sessionInfo, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() || !validSessionID(entry.Name()) {
			continue
		}

		info, err := inspectDirectory(filepath.Join(sessionsRoot(projectRoot), entry.Name()))
		if err != nil {
			continue
		}

		sessions = append(sessions, info)
	}

	return sessions, nil
}

func activeSession(projectRoot string) (*sessionInfo, error) {
	sessions, err := listSessions(projectRoot)
	if err != nil {
		return nil, err
	}

	var active []*sessionInfo

	for _, info := range sessions {
		if info.state == StateStarting ||
			info.state == StateRunning ||
			info.state == StateUnresponsive {
			active = append(active, info)
		}
	}

	if len(active) == 0 {
		return nil, nil
	}

	if len(active) > 1 {
		names := make([]string, 0, len(active))

		for _, info := range active {
			names = append(names, info.metadata.Session)
		}

		sort.Strings(names)

		return nil, newError(
			string(apierr.CodeConflict),
			"multiple runtime sessions are active (%s); stop them with runtime.stop",
			strings.Join(names, ", "),
		)
	}

	return active[0], nil
}

func latestSession(projectRoot string) (*sessionInfo, error) {
	sessions, err := listSessions(projectRoot)
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].directory > sessions[j].directory
	})

	return sessions[0], nil
}

func readHeartbeat(directory string) heartbeatReport {
	var heartbeat heartbeatReport

	_, _ = readJSON(filepath.Join(directory, "heartbeat.json"), &heartbeat)

	return heartbeat
}

func readExit(directory string) (exitReport, bool) {
	var report exitReport

	found, _ := readJSON(filepath.Join(directory, "exit.json"), &report)

	return report, found
}

func readReady(directory string) (readyReport, bool) {
	var report readyReport

	found, _ := readJSON(filepath.Join(directory, "ready.json"), &report)

	return report, found
}

func readErrors(directory string) ([]RuntimeError, int) {
	entries := []RuntimeError{}
	index := map[string]int{}
	total := 0

	for _, source := range []string{"stdout", "stderr"} {
		for _, line := range logLines(filepath.Join(directory, source+".log")) {
			if !errorLinePattern.MatchString(line) {
				continue
			}

			if len([]rune(line)) > maxErrorRunes {
				line = string([]rune(line)[:maxErrorRunes])
			}

			total++

			key := source + "\x00" + line

			if position, ok := index[key]; ok {
				entries[position].Count++

				continue
			}

			if len(entries) >= maxErrors {
				continue
			}

			index[key] = len(entries)
			entries = append(entries, RuntimeError{
				Source:  source,
				Message: line,
				Count:   1,
			})
		}
	}

	return entries, total
}

func logLines(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return nil
	}

	offset := int64(0)

	if stat.Size() > maxLogTail {
		offset = stat.Size() - maxLogTail
	}

	if _, err := file.Seek(offset, 0); err != nil {
		return nil
	}

	lines := []string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}

	return lines
}

func lastCapture(projectRoot string, directory string) string {
	pattern := filepath.Join(directory, "captures", "*.png")

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	sort.Slice(matches, func(i, j int) bool {
		left, leftErr := os.Stat(matches[i])
		right, rightErr := os.Stat(matches[j])

		if leftErr != nil || rightErr != nil {
			return matches[i] > matches[j]
		}

		return left.ModTime().After(right.ModTime())
	})

	path, err := resPath(projectRoot, matches[0])
	if err != nil {
		return ""
	}

	return path
}

func pruneSessions(projectRoot string) {
	sessions, err := listSessions(projectRoot)
	if err != nil {
		return
	}

	for _, info := range sessions {
		if info.state != StateExited {
			continue
		}

		stat, err := os.Stat(info.directory)
		if err != nil || time.Since(stat.ModTime()) < sessionRetention {
			continue
		}

		_ = os.RemoveAll(info.directory)
	}
}

func intPointer(value int) *int {
	return &value
}
