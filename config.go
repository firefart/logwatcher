package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type configuration struct {
	Mailserver  string   `json:"mailserver"`
	Mailport    int      `json:"mailport"`
	Mailfrom    string   `json:"mailfrom"`
	Mailto      string   `json:"mailto"`
	Mailuser    string   `json:"mailuser"`
	Mailpass    string   `json:"mailpass"`
	MailSkipTLS bool     `json:"mailskiptls"`
	File        string   `json:"file"`
	Watches     []string `json:"watches"`
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
