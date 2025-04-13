package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Repos []Repo
}

type Repo struct {
	Name     string
	URL      string
	Revision string
	Auth     Auth
	Webhook  string
	Poll     time.Duration
	Files    []File
}

type Auth struct {
	Username string
	Password string
	Key      string
}

type File struct {
	Config   string
	Path     string
	Template string
	Values   []Value
}

type Value struct {
	Key   string
	Value any
}

func Parse(cfgFile string) (*Config, error) {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	cfg := Config{}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling data: %w", err)
	}

	if err = cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if len(c.Repos) == 0 {
		return errors.New("no repos defined")
	}

	return nil
}
