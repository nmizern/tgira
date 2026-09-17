// Package config loads, defaults and validates the bot configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Priority parsing modes.
const (
	PriorityEdges    = "edges"
	PriorityAnywhere = "anywhere"
)

// Config is the whole configuration of a tgira instance.
type Config struct {
	Token    string  `yaml:"token"`
	DBPath   string  `yaml:"db_path"`
	Locale   string  `yaml:"locale"`
	LogLevel string  `yaml:"log_level"`
	Boards   []Board `yaml:"boards"`
	Parse    Parse   `yaml:"parse"`
	UI       UI      `yaml:"ui"`
}

// Board is one forum topic the bot watches.
type Board struct {
	Code     string `yaml:"code"`
	Title    string `yaml:"title"`
	ChatID   int64  `yaml:"chat_id"`
	ThreadID int64  `yaml:"thread_id"`
}

// Parse holds text parsing options.
type Parse struct {
	PriorityPosition string `yaml:"priority_position"`
}

// UI holds presentation options.
type UI struct {
	Buttons             bool `yaml:"buttons"`
	PinBoard            bool `yaml:"pin_board"`
	EphemeralTTLSeconds int  `yaml:"ephemeral_ttl_seconds"`
	BoardDebounceMS     int  `yaml:"board_debounce_ms"`
}

// EphemeralTTL is how long a service reply survives in a group.
func (u UI) EphemeralTTL() time.Duration {
	return time.Duration(u.EphemeralTTLSeconds) * time.Second
}

// BoardDebounce is how long board redraws are coalesced.
func (u UI) BoardDebounce() time.Duration {
	return time.Duration(u.BoardDebounceMS) * time.Millisecond
}

// Defaults returns a config with every optional field filled in.
func Defaults() Config {
	return Config{
		DBPath:   "./tgira.db",
		Locale:   "en",
		LogLevel: "info",
		Parse:    Parse{PriorityPosition: PriorityEdges},
		UI: UI{
			Buttons:             true,
			PinBoard:            true,
			EphemeralTTLSeconds: 60,
			BoardDebounceMS:     2000,
		},
	}
}

// Load reads the config file, applies TGIRA_* overrides and validates the result.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	// Unmarshalling over the defaults keeps an explicit "false" in the file.
	cfg := Defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.applyEnv(); err != nil {
		return Config{}, err
	}
	cfg.fill()

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyEnv() error {
	str := map[string]*string{
		"TGIRA_TOKEN":             &c.Token,
		"TGIRA_DB_PATH":           &c.DBPath,
		"TGIRA_LOCALE":            &c.Locale,
		"TGIRA_LOG_LEVEL":         &c.LogLevel,
		"TGIRA_PRIORITY_POSITION": &c.Parse.PriorityPosition,
	}
	for key, dst := range str {
		if v, ok := os.LookupEnv(key); ok {
			*dst = v
		}
	}

	nums := map[string]*int{
		"TGIRA_EPHEMERAL_TTL_SECONDS": &c.UI.EphemeralTTLSeconds,
		"TGIRA_BOARD_DEBOUNCE_MS":     &c.UI.BoardDebounceMS,
	}
	for key, dst := range nums {
		v, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		*dst = n
	}

	flags := map[string]*bool{
		"TGIRA_BUTTONS":   &c.UI.Buttons,
		"TGIRA_PIN_BOARD": &c.UI.PinBoard,
	}
	for key, dst := range flags {
		v, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		*dst = b
	}
	return nil
}

func (c *Config) fill() {
	for i := range c.Boards {
		if c.Boards[i].Title == "" {
			c.Boards[i].Title = c.Boards[i].Code
		}
	}
}

var codeRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,7}$`)

type thread struct {
	chatID   int64
	threadID int64
}

// Validate reports the first problem that would make the bot misbehave at
// runtime. A config without boards is allowed on purpose: that is how a fresh
// install starts, before /whereami has told anyone which topic to watch.
func (c Config) Validate() error {
	if c.Token == "" {
		return errors.New("token is required")
	}
	if !oneOf(c.Locale, "en", "ru") {
		return fmt.Errorf("locale: unknown value %q", c.Locale)
	}
	if !oneOf(c.LogLevel, "debug", "info", "warn", "error") {
		return fmt.Errorf("log_level: unknown value %q", c.LogLevel)
	}
	if !oneOf(c.Parse.PriorityPosition, PriorityEdges, PriorityAnywhere) {
		return fmt.Errorf("priority_position: unknown value %q", c.Parse.PriorityPosition)
	}
	if c.UI.EphemeralTTLSeconds < 0 {
		return errors.New("ephemeral_ttl_seconds must not be negative")
	}
	if c.UI.BoardDebounceMS < 0 {
		return errors.New("board_debounce_ms must not be negative")
	}

	codes := make(map[string]bool, len(c.Boards))
	threads := make(map[thread]bool, len(c.Boards))
	for _, b := range c.Boards {
		if !codeRe.MatchString(b.Code) {
			return fmt.Errorf("board code %q must be 1-8 uppercase letters or digits", b.Code)
		}
		if codes[b.Code] {
			return fmt.Errorf("duplicate board code %q", b.Code)
		}
		codes[b.Code] = true

		// Supergroup ids are negative; a positive one means the chat id was copied wrong.
		if b.ChatID >= 0 {
			return fmt.Errorf("board %s: chat_id must be a negative supergroup id", b.Code)
		}
		if b.ThreadID < 0 {
			return fmt.Errorf("board %s: thread_id must not be negative", b.Code)
		}

		key := thread{chatID: b.ChatID, threadID: b.ThreadID}
		if threads[key] {
			return fmt.Errorf("board %s: duplicate chat_id and thread_id", b.Code)
		}
		threads[key] = true
	}
	return nil
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}
