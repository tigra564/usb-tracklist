// Package vfat checks whether a file name can be safely written to
// a VFAT (FAT32) filesystem. It is intentionally conservative: names
// that VFAT would technically accept may still be rejected here, but
// every name that passes this check is guaranteed to be writable.
package vfat

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// MaxLen is the maximum length of a file name on VFAT. This is
	// the limit for the whole name including the extension.
	MaxLen = 255

	// forbidden contains characters that VFAT does not allow in
	// file names. The set matches the specification from Microsoft.
	forbidden = `\/:*?"<>|`
)

// forbiddenReplacers replaces every VFAT-forbidden character with
// an underscore. It is built from forbidden so that the two lists
// cannot diverge.
var forbiddenReplacers = newReplacer(forbidden, "_")

// newReplacer builds a strings.Replacer that replaces every
// character of chars with replacement.
func newReplacer(chars, replacement string) *strings.Replacer {
	pairs := make([]string, 0, 2*len(chars))
	for _, r := range chars {
		pairs = append(pairs, string(r), replacement)
	}
	return strings.NewReplacer(pairs...)
}

// SanitizeName makes a string safe to use as a VFAT file name.
// Forbidden characters are replaced by underscores, leading and
// trailing dots and spaces are removed, and the result is truncated
// to MaxLen bytes on a rune boundary.
//
// The caller is still responsible for checking the final name with
// Check, because the extension and the track number prefix are
// added later.
func SanitizeName(s string) string {
	s = forbiddenReplacers.Replace(s)
	s = strings.Trim(s, " .")
	return truncateUTF8(s, MaxLen)
}

// truncateUTF8 truncates s to at most maxBytes bytes, cutting on a
// rune boundary so that the result is valid UTF-8.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i]
		}
	}
	return ""
}

// reservedNames are device names that VFAT reserves regardless of
// extension. "CON.txt" is just as invalid as "CON".
var reservedNames = map[string]bool{
	"CON":  true,
	"PRN":  true,
	"AUX":  true,
	"NUL":  true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// Check returns nil if name can be written to a VFAT filesystem.
// Otherwise it returns an error describing the first problem found.
//
// The check is applied to the final file name, including extension,
// because VFAT treats the name and the extension as a single unit
// with a shared length limit.
func Check(name string) error {
	if name == "" {
		return fmt.Errorf("empty name")
	}

	if len(name) > MaxLen {
		return fmt.Errorf("length %d exceeds %d", len(name), MaxLen)
	}

	if i := strings.IndexAny(name, forbidden); i >= 0 {
		return fmt.Errorf("forbidden character %q", name[i])
	}

	for _, r := range name {
		if r < 32 {
			return fmt.Errorf("control character (code %d)", r)
		}
	}

	if name[0] == ' ' || name[0] == '.' {
		return fmt.Errorf("name starts with space or dot")
	}
	if name[len(name)-1] == ' ' || name[len(name)-1] == '.' {
		return fmt.Errorf("name ends with space or dot")
	}

	// The part before the first dot must not be a reserved name.
	stem := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		stem = name[:i]
	}
	if reservedNames[strings.ToUpper(stem)] {
		return fmt.Errorf("reserved name %q", stem)
	}

	return nil
}
