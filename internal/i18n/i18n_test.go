package i18n

import (
	"testing"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGetFallsBackToEnglish(t *testing.T) {
	require.Equal(t, "doing", Get("de").Status(domain.StatusDoing))
	require.Equal(t, "в работе", Get("ru").Status(domain.StatusDoing))
}

func TestTFormats(t *testing.T) {
	require.Equal(t, "3 open", Get("en").T(KeyBoardOpen, 3))
	require.Equal(t, "открытых: 3", Get("ru").T(KeyBoardOpen, 3))
}

func TestUnknownKeyIsVisible(t *testing.T) {
	require.Equal(t, "nope", Get("en").T("nope"))
}

// Every language must answer every key, or a locale switch silently loses text.
func TestLocalesCoverTheSameKeys(t *testing.T) {
	for key := range english.texts {
		require.Containsf(t, russian.texts, key, "ru is missing %s", key)
	}
	for key := range russian.texts {
		require.Containsf(t, english.texts, key, "en is missing %s", key)
	}
	for status := range english.statuses {
		require.Contains(t, russian.statuses, status)
	}
}
