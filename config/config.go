package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Account struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
	RedirectURI  string `json:"redirect_uri"`
}

type DelayConfig struct {
	Enabled bool
	Min     int
	Max     int
}

func (d *DelayConfig) UnmarshalJSON(data []byte) error {
	var obj struct {
		Enabled bool `json:"enabled"`
		Min     int  `json:"min"`
		Max     int  `json:"max"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		d.Enabled = obj.Enabled
		d.Min = obj.Min
		d.Max = obj.Max
		return nil
	}

	var arr []int
	if err := json.Unmarshal(data, &arr); err != nil {
		return fmt.Errorf("delay config must be object {enabled,min,max} or array [enabled,min,max]: %w", err)
	}
	if len(arr) != 3 {
		return fmt.Errorf("delay array must have exactly 3 elements [enabled, min, max], got %d", len(arr))
	}
	d.Enabled = arr[0] != 0
	d.Min = arr[1]
	d.Max = arr[2]
	return nil
}

type ReadConfig struct {
	ApiRand     bool        `json:"api_rand"`
	Rounds      int         `json:"rounds"`
	RoundsDelay DelayConfig `json:"rounds_delay"`
	ApiDelay    DelayConfig `json:"api_delay"`
	AppDelay    DelayConfig `json:"app_delay"`
}

type WriteConfig struct {
	AllStart    bool        `json:"allstart"`
	Rounds      int         `json:"rounds"`
	RoundsDelay DelayConfig `json:"rounds_delay"`
	ApiDelay    DelayConfig `json:"api_delay"`
	AppDelay    DelayConfig `json:"app_delay"`
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type Config struct {
	Accounts    []Account      `json:"accounts"`
	Email       string         `json:"email"`
	City        string         `json:"city"`
	Telegram    TelegramConfig `json:"telegram"`
	ReadConfig  ReadConfig     `json:"read_config"`
	WriteConfig WriteConfig    `json:"write_config"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if len(cfg.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured")
	}

	for i := range cfg.Accounts {
		if cfg.Accounts[i].RedirectURI == "" {
			cfg.Accounts[i].RedirectURI = "http://localhost:53682/"
		}
	}

	if cfg.City == "" {
		cfg.City = "Beijing"
	}

	return &cfg, nil
}
