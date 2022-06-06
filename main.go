package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/hpcloud/tail"
	"github.com/sirupsen/logrus"

	gomail "gopkg.in/mail.v2"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
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

func tailSystemd(config *configuration, log *logrus.Logger, wg *sync.WaitGroup, watch systemdWatch) {
	defer wg.Done()

	j, err := sdjournal.NewJournalReader(sdjournal.JournalReaderConfig{
		Matches: []sdjournal.Match{
			{
				Field: sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT,
				Value: watch.UnitFile,
			},
		},
	})
	if err != nil {
		log.Errorf("[ERROR]: %v", err)
		return
	}
	defer j.Close()

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if err := j.Follow(nil, writer); err != nil && err != sdjournal.ErrExpired {
		log.Errorf("[ERROR]: %v", err)
		return
	}

	reader := bufio.NewReader(&buf)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text()
		for _, m := range watch.Strings {
			if strings.Contains(text, m) {
				log.Debugf("Match for %q: %s", m, text)
				subject := fmt.Sprintf("unit file %s matched string %s", watch.UnitFile, m)
				if err := sendEmail(config, subject, text); err != nil {
					// do not exit, continue tailing the file
					log.Errorf("[ERROR]: %v", err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func tailFile(config *configuration, log *logrus.Logger, wg *sync.WaitGroup, watch fileWatch) {
	defer wg.Done()

	// Whence: 2 --> Start at end of file
	t, err := tail.TailFile(watch.File, tail.Config{Follow: true, ReOpen: true, Logger: log, Location: &tail.SeekInfo{Whence: 2}})
	if err != nil {
		log.Errorf("[ERROR]: %v", err)
		return
	}
	for line := range t.Lines {
		log.Debugf("got line: %s", line.Text)
		for _, m := range watch.Strings {
			if strings.Contains(line.Text, m) {
				log.Debugf("Match for %q: %s", m, line.Text)
				subject := fmt.Sprintf("file %s matched string %s", watch.File, m)
				if err := sendEmail(config, subject, line.Text); err != nil {
					// do not exit, continue tailing the file
					log.Errorf("[ERROR]: %v", err)
				}
			}
		}
	}
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

	var wg sync.WaitGroup
	for _, fileWatches := range config.FileWatches {
		wg.Add(1)
		go tailFile(config, log, &wg, fileWatches)
	}

	for _, systemdWatches := range config.SystemdWatches {
		wg.Add(1)
		go tailSystemd(config, log, &wg, systemdWatches)
	}

	wg.Wait()

	return nil
}
