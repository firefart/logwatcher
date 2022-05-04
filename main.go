package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/hpcloud/tail"

	gomail "gopkg.in/mail.v2"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

func sendEmail(config *configuration, from, to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)
	d := gomail.NewDialer(config.Mailserver, config.Mailport, config.Mailuser, config.Mailpass)

	if config.MailSkipTLS {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if err := d.DialAndSend(m); err != nil {
		return err
	}
	return nil
}

func run() error {
	configFile := flag.String("config", "", "config file to use")
	flag.Parse()

	config, err := getConfig(*configFile)
	if err != nil {
		log.Fatalf("could not parse config file: %v", err)
	}

	t, err := tail.TailFile(config.File, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		return err
	}
	for line := range t.Lines {
		for _, m := range config.Watches {
			if strings.Contains(line.Text, m) {
				subject := fmt.Sprintf("file %s matched string %s", config.File, m)
				if err := sendEmail(config, "", "", subject, line.Text); err != nil {
					// do not exit, continue tailing the file
					log.Printf("[ERROR]: %v", err)
				}
			}
		}
	}

	return nil
}
