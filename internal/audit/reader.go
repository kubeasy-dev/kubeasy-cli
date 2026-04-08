package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// AuditEvent is the subset of a Kubernetes audit event forwarded with the submit payload.
type AuditEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Verb         string    `json:"verb"`
	Resource     string    `json:"resource"`
	Subresource  string    `json:"subresource,omitempty"`
	Name         string    `json:"name,omitempty"`
	Namespace    string    `json:"namespace,omitempty"`
	UserAgent    string    `json:"userAgent,omitempty"`
	ResponseCode int       `json:"responseCode,omitempty"`
}

// rawAuditEvent mirrors the NDJSON structure emitted by the Kubernetes API server.
type rawAuditEvent struct {
	Stage          string `json:"stage"`
	StageTimestamp string `json:"stageTimestamp"`
	Verb           string `json:"verb"`
	UserAgent      string `json:"userAgent"`
	ResponseStatus struct {
		Code int `json:"code"`
	} `json:"responseStatus"`
	ObjectRef *struct {
		Resource    string `json:"resource"`
		Namespace   string `json:"namespace"`
		Name        string `json:"name"`
		Subresource string `json:"subresource"`
	} `json:"objectRef"`
}

const maxAuditEvents = 500

// ReadAndFilter reads the audit log at logPath and returns events that:
//   - have stage == "ResponseComplete"
//   - have objectRef.namespace == namespace
//   - have stageTimestamp after since (zero time matches all)
//
// Malformed JSON lines are silently skipped. If logPath does not exist, an
// empty slice is returned without error. The result is capped at 500 events
// (most recent wins when the buffer overflows).
func ReadAndFilter(logPath, namespace string, since time.Time) ([]AuditEvent, error) {
	f, err := os.Open(logPath) //nolint:gosec // logPath is derived from constants, not user input
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	// Use a ring buffer so we naturally keep the 500 most recent events
	// without loading the entire log into memory upfront.
	ring := make([]AuditEvent, maxAuditEvents)
	var total int

	scanner := bufio.NewScanner(f)
	// Increase buffer for long audit lines.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw rawAuditEvent
		if err := json.Unmarshal(line, &raw); err != nil {
			continue // skip malformed lines non-fatally
		}

		if raw.Stage != "ResponseComplete" {
			continue
		}
		if raw.ObjectRef == nil || raw.ObjectRef.Namespace != namespace {
			continue
		}

		ts, err := time.Parse(time.RFC3339Nano, raw.StageTimestamp)
		if err != nil {
			ts, err = time.Parse(time.RFC3339, raw.StageTimestamp)
			if err != nil {
				continue
			}
		}

		if !since.IsZero() && !ts.After(since) {
			continue
		}

		evt := AuditEvent{
			Timestamp:    ts,
			Verb:         raw.Verb,
			Resource:     raw.ObjectRef.Resource,
			Subresource:  raw.ObjectRef.Subresource,
			Name:         raw.ObjectRef.Name,
			Namespace:    raw.ObjectRef.Namespace,
			UserAgent:    raw.UserAgent,
			ResponseCode: raw.ResponseStatus.Code,
		}
		ring[total%maxAuditEvents] = evt
		total++
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	if total == 0 {
		return nil, nil
	}

	// Reconstruct in chronological order from the ring buffer.
	if total <= maxAuditEvents {
		result := make([]AuditEvent, total)
		copy(result, ring[:total])
		return result, nil
	}

	// Ring has wrapped: oldest entry is at total%maxAuditEvents.
	start := total % maxAuditEvents
	result := make([]AuditEvent, maxAuditEvents)
	copy(result, ring[start:])
	copy(result[maxAuditEvents-start:], ring[:start])
	return result, nil
}
