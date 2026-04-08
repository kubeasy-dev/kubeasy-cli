package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeEvent returns a single NDJSON audit log line.
func makeEvent(namespace, verb string, ts time.Time, resource string) string {
	return fmt.Sprintf(
		`{"kind":"Event","apiVersion":"audit.k8s.io/v1","stage":"ResponseComplete","stageTimestamp":%q,"verb":%q,"userAgent":"kubectl/v1.35.0","responseStatus":{"code":200},"objectRef":{"resource":%q,"namespace":%q,"name":"test"}}`,
		ts.Format(time.RFC3339Nano), verb, resource, namespace,
	)
}

func writeLogFile(t *testing.T, lines []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "audit-*.log")
	require.NoError(t, err)
	for _, l := range lines {
		_, err := fmt.Fprintln(f, l)
		require.NoError(t, err)
	}
	require.NoError(t, f.Close())
	return f.Name()
}

func TestReadAndFilter_NamespaceFilter(t *testing.T) {
	now := time.Now().UTC()
	lines := []string{
		makeEvent("ns-a", "create", now, "pods"),
		makeEvent("ns-b", "delete", now, "pods"),
	}
	logPath := writeLogFile(t, lines)

	events, err := ReadAndFilter(logPath, "ns-a", time.Time{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "ns-a", events[0].Namespace)
	assert.Equal(t, "create", events[0].Verb)
}

func TestReadAndFilter_TimeWindow(t *testing.T) {
	base := time.Now().UTC()
	old := base.Add(-10 * time.Minute)
	recent := base.Add(-1 * time.Minute)

	lines := []string{
		makeEvent("ns-a", "create", old, "pods"),
		makeEvent("ns-a", "update", recent, "deployments"),
	}
	logPath := writeLogFile(t, lines)

	// Only return events after base-5m
	since := base.Add(-5 * time.Minute)
	events, err := ReadAndFilter(logPath, "ns-a", since)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "update", events[0].Verb)
}

func TestReadAndFilter_Cap500(t *testing.T) {
	now := time.Now().UTC()
	lines := make([]string, 600)
	for i := range lines {
		ts := now.Add(time.Duration(i) * time.Second)
		lines[i] = makeEvent("ns-a", "create", ts, "pods")
	}
	logPath := writeLogFile(t, lines)

	events, err := ReadAndFilter(logPath, "ns-a", time.Time{})
	require.NoError(t, err)
	assert.Len(t, events, 500)
	// Should be the 500 most recent (indices 100..599)
	assert.Equal(t, now.Add(100*time.Second).Truncate(time.Second), events[0].Timestamp.Truncate(time.Second))
}

func TestReadAndFilter_MissingFile(t *testing.T) {
	events, err := ReadAndFilter(filepath.Join(t.TempDir(), "nonexistent.log"), "ns-a", time.Time{})
	require.NoError(t, err)
	assert.Nil(t, events)
}

func TestReadAndFilter_MalformedLinesSkipped(t *testing.T) {
	now := time.Now().UTC()
	lines := []string{
		`not-json-at-all`,
		`{"stage":"ResponseComplete"}`, // missing objectRef
		makeEvent("ns-a", "create", now, "pods"),
	}
	logPath := writeLogFile(t, lines)

	events, err := ReadAndFilter(logPath, "ns-a", time.Time{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "create", events[0].Verb)
}

func TestReadAndFilter_NonResponseCompleteStageSkipped(t *testing.T) {
	now := time.Now().UTC()
	line := fmt.Sprintf(
		`{"stage":"RequestReceived","stageTimestamp":%q,"verb":"create","responseStatus":{"code":200},"objectRef":{"resource":"pods","namespace":"ns-a","name":"test"}}`,
		now.Format(time.RFC3339Nano),
	)
	logPath := writeLogFile(t, []string{line})

	events, err := ReadAndFilter(logPath, "ns-a", time.Time{})
	require.NoError(t, err)
	assert.Nil(t, events)
}
