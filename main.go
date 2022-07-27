package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"github.com/sirupsen/logrus"

	gomail "gopkg.in/mail.v2"
)

type app struct {
	log    *logrus.Logger
	config *configuration
	mailer *gomail.Dialer
}

type mailQueueItem struct {
	subject string
	body    string
}

func main() {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			// received ctrl+c
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := run(ctx, log); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Fatalf("[ERROR] %v", err)
		}
	}
}

func (a *app) sendEmailLoop(subject, body string) error {
	for i := 0; i < a.config.Mail.Retries; i++ {
		err := a.sendEmail(subject, body)
		if err == nil {
			// email sent successfully, bail out
			return nil
		}

		if i < a.config.Mail.Retries-1 {
			a.log.Errorf("[ERROR]: %v retrying again after %s", err, a.config.Mail.Sleep.Duration)
			time.Sleep(a.config.Mail.Sleep.Duration)
		} else {
			return fmt.Errorf("could not send email after %d retries: %w", a.config.Mail.Retries, err)
		}
	}
	return fmt.Errorf("should never reach here")
}

func (a *app) sendEmail(subject, body string) error {
	a.log.Debug("sending mail")
	m := gomail.NewMessage()
	m.SetAddressHeader("From", a.config.Mail.From.Mail, a.config.Mail.From.Name)
	m.SetHeader("To", a.config.Mail.To...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	if err := a.mailer.DialAndSend(m); err != nil {
		return err
	}
	return nil
}

func run(ctx context.Context, log *logrus.Logger) error {
	configFile := flag.String("config", "", "config file to use")
	debug := flag.Bool("debug", false, "Print debug output")
	flag.Parse()

	if *debug {
		log.SetLevel(logrus.DebugLevel)
		log.Debug("debug logging enabled")
	}

	config, err := getConfig(*configFile)
	if err != nil {
		log.Fatalf("could not parse config file: %v", err)
	}

	app := app{
		log:    log,
		config: config,
		mailer: gomail.NewDialer(config.Mail.Server, config.Mail.Port, config.Mail.User, config.Mail.Password),
	}

	if config.Mail.SkipTLS {
		app.mailer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	mailChan := make(chan (mailQueueItem), 10)

	go func() {
		for {
			select {
			case mail, ok := <-mailChan:
				if !ok {
					// channel closed, break out
					return
				}
				if err := app.sendEmailLoop(mail.subject, mail.body); err != nil {
					log.Errorf("[ERROR]: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Whence: 2 --> Start at end of file
	t, err := tail.TailFile(config.File, tail.Config{Follow: true, ReOpen: true, Logger: log, Location: &tail.SeekInfo{Whence: 2}})
	if err != nil {
		return err
	}

	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				// channel closed, break out
				return nil
			}

			log.Debugf("got line: %s", line.Text)
			for _, m := range config.Watches {
				if strings.Contains(line.Text, m) {
					log.Debugf("Match for %q: %s", m, line.Text)
					subject := fmt.Sprintf("file %s matched string %s", config.File, m)
					mailChan <- mailQueueItem{
						subject: subject,
						body:    line.Text,
					}
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
