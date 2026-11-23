package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/feature/plural"
	xlanguage "golang.org/x/text/language"
)

var interpolationPattern = regexp.MustCompile(`\{([a-z][a-z0-9_]*)\}`)
var forbiddenInterpolationPattern = regexp.MustCompile(`%[sd]|\{[0-9]+\}`)

//go:embed locales/*.json
var localeFiles embed.FS

var (
	loadOnce sync.Once
	locales  map[string]map[string]interface{}
	rawFiles map[string][]byte
)

type languageInfo struct {
	Name             string
	NativeName       string
	Direction        string
	Version          string
	PluralCategories []string
	Required         bool
}

var languageRegistry = map[string]languageInfo{
	"en": {Name: "English", NativeName: "English", Direction: "ltr", Version: "1.0.0", PluralCategories: []string{"one", "other"}, Required: true},
	"es": {Name: "Spanish", NativeName: "Español", Direction: "ltr", Version: "1.0.0", PluralCategories: []string{"one", "other"}, Required: true},
	"fr": {Name: "French", NativeName: "Français", Direction: "ltr", Version: "1.0.0", PluralCategories: []string{"one", "other"}, Required: true},
	"de": {Name: "German", NativeName: "Deutsch", Direction: "ltr", Version: "1.0.0", PluralCategories: []string{"one", "other"}, Required: true},
	"it": {Name: "Italian", NativeName: "Italiano", Direction: "ltr", Version: "1.0.0", PluralCategories: []string{"one", "many", "other"}, Required: true},
	"ar": {Name: "Arabic", NativeName: "العربية", Direction: "rtl", Version: "1.0.0", PluralCategories: []string{"zero", "one", "two", "few", "many", "other"}, Required: true},
}

func load() {
	loadOnce.Do(func() {
		locales = make(map[string]map[string]interface{})
		rawFiles = make(map[string][]byte)
		files, err := fs.Glob(localeFiles, "locales/*.json")
		if err != nil {
			return
		}
		for _, path := range files {
			data, err := localeFiles.ReadFile(path)
			if err != nil {
				continue
			}
			code := strings.TrimSuffix(strings.TrimPrefix(path, "locales/"), ".json")
			var values map[string]interface{}
			if json.Unmarshal(data, &values) != nil {
				continue
			}
			locales[code] = values
			rawFiles[code] = data
		}
	})
}

func Languages() []string {
	languages := make([]string, 0, len(languageRegistry))
	for language := range languageRegistry {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	return languages
}

func RequiredLanguages() []string {
	var languages []string
	for language, info := range languageRegistry {
		if info.Required {
			languages = append(languages, language)
		}
	}
	sort.Strings(languages)
	return languages
}

func CanonicalLanguage(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.SplitN(value, ".", 2)[0]
	value = strings.SplitN(value, "@", 2)[0]
	value = strings.ReplaceAll(value, "_", "-")
	base := strings.SplitN(value, "-", 2)[0]
	_, ok := languageRegistry[base]
	return base, ok
}

func Translate(language, key string) string {
	load()
	if canonical, ok := CanonicalLanguage(language); ok {
		if value, ok := lookup(locales[canonical], key); ok {
			return value
		}
	}
	if value, ok := lookup(locales["en"], key); ok {
		return value
	}
	return key
}

func TranslateFormat(language, key string, arguments map[string]string) string {
	value := Translate(language, key)
	for name, replacement := range arguments {
		value = strings.ReplaceAll(value, "{"+name+"}", replacement)
	}
	return value
}

func TranslatePlural(language, key string, count int) string {
	load()
	canonical, ok := CanonicalLanguage(language)
	if !ok {
		canonical = "en"
	}
	value, _ := lookupValue(locales[canonical], key)
	forms, ok := value.(map[string]interface{})
	if !ok {
		englishForms, _ := lookupValue(locales["en"], key)
		forms, _ = englishForms.(map[string]interface{})
	}
	if forms == nil {
		return key
	}
	tag, err := xlanguage.Parse(canonical)
	if err != nil {
		tag = xlanguage.English
	}
	form := pluralFormName(plural.Cardinal.MatchPlural(tag, count, 0, 0, 0, 0))
	if _, ok := forms[form].(string); ok {
		return TranslateFormat(canonical, key+"."+form, map[string]string{"count": strconv.Itoa(count)})
	} else if _, ok := forms["other"].(string); ok {
		return TranslateFormat(canonical, key+".other", map[string]string{"count": strconv.Itoa(count)})
	} else {
		return key
	}
}

func pluralFormName(form plural.Form) string {
	switch form {
	case plural.Zero:
		return "zero"
	case plural.One:
		return "one"
	case plural.Two:
		return "two"
	case plural.Few:
		return "few"
	case plural.Many:
		return "many"
	default:
		return "other"
	}
}

func LangFromRequest(request *http.Request) string {
	return LangFromRequestWithDefault(request, "en")
}

func LangFromRequestWithDefault(request *http.Request, defaultLanguage string) string {
	if requested := request.URL.Query().Get("lang"); requested != "" {
		if canonical, ok := CanonicalLanguage(requested); ok {
			return canonical
		}
		return "en"
	}
	if cookie, err := request.Cookie("lang"); err == nil && cookie.Value != "" {
		if canonical, ok := CanonicalLanguage(cookie.Value); ok {
			return canonical
		}
		return "en"
	}
	for _, candidate := range acceptedLanguages(request.Header.Get("Accept-Language")) {
		if canonical, ok := CanonicalLanguage(candidate); ok {
			return canonical
		}
	}
	if canonical, ok := CanonicalLanguage(defaultLanguage); ok {
		return canonical
	}
	return "en"
}

func acceptedLanguages(header string) []string {
	type candidate struct {
		language string
		quality  float64
	}
	values := make([]candidate, 0)
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(strings.TrimSpace(item), ";")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[0]) == "*" {
			continue
		}
		quality := 1.0
		if len(parts) > 1 && strings.HasPrefix(strings.TrimSpace(parts[1]), "q=") {
			parsed, err := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(parts[1]), "q="), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				continue
			}
			quality = parsed
		}
		if quality > 0 {
			values = append(values, candidate{language: strings.TrimSpace(parts[0]), quality: quality})
		}
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].quality > values[j].quality })
	languages := make([]string, len(values))
	for index, value := range values {
		languages[index] = value.language
	}
	return languages
}

func NativeName(language string) string {
	load()
	canonical, ok := CanonicalLanguage(language)
	if !ok {
		canonical = "en"
	}
	if info, ok := languageRegistry[canonical]; ok {
		return info.NativeName
	}
	return "English"
}

func Direction(language string) string {
	load()
	canonical, ok := CanonicalLanguage(language)
	if !ok {
		canonical = "en"
	}
	if info, ok := languageRegistry[canonical]; ok {
		return info.Direction
	}
	return "ltr"
}

func JSON(language string) ([]byte, bool) {
	load()
	canonical, ok := CanonicalLanguage(language)
	if !ok {
		return nil, false
	}
	data, ok := rawFiles[canonical]
	return append([]byte(nil), data...), ok
}

func Validate() error {
	load()
	for _, language := range Languages() {
		if _, ok := rawFiles[language]; !ok {
			return fmt.Errorf("required locale %s is missing or invalid", language)
		}
		info := languageRegistry[language]
		for key, expected := range map[string]string{"meta.language": language, "meta.name": info.Name, "meta.native_name": info.NativeName, "meta.direction": info.Direction, "meta.version": info.Version} {
			if actual, ok := lookup(locales[language], key); !ok || actual != expected {
				return fmt.Errorf("locale %s has invalid %s metadata", language, key)
			}
		}
	}
	for language := range rawFiles {
		if _, ok := languageRegistry[language]; !ok {
			return fmt.Errorf("locale %s is missing a language registry entry", language)
		}
	}
	englishKeys := make(map[string]struct{})
	flattenKeys("", locales["en"], englishKeys)
	for key := range englishKeys {
		if strings.HasPrefix(key, "plurals.") {
			if err := validatePluralKey("en", key); err != nil {
				return err
			}
		}
	}
	for _, language := range Languages() {
		if language == "en" {
			continue
		}
		keys := make(map[string]struct{})
		flattenKeys("", locales[language], keys)
		if len(keys) != len(englishKeys) {
			return fmt.Errorf("locale %s has %d keys; English has %d", language, len(keys), len(englishKeys))
		}
		for key := range englishKeys {
			if _, ok := keys[key]; !ok {
				return fmt.Errorf("locale %s is missing key %s", language, key)
			}
			if strings.HasPrefix(key, "plurals.") {
				if err := validatePluralKey(language, key); err != nil {
					return err
				}
				continue
			}
			englishValue, _ := lookup(locales["en"], key)
			translatedValue, _ := lookup(locales[language], key)
			if strings.TrimSpace(translatedValue) == "" {
				return fmt.Errorf("locale %s has an empty translation for %s", language, key)
			}
			if forbiddenInterpolationPattern.MatchString(englishValue) || forbiddenInterpolationPattern.MatchString(translatedValue) {
				return fmt.Errorf("locale %s uses a forbidden interpolation in %s", language, key)
			}
			if strings.Join(interpolationNames(englishValue), "\x00") != strings.Join(interpolationNames(translatedValue), "\x00") {
				return fmt.Errorf("locale %s has mismatched interpolation variables for %s", language, key)
			}
		}
	}
	return nil
}

func validatePluralKey(language, key string) error {
	englishValue, found := lookupValue(locales["en"], key)
	englishForms, ok := englishValue.(map[string]interface{})
	if !found || !ok {
		return fmt.Errorf("english plural message %s is invalid", key)
	}
	value, found := lookupValue(locales[language], key)
	forms, ok := value.(map[string]interface{})
	if !found || !ok {
		return fmt.Errorf("locale %s plural message %s is invalid", language, key)
	}
	info := languageRegistry[language]
	allowed := make(map[string]struct{}, len(info.PluralCategories)+1)
	for _, category := range info.PluralCategories {
		allowed[category] = struct{}{}
		if _, ok := forms[category]; !ok {
			return fmt.Errorf("locale %s plural message %s is missing category %s", language, key, category)
		}
	}
	allowed["zero"] = struct{}{}
	for category, value := range forms {
		if _, ok := allowed[category]; !ok {
			return fmt.Errorf("locale %s plural message %s has unsupported category %s", language, key, category)
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("locale %s plural message %s has an empty category %s", language, key, category)
		}
		englishText, ok := englishForms[category].(string)
		if !ok {
			englishText, _ = englishForms["other"].(string)
		}
		englishNames := withoutCount(interpolationNames(englishText))
		translatedNames := withoutCount(interpolationNames(text))
		if strings.Join(englishNames, "\x00") != strings.Join(translatedNames, "\x00") {
			return fmt.Errorf("locale %s plural message %s has mismatched interpolation variables", language, key)
		}
	}
	return nil
}

func withoutCount(names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		if name != "count" {
			result = append(result, name)
		}
	}
	return result
}

func interpolationNames(value string) []string {
	matches := interpolationPattern.FindAllStringSubmatch(value, -1)
	names := make([]string, len(matches))
	for index, match := range matches {
		names[index] = match[1]
	}
	sort.Strings(names)
	return names
}

func lookup(values map[string]interface{}, key string) (string, bool) {
	value, ok := lookupValue(values, key)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func lookupValue(values map[string]interface{}, key string) (interface{}, bool) {
	var current interface{} = values
	for _, part := range strings.Split(key, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func flattenKeys(prefix string, values map[string]interface{}, output map[string]struct{}) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if prefix == "" && key == "meta" {
			continue
		}
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if child, ok := values[key].(map[string]interface{}); ok {
			if prefix == "plurals" {
				output[path] = struct{}{}
				continue
			}
			flattenKeys(path, child, output)
		} else {
			output[path] = struct{}{}
		}
	}
}
