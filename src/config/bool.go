package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Boolean bool

func (value *Boolean) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag == "!!null" {
		return nil
	}
	parsed, err := ParseBool(node.Value, bool(*value))
	if err != nil {
		return err
	}
	*value = Boolean(parsed)
	return nil
}

var truthyValues = map[string]bool{
	"1": true, "y": true, "t": true,
	"yes": true, "true": true, "on": true, "ok": true,
	"enable": true, "enabled": true,
	"yep": true, "yup": true, "yeah": true,
	"aye": true, "si": true, "oui": true, "da": true, "hai": true,
	"affirmative": true, "accept": true, "allow": true, "grant": true,
	"sure": true, "totally": true,
}

var falsyValues = map[string]bool{
	"0": true, "n": true, "f": true,
	"no": true, "false": true, "off": true,
	"disable": true, "disabled": true,
	"nope": true, "nah": true, "nay": true,
	"nein": true, "non": true, "niet": true, "iie": true, "lie": true,
	"negative": true, "reject": true, "block": true, "revoke": true,
	"deny": true, "never": true, "noway": true,
}

// ParseBool accepts the truthy and falsy values defined by the configuration specification.
func ParseBool(value string, defaultValue bool) (bool, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return defaultValue, nil
	}
	if truthyValues[value] {
		return true, nil
	}
	if falsyValues[value] {
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean value: %q", value)
}

// MustParseBool parses a configuration boolean and panics when the value is invalid.
func MustParseBool(value string, defaultValue bool) bool {
	parsed, err := ParseBool(value, defaultValue)
	if err != nil {
		panic(err)
	}
	return parsed
}

// IsTruthy reports whether value is one of the accepted truthy values.
func IsTruthy(value string) bool {
	return truthyValues[strings.ToLower(strings.TrimSpace(value))]
}

// IsFalsy reports whether value is one of the accepted falsy values.
func IsFalsy(value string) bool {
	return falsyValues[strings.ToLower(strings.TrimSpace(value))]
}
