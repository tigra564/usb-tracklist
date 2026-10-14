package fsops

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tigra564/usb-tracklist/internal/naming"
)

func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func TestListFiles_OnlyAudio(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "01 Song.mp3")
	writeFile(t, dir, "notes.txt")
	writeFile(t, dir, "cover.jpg")
	writeFile(t, dir, "track.MP3")

	files, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	got := []string{files[0].Name, files[1].Name}
	sort.Strings(got)
	want := []string{"01 Song.mp3", "track.MP3"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("file %d = %q, want %q", i, got[i], w)
		}
	}
}

func TestListFiles_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir.mp3"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "01 Song.mp3")

	files, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if files[0].Name != "01 Song.mp3" {
		t.Errorf("got %q, want %q", files[0].Name, "01 Song.mp3")
	}
}

func TestListFiles_TempSuffix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "04.mp3"+naming.TempSuffix)

	files, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	f := files[0]
	if f.Name != "04.mp3"+naming.TempSuffix {
		t.Errorf("Name = %q, want %q",
			f.Name, "04.mp3"+naming.TempSuffix)
	}
	if f.PermanentName != "04.mp3" {
		t.Errorf("PermanentName = %q, want %q", f.PermanentName, "04.mp3")
	}
	if f.Type != "mp3" {
		t.Errorf("Type = %q, want %q", f.Type, "mp3")
	}
}

func TestListFiles_TypeLowercase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Song.MP3")

	files, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Type != "mp3" {
		t.Errorf("Type = %q, want %q", files[0].Type, "mp3")
	}
}

func TestRenameAll_Direct(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "01.mp3")
	writeFile(t, dir, "02.mp3")

	pairs := []RenamePair{
		{Old: "01.mp3", New: "03.mp3"},
		{Old: "02.mp3", New: "04.mp3"},
	}
	if err := RenameAll(dir, pairs); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	want := []string{"03.mp3", "04.mp3"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("file %d = %q, want %q", i, got[i], w)
		}
	}
}

func TestRenameAll_Swap(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "01.mp3")
	writeFile(t, dir, "02.mp3")

	pairs := []RenamePair{
		{Old: "01.mp3", New: "02.mp3"},
		{Old: "02.mp3", New: "01.mp3"},
	}
	if err := RenameAll(dir, pairs); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	want := []string{"01.mp3", "02.mp3"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("file %d = %q, want %q", i, got[i], w)
		}
	}
}

func TestRenameAll_SkipSame(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "01.mp3")

	pairs := []RenamePair{
		{Old: "01.mp3", New: "01.mp3"},
	}
	if err := RenameAll(dir, pairs); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	if len(got) != 1 || got[0] != "01.mp3" {
		t.Errorf("got %v, want [01.mp3]", got)
	}
}

func TestRenameAll_NoTempLeftover(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "01.mp3")
	writeFile(t, dir, "02.mp3")

	pairs := []RenamePair{
		{Old: "01.mp3", New: "02.mp3"},
		{Old: "02.mp3", New: "03.mp3"},
	}
	if err := RenameAll(dir, pairs); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	for _, name := range got {
		if strings.HasSuffix(name, naming.TempSuffix) {
			t.Errorf("temp file left behind: %s", name)
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2", len(got))
	}
}

func TestRenameAll_MissingSource(t *testing.T) {
	dir := t.TempDir()
	pairs := []RenamePair{
		{Old: "missing.mp3", New: "target.mp3"},
	}
	if err := RenameAll(dir, pairs); err == nil {
		t.Error("expected error for missing source")
	}
}
