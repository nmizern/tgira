package parse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestText16LengthMatchesTelegramCounting(t *testing.T) {
	require.Equal(t, 5, newText16("hello").len())
	// Cyrillic letters are one code unit each
	require.Equal(t, 6, newText16("привет").len())
	// anything outside the basic plane takes two
	require.Equal(t, 2, newText16("🎉").len())
	require.Equal(t, 5, newText16("🎉 hi").len())
}

func TestText16Slice(t *testing.T) {
	text := newText16("🎉 сделать #ship")
	require.Equal(t, "#ship", text.slice(Span{Offset: 11, Length: 5}))
	require.Equal(t, "сделать", text.slice(Span{Offset: 3, Length: 7}))

	// out of range spans are clamped instead of panicking
	require.Equal(t, "", text.slice(Span{Offset: 100, Length: 5}))
	require.Equal(t, "#ship", text.slice(Span{Offset: 11, Length: 999}))
}

func TestText16Cut(t *testing.T) {
	text := newText16("🎉 сделать релиз #ship")
	require.Equal(t, "🎉 сделать релиз ", text.cut([]Span{{Offset: 17, Length: 5}}))

	// overlapping and unsorted spans are handled
	require.Equal(t, "сделать релиз #ship", newText16("🎉 сделать релиз #ship").
		cut([]Span{{Offset: 0, Length: 3}}))
	require.Equal(t, "🎉  релиз ", newText16("🎉 сделать релиз #ship").
		cut([]Span{{Offset: 17, Length: 5}, {Offset: 3, Length: 7}}))
}
