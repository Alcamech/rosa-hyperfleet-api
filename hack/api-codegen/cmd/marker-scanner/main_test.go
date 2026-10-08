package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/openshift-online/rosa-hyperfleet-api/hack/api-codegen/pkg/markers"
)

func TestRegistryReportsMutableHiddenFieldsAsPublic(t *testing.T) {
	registry := markers.TypedFieldRegistry{
		"NodePool": {
			"spec.nodePool.nodeLabels": {FieldPath: "spec.nodePool.nodeLabels", WriteMode: markers.Mutable, Hidden: true},
			"spec.nodePool.platform":   {FieldPath: "spec.nodePool.platform", WriteMode: markers.ServiceSet, Hidden: true},
		},
	}

	table := captureStdout(t, func() error { return printTypedRegistryTable(registry) })
	var nodeLabelsRow string
	for _, line := range strings.Split(table, "\n") {
		if strings.Contains(line, "spec.nodePool.nodeLabels") {
			nodeLabelsRow = line
			break
		}
	}
	if nodeLabelsRow == "" || !strings.HasSuffix(strings.TrimSpace(nodeLabelsRow), "no") {
		t.Errorf("mutable hidden field was not reported as public in registry table:\n%s", table)
	}

	stats := captureStdout(t, func() error { return printTypedRegistryStats(registry) })
	if !strings.Contains(stats, "Visibility:  1 visible, 1 hidden") {
		t.Errorf("registry stats did not count mutable hidden field as visible:\n%s", stats)
	}
}

func captureStdout(t *testing.T, write func() error) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close stdout pipe reader: %v", err)
		}
	}()
	previous := os.Stdout
	os.Stdout = writer
	writeErr := write()
	closeErr := writer.Close()
	os.Stdout = previous
	if closeErr != nil {
		t.Errorf("close stdout pipe writer: %v", closeErr)
	}
	if writeErr != nil {
		t.Fatalf("write stdout: %v", writeErr)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(output)
}
