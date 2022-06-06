package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type configuration struct {
	Mail struct {
		Server string `json:"server"`
		Port   int    `json:"port"`
		From   struct {
			Name string `json:"name"`
			Mail string `json:"mail"`
		} `json:"from"`
		To       []string `json:"to"`
		User     string   `json:"user"`
		Password string   `json:"password"`
		SkipTLS  bool     `json:"skiptls"`
	} `json:"mail"`
	FileWatches    []fileWatch    `json:"files"`
	SystemdWatches []systemdWatch `json:"systemd"`
}

type fileWatch struct {
	File    string   `json:"file"`
	Strings []string `json:"strings"`
}

type systemdWatch struct {
	UnitFile string   `json:"unitfile"`
	Strings  []string `json:"strings"`
}

func getConfig(f string) (*configuration, error) {
	if f == "" {
		return nil, fmt.Errorf("please provide a valid config file")
	}

	b, err := os.ReadFile(f) // nolint: gosec
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(b)

	decoder := json.NewDecoder(reader)
	c := configuration{}
	if err = decoder.Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
