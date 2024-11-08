package main

import (
	"errors"
	"github.com/hashicorp/go-multierror"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"

	"github.com/go-playground/validator/v10"
)

type configuration struct {
	Files         []configFile       `koanf:"files" validate:"dive"`
	Notifications configNotification `koanf:"notifications"`
}

type configFile struct {
	FileName string   `koanf:"filename" validate:"required,file"`
	Watches  []string `koanf:"watches" validate:"dive,required"`
	Excludes []string `koanf:"excludes"`
}

type configNotification struct {
	Telegram configNotificationTelegram `koanf:"telegram"`
	Discord  configNotificationDiscord  `koanf:"discord"`
	Email    configNotificationEmail    `koanf:"email"`
	SendGrid configNotificationSendGrid `koanf:"sendgrid"`
	MSTeams  configNotificationMSTeams  `koanf:"msteams"`
}

type configNotificationTelegram struct {
	Enabled  bool    `koanf:"enabled"`
	APIToken string  `koanf:"api_token"`
	ChatIDs  []int64 `koanf:"chat_ids"`
}

type configNotificationDiscord struct {
	Enabled    bool     `koanf:"enabled"`
	BotToken   string   `koanf:"bot_token"`
	OAuthToken string   `koanf:"oauth_token"`
	ChannelIDs []string `koanf:"channel_ids"`
}

type configNotificationEmail struct {
	Enabled    bool     `koanf:"enabled"`
	Sender     string   `koanf:"sender"`
	Server     string   `koanf:"server"`
	Port       int      `koanf:"port"`
	Username   string   `koanf:"username"`
	Password   string   `koanf:"password"`
	Recipients []string `koanf:"recipients"`
}

type configNotificationSendGrid struct {
	Enabled       bool     `koanf:"enabled"`
	APIKey        string   `koanf:"api_key"`
	SenderAddress string   `koanf:"sender_address"`
	SenderName    string   `koanf:"sender_name"`
	Recipients    []string `koanf:"recipients"`
}

type configNotificationMSTeams struct {
	Enabled  bool     `koanf:"enabled"`
	Webhooks []string `koanf:"webhooks"`
}

var defaultConfig = configuration{}

func getConfig(f string) (configuration, error) {
	validate := validator.New(validator.WithRequiredStructEnabled())

	k := koanf.NewWithConf(koanf.Conf{
		Delim: ".",
	})

	if err := k.Load(structs.Provider(defaultConfig, "koanf"), nil); err != nil {
		return configuration{}, err
	}

	if err := k.Load(file.Provider(f), json.Parser()); err != nil {
		return configuration{}, err
	}

	var config configuration
	if err := k.Unmarshal("", &config); err != nil {
		return configuration{}, err
	}

	if err := validate.Struct(config); err != nil {
		var invalidValidationError *validator.InvalidValidationError
		if errors.As(err, &invalidValidationError) {
			return configuration{}, err
		}

		var resultErr error
		for _, err := range err.(validator.ValidationErrors) {
			resultErr = multierror.Append(resultErr, err)
		}
		return configuration{}, resultErr
	}

	return config, nil
}
