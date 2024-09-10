package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type duration struct {
	time.Duration
}

func (d duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}

type mailConfig struct {
	Server string `json:"server"`
	Port   int    `json:"port"`
	From   struct {
		Name string `json:"name"`
		Mail string `json:"mail"`
	} `json:"from"`
	To       []string      `json:"to"`
	User     string        `json:"user"`
	Password string        `json:"password"`
	TLS      bool          `json:"tls"`
	SkipTLS  bool          `json:"skiptls"`
	Retries  int           `json:"retries"`
	Sleep    duration      `json:"sleep"`
	Timeout  time.Duration `json:"timeout"`
}

type configuration struct {
	Mail  mailConfig `json:"mail"`
	Files []file     `json:"files"`
}

type file struct {
	FileName string   `json:"filename"`
	Watches  []string `json:"watches"`
	Excludes []string `json:"excludes"`
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
	decoder.DisallowUnknownFields()

	// set some defaults
	c := configuration{
		Mail: mailConfig{
			Retries: 3,
			Sleep: duration{
				Duration: 1 * time.Second,
			},
			Timeout: 10 * time.Second,
		},
	}

	if err = decoder.Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
