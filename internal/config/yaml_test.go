package config

import "gopkg.in/yaml.v3"

func unmarshalYAML(value string, out any) error {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(value), &node); err != nil {
		return err
	}
	return node.Content[0].Decode(out)
}
