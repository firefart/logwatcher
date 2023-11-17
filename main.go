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
	"sync"
	"time"

	"github.com/nxadm/tail"
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
		return fmt.Errorf("could not parse config file: %w", err)
	}

	app := app{
		log:    log,
		config: config,
		mailer: gomail.NewDialer(config.Mail.Server, config.Mail.Port, config.Mail.User, config.Mail.Password),
	}

	if config.Mail.SkipTLS {
		app.mailer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	mailChan := make(chan mailQueueItem, 10)
	errorChan := make(chan error, 10)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case err, ok := <-errorChan:
				if !ok {
					// channel closed, break out
					return
				}
				if !errors.Is(err, context.Canceled) {
					log.Errorf("[ERROR] %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	var filesWg sync.WaitGroup
	for _, fileConfig := range config.Files {
		filesWg.Add(1)
		go func(f file) {
			defer filesWg.Done()
			tailFile(ctx, f, log, mailChan, errorChan)
		}(fileConfig)
	}
	// wait for tails to finish
	filesWg.Wait()
	// once all tails are finished close the channels
	// this path should only be reached if the tails
	// error out. on ctrl+c all goroutines are cancelled
	// so the last errors are not logged
	close(mailChan)
	close(errorChan)
	// wait for main waitgroup
	wg.Wait()

	return nil
}

func lineIsExcluded(file file, line string) bool {
	for _, exclude := range file.Excludes {
		if strings.Contains(line, exclude) {
			return true
		}
	}
	return false
}

func tailFile(ctx context.Context, file file, log *logrus.Logger, mailChan chan<- mailQueueItem, errorChan chan<- error) {
	// Whence: 2 --> Start at end of file
	t, err := tail.TailFile(file.FileName, tail.Config{Follow: true, ReOpen: true, Logger: log, Location: &tail.SeekInfo{Whence: 2}})
	if err != nil {
		errorChan <- err
		return
	}

	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				// channel closed, break out
				return
			}

			log.Debugf("%s: got line: %s", file.FileName, line.Text)
			for _, watchString := range file.Watches {
				if !strings.Contains(line.Text, watchString) {
					continue
				}
				// check for excludes
				if !lineIsExcluded(file, line.Text) {
					log.Debugf("%s: match for %q: %s", file.FileName, watchString, line.Text)
					subject := fmt.Sprintf("file %s matched string %s", file.FileName, watchString)
					mailChan <- mailQueueItem{
						subject: subject,
						body:    line.Text,
					}
				}
			}
		case <-ctx.Done():
			errorChan <- ctx.Err()
			return
		}
	}
}
