// Package vfat checks whether a file name can be safely written to
// a VFAT (FAT32) filesystem. It is intentionally conservative: names
// that VFAT would technically accept may still be rejected here, but
// every name that passes this check is guaranteed to be writable.
package vfat

import (
	"fmt"
	"strings"
)

const (
	// MaxLen is the maximum length of a file name on VFAT. This is
	// the limit for the whole name including the extension.
	MaxLen = 255

	// forbidden contains characters that VFAT does not allow in
	// file names. The set matches the specification from Microsoft.
	forbidden = `\/:*?"<>|`
)

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
