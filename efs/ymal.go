package efs

import (
	"github.com/kylelemons/go-gypsy/yaml"
)

// ConfigFile reads a YAML configuration file from the given filename.
func ConfigFile(filename string) (*yaml.File, error) {
	fin, err := Open(filename)
	if err != nil {
		return nil, err
	}
	defer fin.Close()

	f := new(yaml.File)
	f.Root, err = yaml.Parse(fin)
	if err != nil {
		return nil, err
	}

	return f, nil
}
