package naming

import (
	"strings"
	"testing"
)

func TestBasePart(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"05 Song.mp3", "Song.mp3"},
		{"05-Song.mp3", "Song.mp3"},
		{"05_Song.mp3", "Song.mp3"},
		{"05 - Song.mp3", "Song.mp3"},
		{"05 -_- Song.mp3", "Song.mp3"},
		{"05Song.mp3", "Song.mp3"},
		{"5 Song.mp3", "Song.mp3"},
		{"005 Song.mp3", "Song.mp3"},
		{"Song.mp3", "Song.mp3"},
		{"Song.ogg", "Song.ogg"},
		{"05 Song.ogg", "Song.ogg"},
		{"05 Song.MP3", "Song.MP3"},
		{"", ""},
		{"05.mp3", ".mp3"},
		{"1000 miles.mp3", "miles.mp3"},
		{"04.mp3.usb-tracklist-tmp", ".mp3.usb-tracklist-tmp"},
	}
	for _, c := range cases {
		if got := BasePart(c.in); got != c.want {
			t.Errorf("BasePart(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHasAudioExtension(t *testing.T) {
	yes := []string{"song.mp3", "song.MP3", "Song.Mp3"}
	for _, name := range yes {
		if !HasAudioExtension(name) {
			t.Errorf("HasAudioExtension(%q) = false, want true", name)
		}
	}
	no := []string{"song.ogg", "song.wav", "song", "", "song.txt",
		"song.mp3.usb-tracklist-tmp"}
	for _, name := range no {
		if HasAudioExtension(name) {
			t.Errorf("HasAudioExtension(%q) = true, want false", name)
		}
	}
}

func TestPrefixWidth(t *testing.T) {
	cases := []struct {
		count int
		want  int
	}{
		{0, 2},
		{1, 2},
		{9, 2},
		{10, 2},
		{99, 2},
		{100, 3},
		{999, 3},
		{1000, 4},
		{9999, 4},
	}
	for _, c := range cases {
		got, err := PrefixWidth(c.count)
		if err != nil {
			t.Errorf("PrefixWidth(%d) returned error: %v", c.count, err)
			continue
		}
		if got != c.want {
			t.Errorf("PrefixWidth(%d) = %d, want %d",
				c.count, got, c.want)
		}
	}
}

func TestPrefixWidth_TooMany(t *testing.T) {
	for _, count := range []int{10000, 10001, 100000} {
		if _, err := PrefixWidth(count); err == nil {
			t.Errorf("PrefixWidth(%d) = nil error, want error", count)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		index int
		width int
		want  string
	}{
		{1, 2, "01"},
		{5, 2, "05"},
		{99, 2, "99"},
		{1, 3, "001"},
		{100, 3, "100"},
		{1, 4, "0001"},
		{9999, 4, "9999"},
	}
	for _, c := range cases {
		if got := FormatNumber(c.index, c.width); got != c.want {
			t.Errorf("FormatNumber(%d, %d) = %q, want %q",
				c.index, c.width, got, c.want)
		}
	}
}

func TestMakeNewName(t *testing.T) {
	cases := []struct {
		number string
		rest   string
		want   string
	}{
		{"01", "Song.mp3", "01 Song.mp3"},
		{"01", "La camisa negra.mp3", "01 La camisa negra.mp3"},
		{"01", ".mp3", "01.mp3"},
		{"01", "", "01"},
		{"01", "Song.ogg", "01 Song.ogg"},
		{"01", "Song.MP3", "01 Song.MP3"},
		{"001", "Song.mp3", "001 Song.mp3"},
		{"01", ".mp3.usb-tracklist-tmp", "01.mp3.usb-tracklist-tmp"},
	}
	for _, c := range cases {
		got := MakeNewName(c.number, c.rest)
		if got != c.want {
			t.Errorf("MakeNewName(%q, %q) = %q, want %q",
				c.number, c.rest, got, c.want)
		}
	}
}

func TestNaturalCompare(t *testing.T) {
	pairs := [][2]string{
		{"track2.mp3", "track10.mp3"},
		{"track 10.mp3", "track 11.mp3"},
		{"track 02.mp3", "track 10.mp3"},
		{"01 song.mp3", "02 song.mp3"},
		{"2abc", "abc"},
		{"track10 extra.mp3", "track10.mp3"},
		{"a", "b"},
		{"a", "aa"},
	}
	for _, p := range pairs {
		if c := NaturalCompare(p[0], p[1]); c >= 0 {
			t.Errorf("NaturalCompare(%q, %q) = %d, want < 0",
				p[0], p[1], c)
		}
		if c := NaturalCompare(p[1], p[0]); c <= 0 {
			t.Errorf("NaturalCompare(%q, %q) = %d, want > 0",
				p[1], p[0], c)
		}
	}
}

func TestNaturalCompare_Equal(t *testing.T) {
	equal := [][2]string{
		{"track10.mp3", "track10.mp3"},
		{"Track10.mp3", "track10.mp3"},
		{"Track.mp3", "track.mp3"},
		{"TRACK.MP3", "track.mp3"},
		{"", ""},
	}
	for _, p := range equal {
		if c := NaturalCompare(p[0], p[1]); c != 0 {
			t.Errorf("NaturalCompare(%q, %q) = %d, want 0",
				p[0], p[1], c)
		}
	}
}

func TestNaturalCompare_EmptyFirst(t *testing.T) {
	if c := NaturalCompare("", "a"); c >= 0 {
		t.Errorf("NaturalCompare(\"\", \"a\") = %d, want < 0", c)
	}
	if c := NaturalCompare("a", ""); c <= 0 {
		t.Errorf("NaturalCompare(\"a\", \"\") = %d, want > 0", c)
	}
}

func TestHashTags(t *testing.T) {
	h1 := HashTags("Song", "Artist", "Album")
	h2 := HashTags("Song", "Artist", "Album")
	if h1 != h2 {
		t.Errorf("HashTags is not deterministic: %q vs %q", h1, h2)
	}

	if len(h1) != 32 {
		t.Errorf("HashTags length = %d, want 32", len(h1))
	}
	if strings.Trim(h1, "0123456789abcdef") != "" {
		t.Errorf("HashTags = %q, want lowercase hex", h1)
	}

	variants := []string{
		HashTags("Other", "Artist", "Album"),
		HashTags("Song", "Other", "Album"),
		HashTags("Song", "Artist", "Other"),
	}
	for _, v := range variants {
		if v == h1 {
			t.Errorf("HashTags returned the same hash for different input")
		}
	}
}

func TestHashTags_CaseSensitive(t *testing.T) {
	// Tag values are case-sensitive: "Juanes" and "juanes" are
	// different artists and must produce different hashes.
	h1 := HashTags("Song", "Juanes", "Album")
	h2 := HashTags("Song", "juanes", "Album")
	if h1 == h2 {
		t.Error("HashTags is case-insensitive, want case-sensitive")
	}
}

func TestIsTemp(t *testing.T) {
	temp := []string{
		"04.mp3" + TempSuffix,
		"Song.ogg" + TempSuffix,
		TempSuffix,
	}
	for _, name := range temp {
		if !IsTemp(name) {
			t.Errorf("IsTemp(%q) = false, want true", name)
		}
	}
	notTemp := []string{
		"04.mp3",
		"Song.ogg",
		"",
		"usb-tracklist-tmp", // no leading dot
		"04.mp3.tmp",        // different suffix
	}
	for _, name := range notTemp {
		if IsTemp(name) {
			t.Errorf("IsTemp(%q) = true, want false", name)
		}
	}
}

func TestTempSuffix(t *testing.T) {
	if !strings.HasPrefix(TempSuffix, ".") {
		t.Errorf("TempSuffix = %q, want leading dot", TempSuffix)
	}
	if strings.ContainsAny(TempSuffix, `\/:*?"<>|`) {
		t.Errorf("TempSuffix = %q contains VFAT-forbidden characters",
			TempSuffix)
	}
}
