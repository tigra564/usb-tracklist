// Package naming provides utilities for working with audio file
// names: stripping track number prefixes, comparing names naturally,
// and building target names.
package naming

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// prefixRe strips a leading track number together with any sequence
// of spaces, dashes and underscores that follows it. "05-Song",
// "05 - Song", and "05 -_- Song" all yield "Song".
var prefixRe = regexp.MustCompile(`^[0-9]+[-_ ]*`)

// tokenRe splits a string into alternating runs of digits and
// non-digits. Used for natural sorting: "track10" must come after
// "track2", and "2abc" must come before "abc".
var tokenRe = regexp.MustCompile(`[0-9]+|[^0-9]+`)

// audioExtensions lists the file extensions that this tool
// recognizes. Currently only MP3, but the list is expected to grow.
var audioExtensions = map[string]bool{
	".mp3": true,
}

// HasAudioExtension reports whether name has one of the recognized
// audio extensions, case-insensitively. It is the only function in
// this package that treats the extension as a separate entity; all
// other functions keep it as part of the name.
func HasAudioExtension(name string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(name))]
}

// TagsTempSuffix is appended to a file name while its tags are
// being rewritten. Files with this suffix are discarded if a run
// is interrupted.
const TagsTempSuffix = ".usb-tracklist-newtags.tmp"

// TempSuffix is appended to a file name when a rename cannot be
// done directly because the target name is occupied. Files with
// this suffix are left over from an interrupted run and are
// finalized on the next run.
const TempSuffix = ".usb-tracklist-tmp"

// IsTemp reports whether name ends with TempSuffix.
func IsTemp(name string) bool {
	return strings.HasSuffix(name, TempSuffix)
}

// BasePart strips the leading track number from a file name.
// The extension is NOT removed: the whole name after the prefix is
// what identifies a file across the playlist and the directory.
func BasePart(name string) string {
	loc := prefixRe.FindStringIndex(name)
	if loc == nil {
		return name
	}
	return name[loc[1]:]
}

// PrefixWidth returns the number of digits used for the track number
// prefix. Minimum is 2 ("01"), maximum is 4 ("0001").
//
// Returns an error if count is larger than 9999, because a 5-digit
// prefix would exceed the width that this tool is willing to emit.
func PrefixWidth(count int) (int, error) {
	switch {
	case count < 100:
		return 2, nil
	case count < 1000:
		return 3, nil
	case count < 10000:
		return 4, nil
	default:
		return 0, fmt.Errorf("too many tracks: %d (maximum is 9999)", count)
	}
}

// FormatNumber formats a 1-based track index with the given width.
// FormatNumber(1, 2) == "01", FormatNumber(1, 4) == "0001".
func FormatNumber(index, width int) string {
	return fmt.Sprintf("%0*d", width, index)
}

// MakeNewName joins a track number with the rest of the file name.
// The rest is everything after the leading track number, including
// the extension. If the rest starts with a dot (empty base name),
// no space is inserted: MakeNewName("01", ".mp3") == "01.mp3".
func MakeNewName(number, rest string) string {
	if rest == "" {
		return number
	}
	if strings.HasPrefix(rest, ".") {
		return number + rest
	}
	return number + " " + rest
}

// NaturalCompare compares two strings using natural ordering:
// digit runs are compared as numbers, everything else as strings.
func NaturalCompare(a, b string) int {
	at := tokenRe.FindAllString(a, -1)
	bt := tokenRe.FindAllString(b, -1)

	// Empty input sorts before anything else. In practice both
	// strings are non-empty file names, but the check is cheap and
	// makes the function total.
	switch {
	case len(at) == 0 && len(bt) == 0:
		return 0
	case len(at) == 0:
		return -1
	case len(bt) == 0:
		return 1
	}

	// A leading digit run sorts before a leading non-digit run.
	aDigit := startsWithDigit(at[0])
	bDigit := startsWithDigit(bt[0])
	if aDigit != bDigit {
		if aDigit {
			return -1
		}
		return 1
	}

	n := len(at)
	if len(bt) < n {
		n = len(bt)
	}
	for i := 0; i < n; i++ {
		if c := compareTokens(at[i], bt[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(at) < len(bt):
		return -1
	case len(at) > len(bt):
		return 1
	default:
		return 0
	}
}

// startsWithDigit reports whether s begins with an ASCII digit.
// Callers guarantee s is non-empty.
func startsWithDigit(s string) bool {
	return s[0] >= '0' && s[0] <= '9'
}

// compareTokens compares two tokens from tokenRe.FindAllString.
// Numeric tokens are compared as integers; otherwise as
// case-insensitive strings.
func compareTokens(a, b string) int {
	if startsWithDigit(a) && startsWithDigit(b) {
		an, _ := strconv.Atoi(a)
		bn, _ := strconv.Atoi(b)
		switch {
		case an < bn:
			return -1
		case an > bn:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// HashTags returns the hex MD5 of the tag fields that this tool
// manages. The order is fixed: title, artist, album. The track
// number is NOT part of the hash: it is derived from the playlist
// order and can change between runs without any change to the
// underlying track.
//
// The hash is used to detect changes to the tag fields and to
// protect against file substitution. It is not a security measure.
func HashTags(title, artist, album string) string {
	joined := strings.Join([]string{title, artist, album}, "|")
	sum := md5.Sum([]byte(joined))
	return hex.EncodeToString(sum[:])
}

// NormalizeSpaces collapses runs of whitespace into a single space
// and trims leading and trailing whitespace. It is applied to tag
// values before they are written, so that " Album " and "Album"
// are treated as the same value.
func NormalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// NormalizeTrack brings a raw TRCK value to the canonical form
// used by this tool: a zero-padded number of the given width, or
// an empty string if the value is not a number. The "/total" part,
// if present, is dropped: "5/12", "5", and "05" all become "05"
// with width 2.
func NormalizeTrack(track string, width int) string {
	if i := strings.IndexByte(track, '/'); i >= 0 {
		track = track[:i]
	}
	n, err := strconv.Atoi(track)
	if err != nil || n < 1 {
		return ""
	}
	return FormatNumber(n, width)
}
