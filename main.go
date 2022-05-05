package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hpcloud/tail"
	"github.com/sirupsen/logrus"

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
	log := logrus.New()

	configFile := flag.String("config", "", "config file to use")
	debug := flag.Bool("debug", false, "Print debug output")
	flag.Parse()

	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
	if *debug {
		log.SetLevel(logrus.DebugLevel)
	}

	config, err := getConfig(*configFile)
	if err != nil {
		log.Fatalf("could not parse config file: %v", err)
	}

	// Whence: 2 --> Start at end of file
	t, err := tail.TailFile(config.File, tail.Config{Follow: true, ReOpen: true, Logger: log, Location: &tail.SeekInfo{Whence: 2}})
	if err != nil {
		return err
	}
	for line := range t.Lines {
		log.Debugf("got line: %s", line.Text)
		for _, m := range config.Watches {
			if strings.Contains(line.Text, m) {
				log.Debugf("Match for %q: %s", m, line.Text)
				subject := fmt.Sprintf("file %s matched string %s", config.File, m)
				if err := sendEmail(config, config.Mailfrom, config.Mailto, subject, line.Text); err != nil {
					// do not exit, continue tailing the file
					log.Errorf("[ERROR]: %v", err)
				}
			}
		}
	}

	return nil
}
