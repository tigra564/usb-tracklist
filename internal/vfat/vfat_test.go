package vfat

import (
	"strings"
	"testing"
)

func TestCheck_Valid(t *testing.T) {
	valid := []string{
		"01 Song.mp3",
		"Track.mp3",
		"a.mp3",
		"01 - La camisa negra.mp3",
		"Кириллица.mp3",
		"foo.bar.mp3",
		"CONSOLE.mp3",
		"con artist.mp3",
		strings.Repeat("a", 255),
	}
	for _, name := range valid {
		if err := Check(name); err != nil {
			t.Errorf("Check(%q) = %v, want nil", name, err)
		}
	}
}

func TestCheck_Empty(t *testing.T) {
	if err := Check(""); err == nil {
		t.Error("Check(\"\") = nil, want error")
	}
}

func TestCheck_TooLong(t *testing.T) {
	name := strings.Repeat("a", 256)
	if err := Check(name); err == nil {
		t.Error("Check(256 chars) = nil, want error")
	}
}

func TestCheck_ForbiddenChars(t *testing.T) {
	// Every forbidden character, in a name that is otherwise valid.
	for _, r := range `\/:*?"<>|` {
		name := "a" + string(r) + "b.mp3"
		if err := Check(name); err == nil {
			t.Errorf("Check(%q) = nil, want error", name)
		}
	}
}

func TestCheck_ControlChar(t *testing.T) {
	name := "a\x01b.mp3"
	if err := Check(name); err == nil {
		t.Error("Check with control char = nil, want error")
	}
}

func TestCheck_LeadingSpace(t *testing.T) {
	if err := Check(" song.mp3"); err == nil {
		t.Error("Check(leading space) = nil, want error")
	}
}

func TestCheck_LeadingDot(t *testing.T) {
	if err := Check(".song.mp3"); err == nil {
		t.Error("Check(leading dot) = nil, want error")
	}
}

func TestCheck_TrailingSpace(t *testing.T) {
	if err := Check("song.mp3 "); err == nil {
		t.Error("Check(trailing space) = nil, want error")
	}
}

func TestCheck_TrailingDot(t *testing.T) {
	if err := Check("song.mp3."); err == nil {
		t.Error("Check(trailing dot) = nil, want error")
	}
}

func TestCheck_ReservedNames(t *testing.T) {
	// Reserved names are case-insensitive and apply to the stem
	// before the first dot.
	reserved := []string{
		"CON", "con", "Con.mp3", "CON.txt",
		"PRN", "AUX", "NUL",
		"COM1", "com9.mp3", "LPT1", "lpt9.txt",
	}
	for _, name := range reserved {
		if err := Check(name); err == nil {
			t.Errorf("Check(%q) = nil, want error", name)
		}
	}
}

func TestCheck_ReservedNameAsSubstring(t *testing.T) {
	// A reserved name inside a longer stem is fine.
	valid := []string{
		"CONSOLE.mp3",
		"COM10.mp3",
		"LPT0.mp3",
		"con artist.mp3",
	}
	for _, name := range valid {
		if err := Check(name); err != nil {
			t.Errorf("Check(%q) = %v, want nil", name, err)
		}
	}
}
