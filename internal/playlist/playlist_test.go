package playlist

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tigra564/usb-tracklist/internal/fsops"
	"github.com/tigra564/usb-tracklist/internal/tags"
)

// fakeReader implements tags.Reader from a map from path to Tags.
type fakeReader struct {
	tags map[string]tags.Tags
}

func (f *fakeReader) Read(path string, typ string) (tags.Tags, error) {
	t, ok := f.tags[path]
	if !ok {
		return tags.Tags{}, errNoTags
	}
	return t, nil
}

var errNoTags = &fakeError{"no tags for path"}

type fakeError struct{ msg string }

func (e *fakeError) Error() string { return e.msg }

// ---------- Read / Write ----------

func TestReadWriteRoundTrip(t *testing.T) {
	original := PlayList{
		{
			FileName: "01 Song.mp3",
			Number:   "01",
			Title:    "Song",
			Artist:   "Artist",
			Album:    "Album",
			Type:     "mp3",
			Hash:     "abc123",
		},
		{
			FileName: "02 Other.mp3",
			Number:   "02",
			Title:    "Other",
			Artist:   "Другой",
			Album:    "Альбом",
			Type:     "mp3",
			Hash:     "def456",
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, original); err != nil {
		t.Fatal(err)
	}

	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(original) {
		t.Fatalf("got %d songs, want %d", len(got), len(original))
	}
	for i := range original {
		if got[i] != original[i] {
			t.Errorf("song %d: got %+v, want %+v", i, got[i], original[i])
		}
	}
}

func TestWrite_BOM(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("output does not start with UTF-8 BOM")
	}
}

func TestWrite_HeaderOrder(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, nil); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	want := "File Name;Track Number;Title;Artist;Album;Type;Hash"
	if !strings.Contains(body, want) {
		t.Errorf("header missing or wrong: %q", body)
	}
}

func TestRead_Empty(t *testing.T) {
	list, err := Read(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("got %d songs, want 0", len(list))
	}
}

func TestRead_BOM(t *testing.T) {
	body := "\ufeffFile Name;Track Number;Title;Artist;Album;Type;Hash\r\n" +
		"01 Song.mp3;01;Song;Artist;Album;mp3;abc\r\n"
	list, err := Read(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d songs, want 1", len(list))
	}
	if list[0].FileName != "01 Song.mp3" {
		t.Errorf("FileName = %q, want %q",
			list[0].FileName, "01 Song.mp3")
	}
	if list[0].Type != "mp3" {
		t.Errorf("Type = %q, want %q", list[0].Type, "mp3")
	}
}

func TestRead_WrongHeader(t *testing.T) {
	body := "Foo;Bar;Baz;Qux;Quux;Corge;Grault\r\n"
	_, err := Read(strings.NewReader(body))
	if err == nil {
		t.Error("expected error for wrong header")
	}
}

func TestRead_FieldCountMismatch(t *testing.T) {
	body := "File Name;Track Number;Title;Artist;Album;Type;Hash\r\n" +
		"01 Song.mp3;01;Song;Artist\r\n"
	_, err := Read(strings.NewReader(body))
	if err == nil {
		t.Error("expected error for too few fields")
	}
}

// ---------- FromFileList ----------

func TestFromFileList_NaturalOrder(t *testing.T) {
	dir := "/music"
	files := []fsops.AudioFile{
		{Name: "track10.mp3", PermanentName: "track10.mp3", Type: "mp3"},
		{Name: "track2.mp3", PermanentName: "track2.mp3", Type: "mp3"},
		{Name: "track1.mp3", PermanentName: "track1.mp3", Type: "mp3"},
	}
	reader := &fakeReader{tags: map[string]tags.Tags{
		filepath.Join(dir, "track1.mp3"):  {Title: "One"},
		filepath.Join(dir, "track2.mp3"):  {Title: "Two"},
		filepath.Join(dir, "track10.mp3"): {Title: "Ten"},
	}}

	var list PlayList
	if err := list.FromFileList(dir, files, reader); err != nil {
		t.Fatal(err)
	}
	want := []string{"track1.mp3", "track2.mp3", "track10.mp3"}
	for i, w := range want {
		if list[i].FileName != w {
			t.Errorf("position %d: got %s, want %s",
				i, list[i].FileName, w)
		}
	}
}

func TestFromFileList_TrackFromTags(t *testing.T) {
	dir := "/music"
	files := []fsops.AudioFile{
		{Name: "05 Song.mp3", PermanentName: "05 Song.mp3", Type: "mp3"},
	}
	reader := &fakeReader{tags: map[string]tags.Tags{
		filepath.Join(dir, "05 Song.mp3"): {
			Track: "5/12",
			Title: "Song",
		},
	}}

	var list PlayList
	if err := list.FromFileList(dir, files, reader); err != nil {
		t.Fatal(err)
	}
	if list[0].Number != "05" {
		t.Errorf("Number = %q, want %q", list[0].Number, "05")
	}
}

func TestFromFileList_TypeCopied(t *testing.T) {
	dir := "/music"
	files := []fsops.AudioFile{
		{Name: "01 Song.mp3", PermanentName: "01 Song.mp3", Type: "mp3"},
	}
	reader := &fakeReader{tags: map[string]tags.Tags{
		filepath.Join(dir, "01 Song.mp3"): {Title: "Song"},
	}}

	var list PlayList
	if err := list.FromFileList(dir, files, reader); err != nil {
		t.Fatal(err)
	}
	if list[0].Type != "mp3" {
		t.Errorf("Type = %q, want %q", list[0].Type, "mp3")
	}
}

func TestFromFileList_HashComputed(t *testing.T) {
	dir := "/music"
	files := []fsops.AudioFile{
		{Name: "01 Song.mp3", PermanentName: "01 Song.mp3", Type: "mp3"},
	}
	reader := &fakeReader{tags: map[string]tags.Tags{
		filepath.Join(dir, "01 Song.mp3"): {
			Track:  "01",
			Title:  "Song",
			Artist: "Artist",
			Album:  "Album",
		},
	}}

	var list PlayList
	if err := list.FromFileList(dir, files, reader); err != nil {
		t.Fatal(err)
	}
	if list[0].Hash == "" {
		t.Error("Hash is empty")
	}
	if len(list[0].Hash) != 32 {
		t.Errorf("Hash length = %d, want 32", len(list[0].Hash))
	}
}

func TestFromFileList_PermanentNameUsed(t *testing.T) {
	// The file has a tempSuffix on disk, but the playlist must
	// record the permanent name.
	dir := "/music"
	files := []fsops.AudioFile{
		{
			Name:          "04.mp3.usb-tracklist-tmp",
			PermanentName: "04.mp3",
			Type:          "mp3",
		},
	}
	reader := &fakeReader{tags: map[string]tags.Tags{
		filepath.Join(dir, "04.mp3.usb-tracklist-tmp"): {
			Track: "04",
			Title: "Song",
		},
	}}

	var list PlayList
	if err := list.FromFileList(dir, files, reader); err != nil {
		t.Fatal(err)
	}
	if list[0].FileName != "04.mp3" {
		t.Errorf("FileName = %q, want %q", list[0].FileName, "04.mp3")
	}
}

func TestFromFileList_ReadError(t *testing.T) {
	dir := "/music"
	files := []fsops.AudioFile{
		{Name: "01 Song.mp3", PermanentName: "01 Song.mp3", Type: "mp3"},
	}
	reader := &fakeReader{tags: map[string]tags.Tags{}}

	var list PlayList
	err := list.FromFileList(dir, files, reader)
	if err == nil {
		t.Error("expected error when reader fails")
	}
}
