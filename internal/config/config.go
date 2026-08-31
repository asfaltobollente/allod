package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Node    NodeConfig              `yaml:"node"`
	Catalog CatalogConfig           `yaml:"catalog"`
	People  []PersonConfig          `yaml:"people"`
	Modules map[string]ModuleConfig `yaml:"modules"`
}

type NodeConfig struct {
	Name    string `yaml:"name"`
	Channel string `yaml:"channel"`
	Group   string `yaml:"group"`
}

type CatalogConfig struct {
	Source string `yaml:"source"`
	Mode   string `yaml:"mode"`
}

type PersonConfig struct {
	ID      string `yaml:"id"`
	Role    string `yaml:"role"`
	QuotaGB int    `yaml:"quota_gb"`
}

type ModuleConfig struct {
	Level string   `yaml:"level"`
	Disks []string `yaml:"disks,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	// Windows: os.Rename fails if target exists, so remove target or replace
	os.Remove(path)
	return os.Rename(tmpPath, path)
}
