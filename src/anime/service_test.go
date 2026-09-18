package anime

import (
	"testing"
)

const sampleQuotes = `[
	{"anime":"Naruto","character":"Naruto Uzumaki","quote":"Believe it!"},
	{"anime":"Bleach","character":"Ichigo Kurosaki","quote":"Protect what matters."}
]`

func TestNewServiceValid(t *testing.T) {
	svc, err := NewService([]byte(sampleQuotes))
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if svc.GetTotalQuotes() != 2 {
		t.Fatalf("expected 2 quotes, got %d", svc.GetTotalQuotes())
	}
}

func TestNewServiceInvalidJSON(t *testing.T) {
	_, err := NewService([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestGetRandomQuote(t *testing.T) {
	svc, err := NewService([]byte(sampleQuotes))
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	quote := svc.GetRandomQuote()
	m, ok := quote.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", quote)
	}
	for _, key := range []string{"anime", "character", "quote"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected key %q in quote result", key)
		}
	}
}

func TestGetRandomQuoteEmpty(t *testing.T) {
	svc, err := NewService([]byte(`[]`))
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	quote := svc.GetRandomQuote()
	m, ok := quote.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", quote)
	}
	if m["error"] != "no quotes available" {
		t.Errorf("expected error message, got %q", m["error"])
	}
}

func TestGetAllQuotes(t *testing.T) {
	svc, err := NewService([]byte(sampleQuotes))
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	all := svc.GetAllQuotes()
	slice, ok := all.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", all)
	}
	if len(slice) != 2 {
		t.Fatalf("expected 2 quotes, got %d", len(slice))
	}

	first, ok := slice[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", slice[0])
	}
	if first["anime"] != "Naruto" {
		t.Errorf("expected anime %q, got %v", "Naruto", first["anime"])
	}
}

func TestGetTotalQuotes(t *testing.T) {
	svc, err := NewService([]byte(`[]`))
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if svc.GetTotalQuotes() != 0 {
		t.Fatalf("expected 0 quotes, got %d", svc.GetTotalQuotes())
	}
}
