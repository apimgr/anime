package config

import "gopkg.in/yaml.v3"

type LocalizedContentBody map[string]string

func (body *LocalizedContentBody) UnmarshalYAML(node *yaml.Node) error {
	values := make(map[string]string)
	if node.Kind == yaml.ScalarNode {
		values["default"] = node.Value
	} else if err := node.Decode(&values); err != nil {
		return err
	}
	*body = values
	return nil
}

func (body LocalizedContentBody) Resolve(language string) string {
	if value := body[language]; value != "" {
		return value
	}
	return body["default"]
}
