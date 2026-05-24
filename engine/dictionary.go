package engine

import (
	_ "embed"

	"go.yaml.in/yaml/v3"
)

//go:embed data/app_dictionary.yaml
var appDictionaryYAML []byte

// LoadDictionary parses the embedded app-trace YAML dictionary.
func LoadDictionary() (AppDictionary, error) {
	var dict AppDictionary
	if err := yaml.Unmarshal(appDictionaryYAML, &dict); err != nil {
		return nil, err
	}
	return dict, nil
}
