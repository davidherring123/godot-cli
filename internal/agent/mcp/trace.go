package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// traceEvent is one complete tools/call record. It contains only MCP call data,
// rather than metadata specific to any client or agent harness.
type traceEvent struct {
	Tool       string          `json:"tool"`
	Arguments  json.RawMessage `json:"arguments"`
	IsError    bool            `json:"is_error"`
	DurationMS int64           `json:"duration_ms"`
	Timestamp  string          `json:"timestamp"`
}

// callTracer writes one JSON object per completed tool call. Calls can finish
// concurrently (in particular, an action abandoned after client cancellation
// may finish while another call is running), so writes are serialized.
//
// A write failure is reported once and disables further tracing for the
// process. Retrying or allowing later records to append after a partial line
// would corrupt the JSONL stream, and trace persistence must never change the
// outcome of the tool call that has already been applied.
type callTracer struct {
	mu          sync.Mutex
	writer      io.Writer
	errorWriter io.Writer
	failed      bool
}

func newCallTracer(writer, errorWriter io.Writer) *callTracer {
	if writer == nil {
		return nil
	}

	if errorWriter == nil {
		errorWriter = os.Stderr
	}

	return &callTracer{writer: writer, errorWriter: errorWriter}
}

func (tracer *callTracer) record(
	tool string,
	arguments json.RawMessage,
	isError bool,
	started time.Time,
) {
	if tracer == nil {
		return
	}

	event := traceEvent{
		Tool:       tool,
		Arguments:  traceArguments(arguments),
		IsError:    isError,
		DurationMS: time.Since(started).Milliseconds(),
		Timestamp:  started.UTC().Format(time.RFC3339Nano),
	}

	line, err := json.Marshal(event)
	if err != nil {
		tracer.report(fmt.Errorf("encoding trace record: %w", err))
		return
	}

	line = append(line, '\n')

	tracer.mu.Lock()
	defer tracer.mu.Unlock()

	if tracer.failed {
		return
	}

	written, err := tracer.writer.Write(line)
	if err == nil && written != len(line) {
		err = io.ErrShortWrite
	}

	if err != nil {
		tracer.failed = true
		tracer.reportLocked(fmt.Errorf("writing trace record: %w", err))
	}
}

// traceArguments preserves the raw request arguments. Invalid JSON (only
// possible through a direct handler call, not the SDK's tools/call path) is
// encoded as a string so one malformed request cannot drop its trace record.
func traceArguments(arguments json.RawMessage) json.RawMessage {
	if len(arguments) == 0 {
		return json.RawMessage("null")
	}

	if !json.Valid(arguments) {
		encoded, err := json.Marshal(string(arguments))
		if err != nil {
			return json.RawMessage("null")
		}

		return encoded
	}

	return arguments
}

func (tracer *callTracer) report(err error) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()

	tracer.reportLocked(err)
}

// reportLocked writes a diagnostic to the configured trace error writer while
// the tracer mutex is held, keeping diagnostics from interleaving.
func (tracer *callTracer) reportLocked(err error) {
	_, _ = fmt.Fprintf(tracer.errorWriter, "godot-mcp-host: trace disabled: %v\n", err)
}
