package utils

import (
	"fmt"
	"strings"
	"time"
)

func AgeSince(t time.Time) string {
	d := time.Since(t)

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

func Int64Ptr(i int64) *int64 { return &i }

func LastIndex(s, sep string) int {
	return strings.LastIndex(s, sep)
}

func MapValuesToSlice[K comparable, V any](m map[K]V) []V {
	out := make([]V, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func MapKeysToSlice[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
