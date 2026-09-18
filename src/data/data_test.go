package data

import (
	"encoding/json"
	"testing"
)

func TestReadFileExisting(t *testing.T) {
	content, err := ReadFile("dataset.json")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected non-empty dataset.json content")
	}

	var quotes []map[string]interface{}
	if err := json.Unmarshal(content, &quotes); err != nil {
		t.Fatalf("dataset.json is not valid JSON: %v", err)
	}
	if len(quotes) == 0 {
		t.Error("expected at least one quote in dataset.json")
	}
}

func TestReadFileMissing(t *testing.T) {
	_, err := ReadFile("does-not-exist.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
