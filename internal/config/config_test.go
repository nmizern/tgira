package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const minimal = `
token: "123:abc"
boards:
  - code: TG
    title: Tasks
    chat_id: -1001234567890
    thread_id: 42
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadFillsDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimal))
	require.NoError(t, err)

	require.Equal(t, "123:abc", cfg.Token)
	require.Equal(t, "./tgira.db", cfg.DBPath)
	require.Equal(t, "en", cfg.Locale)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, PriorityEdges, cfg.Parse.PriorityPosition)
	require.True(t, cfg.UI.Buttons)
	require.True(t, cfg.UI.PinBoard)
	require.Equal(t, time.Minute, cfg.UI.EphemeralTTL())
	require.Equal(t, 2*time.Second, cfg.UI.BoardDebounce())
}

// Defaults are applied before unmarshalling, so an explicit false must survive.
func TestLoadKeepsExplicitFalse(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimal+"ui:\n  buttons: false\n  pin_board: false\n"))
	require.NoError(t, err)

	require.False(t, cfg.UI.Buttons)
	require.False(t, cfg.UI.PinBoard)
}

func TestLoadEnvOverridesFile(t *testing.T) {
	t.Setenv("TGIRA_TOKEN", "env:token")
	t.Setenv("TGIRA_DB_PATH", "/var/lib/tgira/tgira.db")
	t.Setenv("TGIRA_LOCALE", "ru")
	t.Setenv("TGIRA_LOG_LEVEL", "debug")
	t.Setenv("TGIRA_BUTTONS", "false")
	t.Setenv("TGIRA_BOARD_DEBOUNCE_MS", "500")

	cfg, err := Load(writeConfig(t, minimal))
	require.NoError(t, err)

	require.Equal(t, "env:token", cfg.Token)
	require.Equal(t, "/var/lib/tgira/tgira.db", cfg.DBPath)
	require.Equal(t, "ru", cfg.Locale)
	require.Equal(t, "debug", cfg.LogLevel)
	require.False(t, cfg.UI.Buttons)
	require.Equal(t, 500, cfg.UI.BoardDebounceMS)
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	require.Error(t, err)
}

func TestBoardTitleDefaultsToCode(t *testing.T) {
	cfg, err := Load(writeConfig(t, "token: \"123:abc\"\nboards:\n  - code: TG\n    chat_id: -100123\n    thread_id: 0\n"))
	require.NoError(t, err)
	require.Equal(t, "TG", cfg.Boards[0].Title)
}

func good() Config {
	cfg := Defaults()
	cfg.Token = "123:abc"
	cfg.Boards = []Board{{Code: "TG", Title: "Tasks", ChatID: -1001234567890, ThreadID: 42}}
	return cfg
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name  string
		mutit func(*Config)
		want  string
	}{
		{"ok", func(*Config) {}, ""},
		{"no token", func(c *Config) { c.Token = "" }, "token"},
		// a fresh install has no boards yet; /whereami still has to answer
		{"no boards yet", func(c *Config) { c.Boards = nil }, ""},
		{"lowercase code", func(c *Config) { c.Boards[0].Code = "tg" }, "code"},
		{"code too long", func(c *Config) { c.Boards[0].Code = "VERYLONGCODE" }, "code"},
		{"code with dash", func(c *Config) { c.Boards[0].Code = "T-G" }, "code"},
		{"duplicate code", func(c *Config) {
			c.Boards = append(c.Boards, Board{Code: "TG", ChatID: -100999, ThreadID: 1})
		}, "duplicate"},
		{"duplicate thread", func(c *Config) {
			c.Boards = append(c.Boards, Board{Code: "OPS", ChatID: -1001234567890, ThreadID: 42})
		}, "duplicate"},
		{"positive chat id", func(c *Config) { c.Boards[0].ChatID = 123 }, "chat_id"},
		{"negative thread id", func(c *Config) { c.Boards[0].ThreadID = -1 }, "thread_id"},
		{"unknown locale", func(c *Config) { c.Locale = "de" }, "locale"},
		{"unknown log level", func(c *Config) { c.LogLevel = "trace" }, "log_level"},
		{"unknown priority position", func(c *Config) { c.Parse.PriorityPosition = "middle" }, "priority_position"},
		{"negative ttl", func(c *Config) { c.UI.EphemeralTTLSeconds = -1 }, "ephemeral_ttl_seconds"},
		{"negative debounce", func(c *Config) { c.UI.BoardDebounceMS = -1 }, "board_debounce_ms"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := good()
			tc.mutit(&cfg)
			err := cfg.Validate()
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.want)
		})
	}
}
