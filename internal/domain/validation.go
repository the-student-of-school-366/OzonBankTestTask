package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// ValidateAuthor проверяет стабильный ID автора, не подтверждая его личность.
func ValidateAuthor(author string) error {
	return ValidateRequiredText(author, ErrEmptyAuthor)
}

func ValidateRequiredText(value string, emptyError error) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return ErrInvalidText
	}
	if strings.TrimSpace(value) == "" {
		return emptyError
	}
	return nil
}

func NormalizeTime(t time.Time) time.Time {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Truncate(time.Microsecond)
}

// Время должно представляться в API и без потерь сохраняться в PostgreSQL.
func ValidateTime(t time.Time) error {
	t = t.UTC()
	if t.IsZero() || t.Year() < 1 || t.Year() > 9999 || t.Nanosecond()%1000 != 0 {
		return ErrInvalidTime
	}
	return nil
}
