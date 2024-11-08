package main

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestParseConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test")
	require.Nil(t, err)

	defer func(tmpFile *os.File) {
		err := tmpFile.Close()
		if err != nil {
			require.Nil(t, err)
		}
	}(tmpFile)

	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			require.Nil(t, err)
		}
	}(tmpFile.Name())

	config := fmt.Sprintf(`{
    "files": [
    {
      "filename": "%[1]s",
      "watches": [
        "match1",
        "match2",
        "match3"
      ]
    },
    {
      "filename": "%[1]s",
      "watches": [
        "match1",
        "match2",
        "match3"
      ],
      "excludes": [
        "exclude1",
        "exclude2"
      ]
    }
  ],
  "notifications": {
    "telegram": {
      "enabled": true,
      "api_token": "token",
      "chat_ids": [
        1,
        2
      ]
    },
    "discord": {
      "enabled": true,
      "bot_token": "token",
      "oauth_token": "token",
      "channel_ids": [
        "1",
        "2"
      ]
    },
    "email": {
      "enabled": true,
      "sender": "test@test.com",
      "server": "smtp.server.com",
      "port": 25,
      "username": "user",
      "password": "pass",
      "recipients": [
        "test@test.com",
        "a@a.com"
      ]
    },
    "sendgrid": {
      "enabled": true,
      "api_key": "apikey",
      "sender_address": "test@test.com",
      "sender_name": "Test",
      "recipients": [
        "test@test.com",
        "a@a.com"
      ]
    },
    "msteams": {
      "enabled": true,
      "webhooks": [
        "url1",
        "url2"
      ]
    }
  }
}`, tmpFile.Name())

	f, err := os.CreateTemp("", "config")
	require.Nil(t, err)
	tmpFilename := f.Name()
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			require.Nil(t, err)
		}
	}(f)
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			require.Nil(t, err)
		}
	}(tmpFilename)
	_, err = f.WriteString(config)
	require.Nil(t, err)

	c, err := getConfig(tmpFilename)
	require.Nil(t, err)

	require.Len(t, c.Files, 2)
	require.Equal(t, tmpFile.Name(), c.Files[0].FileName)
	require.Len(t, c.Files[0].Watches, 3)
	require.Equal(t, c.Files[0].Watches[0], "match1")
	require.Equal(t, c.Files[0].Watches[1], "match2")
	require.Equal(t, c.Files[0].Watches[2], "match3")

	require.Equal(t, tmpFile.Name(), c.Files[1].FileName)
	require.Len(t, c.Files[1].Watches, 3)
	require.Equal(t, c.Files[1].Watches[0], "match1")
	require.Equal(t, c.Files[1].Watches[1], "match2")
	require.Equal(t, c.Files[1].Watches[2], "match3")
	require.Len(t, c.Files[1].Excludes, 2)
	require.Equal(t, c.Files[1].Excludes[0], "exclude1")
	require.Equal(t, c.Files[1].Excludes[1], "exclude2")

	require.Len(t, c.Notifications.Telegram.ChatIDs, 2)
	require.Equal(t, int64(1), c.Notifications.Telegram.ChatIDs[0])
	require.Equal(t, int64(2), c.Notifications.Telegram.ChatIDs[1])
	require.Equal(t, "token", c.Notifications.Telegram.APIToken)

	require.Len(t, c.Notifications.Discord.ChannelIDs, 2)
	require.Equal(t, "1", c.Notifications.Discord.ChannelIDs[0])
	require.Equal(t, "2", c.Notifications.Discord.ChannelIDs[1])
	require.Equal(t, "token", c.Notifications.Discord.BotToken)
	require.Equal(t, "token", c.Notifications.Discord.OAuthToken)

	require.Equal(t, "test@test.com", c.Notifications.Email.Sender)
	require.Equal(t, "smtp.server.com", c.Notifications.Email.Server)
	require.Equal(t, 25, c.Notifications.Email.Port)
	require.Equal(t, "user", c.Notifications.Email.Username)
	require.Equal(t, "pass", c.Notifications.Email.Password)
	require.Len(t, c.Notifications.Email.Recipients, 2)
	require.Equal(t, "test@test.com", c.Notifications.Email.Recipients[0])
	require.Equal(t, "a@a.com", c.Notifications.Email.Recipients[1])

	require.Equal(t, "apikey", c.Notifications.SendGrid.APIKey)
	require.Equal(t, "test@test.com", c.Notifications.SendGrid.SenderAddress)
	require.Equal(t, "Test", c.Notifications.SendGrid.SenderName)
	require.Len(t, c.Notifications.SendGrid.Recipients, 2)
	require.Equal(t, "test@test.com", c.Notifications.SendGrid.Recipients[0])
	require.Equal(t, "a@a.com", c.Notifications.SendGrid.Recipients[1])

	require.Len(t, c.Notifications.MSTeams.Webhooks, 2)
	require.Equal(t, "url1", c.Notifications.MSTeams.Webhooks[0])
	require.Equal(t, "url2", c.Notifications.MSTeams.Webhooks[1])
}
