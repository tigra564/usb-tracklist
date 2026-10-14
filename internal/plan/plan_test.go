package plan

import (
	"strings"
	"testing"

	"github.com/tigra564/usb-tracklist/internal/naming"
	"github.com/tigra564/usb-tracklist/internal/playlist"
)

// song is a helper to build a Song with the hash already computed.
func song(name, number, title, artist, album, typ string) playlist.Song {
	return playlist.Song{
		FileName: name,
		Number:   number,
		Title:    title,
		Artist:   artist,
		Album:    album,
		Type:     typ,
		Hash:     naming.HashTags(title, artist, album),
	}
}

func TestBuildPlan_Empty(t *testing.T) {
	p, w, err := BuildPlan(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tags) != 0 || len(p.Renames) != 0 || len(p.Finalize) != 0 {
		t.Errorf("plan is not empty: %+v", p)
	}
	if len(w) != 0 {
		t.Errorf("warnings = %v, want none", w)
	}
}

func TestBuildPlan_AlreadySynced(t *testing.T) {
	d := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
		song("02 Other.mp3", "02", "Other", "Artist", "Album", "mp3"),
	}
	p := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
		song("02 Other.mp3", "02", "Other", "Artist", "Album", "mp3"),
	}

	plan, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}
	if len(plan.Tags) != 0 {
		t.Errorf("unexpected tag changes: %+v", plan.Tags)
	}
	if len(plan.Renames) != 0 {
		t.Errorf("unexpected renames: %+v", plan.Renames)
	}
}

func TestBuildPlan_TagsDirty(t *testing.T) {
	d := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
	}
	p := playlist.PlayList{
		song("01 Song.mp3", "01", "New Title", "Artist", "Album", "mp3"),
	}
	// The playlist still carries the hash of the OLD tags,
	// because the user edited Title by hand in the CSV.
	p[0].Hash = naming.HashTags("Song", "Artist", "Album")

	plan, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}
	if len(plan.Tags) != 1 {
		t.Fatalf("got %d tag changes, want 1", len(plan.Tags))
	}
	if plan.Tags[0].Title != "New Title" {
		t.Errorf("Title = %q, want %q",
			plan.Tags[0].Title, "New Title")
	}
}

func TestBuildPlan_RenameByNumber(t *testing.T) {
	d := playlist.PlayList{
		song("05 Song.mp3", "05", "Song", "Artist", "Album", "mp3"),
	}
	p := playlist.PlayList{
		song("05 Song.mp3", "05", "Song", "Artist", "Album", "mp3"),
	}

	plan, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}
	if len(plan.Renames) != 1 {
		t.Fatalf("got %d renames, want 1", len(plan.Renames))
	}
	r := plan.Renames[0]
	if r.OldName != "05 Song.mp3" || r.NewName != "01 Song.mp3" {
		t.Errorf("rename = %s → %s, want 05 Song.mp3 → 01 Song.mp3",
			r.OldName, r.NewName)
	}
}

func TestBuildPlan_RenameByHash(t *testing.T) {
	// The directory file was renamed by hand; it matches the
	// playlist by tag hash. The new base name comes from the
	// directory, not from the playlist.
	d := playlist.PlayList{
		song("Renamed.mp3", "05", "Song", "Artist", "Album", "mp3"),
	}
	p := playlist.PlayList{
		song("05 Song.mp3", "05", "Song", "Artist", "Album", "mp3"),
	}

	plan, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}
	if len(plan.Renames) != 1 {
		t.Fatalf("got %d renames, want 1", len(plan.Renames))
	}
	r := plan.Renames[0]
	if r.OldName != "Renamed.mp3" || r.NewName != "01 Renamed.mp3" {
		t.Errorf("rename = %s → %s, want Renamed.mp3 → 01 Renamed.mp3",
			r.OldName, r.NewName)
	}
}

func TestBuildPlan_LeftP(t *testing.T) {
	d := playlist.PlayList{}
	p := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
	}

	_, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, msg := range w {
		if strings.Contains(msg, "in playlist but not in directory") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected leftP warning, got %v", w)
	}
}

func TestBuildPlan_LeftD(t *testing.T) {
	d := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
		song("02 New.mp3", "02", "New", "Artist2", "Album2", "mp3"),
	}
	p := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
	}

	plan, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}
	// The new file should get a rename and tag changes.
	if len(plan.Tags) != 1 {
		t.Errorf("got %d tag changes, want 1", len(plan.Tags))
	}
	// Find the rename for the new file.
	found := false
	for _, r := range plan.Renames {
		if r.OldName == "02 New.mp3" && r.NewName == "02 New.mp3" {
			found = true
		}
	}
	if !found && len(plan.Renames) > 0 {
		t.Errorf("expected rename for new file, got %+v", plan.Renames)
	}
	// No rename needed since it's already number 02, but tags
	// must be updated.
}

func TestBuildPlan_Swap(t *testing.T) {
	// Two files with the SAME base name, only the prefix differs.
	// Swapping their order forces a two-phase rename.
	d := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
		song("02 Song.mp3", "02", "Song", "Artist", "Album", "mp3"),
	}
	// Playlist wants them in reverse order.
	p := playlist.PlayList{
		song("02 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
		song("01 Song.mp3", "02", "Song", "Artist", "Album", "mp3"),
	}

	plan, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}
	if len(plan.Renames) < 2 {
		t.Fatalf("got %d renames, want at least 2", len(plan.Renames))
	}
	if len(plan.Finalize) == 0 {
		t.Errorf("expected finalize entries for a swap")
	}
}

func TestBuildPlan_TypeMismatch(t *testing.T) {
	d := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "mp3"),
	}
	p := playlist.PlayList{
		song("01 Song.mp3", "01", "Song", "Artist", "Album", "ogg"),
	}

	_, w, err := BuildPlan(d, p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, msg := range w {
		if strings.Contains(msg, "type mismatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected type mismatch warning, got %v", w)
	}
}

func TestOrderRenames_Direct(t *testing.T) {
	candidates := []RenameChange{
		{OldName: "01.mp3", NewName: "03.mp3"},
		{OldName: "02.mp3", NewName: "04.mp3"},
	}
	changes, finalize := orderRenames(candidates)
	if len(changes) != 2 {
		t.Errorf("got %d changes, want 2", len(changes))
	}
	if len(finalize) != 0 {
		t.Errorf("got %d finalize entries, want 0", len(finalize))
	}
}

func TestOrderRenames_Swap(t *testing.T) {
	candidates := []RenameChange{
		{OldName: "01.mp3", NewName: "02.mp3"},
		{OldName: "02.mp3", NewName: "01.mp3"},
	}
	changes, finalize := orderRenames(candidates)
	if len(changes) != 2 {
		t.Errorf("got %d changes, want 2", len(changes))
	}
	if len(finalize) != 1 {
		t.Errorf("got %d finalize entries, want 1", len(finalize))
	}
}
