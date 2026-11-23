package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequiredLocalesHaveMatchingKeys(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTranslationFallbackAndDirection(t *testing.T) {
	if got, ok := CanonicalLanguage("fr_CA.UTF-8"); !ok || got != "fr" {
		t.Fatalf("system locale canonicalization=%q supported=%t", got, ok)
	}
	if got := Translate("fr-CA", "preferences.tokens.title"); got != "Jetons API" {
		t.Fatalf("regional locale translation=%q", got)
	}
	if got := Translate("es", "missing.key"); got != "missing.key" {
		t.Fatalf("missing key fallback=%q", got)
	}
	if got := Direction("ar"); got != "rtl" {
		t.Fatalf("Arabic direction=%q", got)
	}
}

func TestLanguageDetectionPriorityAndQuality(t *testing.T) {
	request := httptest.NewRequest("GET", "/?lang=it", nil)
	request.AddCookie(&http.Cookie{Name: "lang", Value: "de"})
	request.Header.Set("Accept-Language", "fr;q=1,es;q=0.8")
	if got := LangFromRequest(request); got != "it" {
		t.Fatalf("query language=%q, want it", got)
	}

	request = httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Accept-Language", "de;q=0.8, fr-CA;q=0.9, es;q=0")
	if got := LangFromRequest(request); got != "fr" {
		t.Fatalf("quality-selected language=%q, want fr", got)
	}

	request = httptest.NewRequest("GET", "/?lang=unsupported", nil)
	request.Header.Set("Accept-Language", "fr")
	if got := LangFromRequest(request); got != "en" {
		t.Fatalf("unsupported query fallback=%q, want en", got)
	}

	request = httptest.NewRequest("GET", "/", nil)
	request.AddCookie(&http.Cookie{Name: "lang", Value: "unsupported"})
	request.Header.Set("Accept-Language", "fr")
	if got := LangFromRequest(request); got != "en" {
		t.Fatalf("unsupported cookie fallback=%q, want en", got)
	}
}

func TestTranslateFormatAndPluralForms(t *testing.T) {
	load()
	englishPrevious, hadEnglish := locales["en"]["test_plural"]
	arabicPrevious, hadArabic := locales["ar"]["test_plural"]
	t.Cleanup(func() {
		if hadEnglish {
			locales["en"]["test_plural"] = englishPrevious
		} else {
			delete(locales["en"], "test_plural")
		}
		if hadArabic {
			locales["ar"]["test_plural"] = arabicPrevious
		} else {
			delete(locales["ar"], "test_plural")
		}
	})
	locales["en"]["test_plural"] = map[string]interface{}{"one": "{count} item", "other": "{count} items"}
	locales["ar"]["test_plural"] = map[string]interface{}{
		"zero": "No items", "one": "{count} item", "two": "{count} items",
		"few": "{count} items", "many": "{count} items", "other": "{count} items",
	}
	if got := TranslateFormat("en", "test_plural.one", map[string]string{"count": "1"}); got != "1 item" {
		t.Fatalf("formatted translation=%q", got)
	}
	if got := TranslatePlural("ar", "test_plural", 2); got != "2 items" {
		t.Fatalf("Arabic plural translation=%q", got)
	}
	if got := TranslatePlural("en", "test_plural", 5); got != "5 items" {
		t.Fatalf("English plural translation=%q", got)
	}
}
