package tg

import (
	"strconv"
)

func itoa(v int) string { return strconv.Itoa(v) }

func atoi(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
