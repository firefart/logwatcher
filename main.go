package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"github.com/sirupsen/logrus"

	gomail "gopkg.in/mail.v2"
)

func main() {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)

	if err := run(log); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

func sendEmailLoop(log *logrus.Logger, config *configuration, subject, body string) error {
	for i := 0; i < config.Mail.Retries; i++ {
		err := sendEmail(config, subject, body)
		if err == nil {
			// email sent successfully, bail out
			return nil
		}

		if i < config.Mail.Retries-1 {
			log.Errorf("[ERROR]: %v retrying again after %s", err, config.Mail.Sleep.Duration)
			time.Sleep(config.Mail.Sleep.Duration)
		} else {
			return fmt.Errorf("could not send email after %d retries: %w", config.Mail.Retries, err)
		}
	}
	return fmt.Errorf("should never reach here")
}

func sendEmail(config *configuration, subject, body string) error {
	m := gomail.NewMessage()
	m.SetAddressHeader("From", config.Mail.From.Mail, config.Mail.From.Name)
	m.SetHeader("To", config.Mail.To...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)
	d := gomail.NewDialer(config.Mail.Server, config.Mail.Port, config.Mail.User, config.Mail.Password)

	if config.Mail.SkipTLS {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if err := d.DialAndSend(m); err != nil {
		return err
	}
	return nil
}

func run(log *logrus.Logger) error {
	configFile := flag.String("config", "", "config file to use")
	debug := flag.Bool("debug", false, "Print debug output")
	flag.Parse()

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
				// async email sending
				go func(subj, body string) {
					if err := sendEmailLoop(log, config, subj, body); err != nil {
						log.Errorf("[ERROR]: %v", err)
					}
				}(subject, line.Text)
			}
		}
	}

	return nil
}
