package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/nikoksr/notify"
	"github.com/nxadm/tail"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
)

type app struct {
	log    *slog.Logger
	config configuration
	notify *notify.Notify
}

type notifyQueueItem struct {
	subject string
	body    string
}

type tailLogger struct {
	log *slog.Logger
}

func (l tailLogger) Fatal(v ...interface{}) {
	panic(fmt.Sprint(v...))
}
func (l tailLogger) Fatalf(format string, v ...interface{}) {
	panic(fmt.Sprintf(format, v...))
}
func (l tailLogger) Fatalln(v ...interface{}) {
	panic(fmt.Sprint(v...))
}
func (l tailLogger) Panic(v ...interface{}) {
	panic(fmt.Sprintln(v...))
}
func (l tailLogger) Panicf(format string, v ...interface{}) {
	panic(fmt.Sprintf(format, v...))
}
func (l tailLogger) Panicln(v ...interface{}) {
	panic(fmt.Sprintln(v...))
}
func (l tailLogger) Print(v ...interface{}) {
	l.log.Info(fmt.Sprint(v...))
}
func (l tailLogger) Printf(format string, v ...interface{}) {
	l.log.Info(fmt.Sprintf(format, v...))
}
func (l tailLogger) Println(v ...interface{}) {
	l.log.Info(fmt.Sprint(v...))
}

func main() {
	var debugMode bool
	var configFilename string
	var jsonOutput bool
	var version bool
	var configCheckMode bool
	flag.BoolVar(&debugMode, "debug", false, "Enable DEBUG mode")
	flag.StringVar(&configFilename, "config", "", "config file to use")
	flag.BoolVar(&jsonOutput, "json", false, "output in json instead")
	flag.BoolVar(&configCheckMode, "configcheck", false, "just check the config")
	flag.BoolVar(&version, "version", false, "show version")
	flag.Parse()

	if version {
		buildInfo, ok := debug.ReadBuildInfo()
		if !ok {
			fmt.Println("Unable to determine version information")
			os.Exit(1)
		}
		fmt.Printf("%s", buildInfo)
		os.Exit(0)
	}

	logger := newLogger(debugMode, jsonOutput)
	var err error
	if configCheckMode {
		err = configCheck(configFilename)
	} else {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		err = run(ctx, logger, configFilename)
	}

	if err != nil {
		// check if we have a multierror
		var merr *multierror.Error
		if errors.As(err, &merr) {
			for _, e := range merr.Errors {
				logger.Error(e.Error())
			}
			os.Exit(1)
		}
		// a normal error
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func configCheck(configFilename string) error {
	_, err := getConfig(configFilename)
	return err
}

func run(ctx context.Context, log *slog.Logger, configFileName string) error {
	config, err := getConfig(configFileName)
	if err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}

	notifier, err := setupNotifications(config, log)
	if err != nil {
		return err
	}

	app := app{
		log:    log,
		config: config,
		notify: notifier,
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("could not get hostname: %w", err)
	}

	notifyChan := make(chan notifyQueueItem, 10)
	errorChan := make(chan error, 10)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case n, ok := <-notifyChan:
				if !ok {
					// channel closed, break out
					return
				}
				if err := app.notify.Send(ctx, n.subject, n.body); err != nil {
					log.Error("error on sending notification", slog.String("err", err.Error()))
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
					log.Error("error on tail", slog.String("err", err.Error()))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	var filesWg sync.WaitGroup
	for _, fileConfig := range config.Files {
		filesWg.Add(1)
		go func(f configFile) {
			defer filesWg.Done()
			tailFile(ctx, f, hostname, log, notifyChan, errorChan)
		}(fileConfig)
	}
	// wait for tails to finish
	filesWg.Wait()
	// once all tails are finished close the channels
	// this path should only be reached if the tails
	// error out. on ctrl+c all goroutines are cancelled
	// so the last errors are not logged
	close(notifyChan)
	close(errorChan)
	// wait for main waitgroup
	wg.Wait()

	return nil
}

func lineIsExcluded(file configFile, line string) bool {
	for _, exclude := range file.Excludes {
		if strings.Contains(line, exclude) {
			return true
		}
	}
	return false
}

func tailFile(ctx context.Context, file configFile, hostname string, log *slog.Logger, notifyChan chan<- notifyQueueItem, errorChan chan<- error) {
	tailLog := tailLogger{
		log: log,
	}
	// Whence: 2 --> Start at end of file
	t, err := tail.TailFile(file.FileName, tail.Config{Follow: true, ReOpen: true, Logger: tailLog, Location: &tail.SeekInfo{Whence: 2}})
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

			log.Debug("got line", slog.String("filename", file.FileName), slog.String("line", line.Text))
			for _, watchString := range file.Watches {
				if !strings.Contains(line.Text, watchString) {
					continue
				}
				// check for excludes
				if !lineIsExcluded(file, line.Text) {
					log.Info("match found", slog.String("filename", file.FileName), slog.String("line", line.Text), slog.String("watch", watchString))
					subject := fmt.Sprintf("[%s] file %s matched string %s", hostname, file.FileName, watchString)
					notifyChan <- notifyQueueItem{
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
