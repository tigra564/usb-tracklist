// Package plan builds the synchronization plan: what changes are
// needed to make a directory of audio files match a playlist.
package plan

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tigra564/usb-tracklist/internal/naming"
	"github.com/tigra564/usb-tracklist/internal/playlist"
	"github.com/tigra564/usb-tracklist/internal/vfat"
)

// Options controls how the plan is built.
type Options struct {
	// NameFromTags, when true, builds the base part of each file
	// name from the track's Title and Artist tags instead of from
	// the current file name. The format is "Artist - Title".
	//
	// Every track must have both Title and Artist set; otherwise
	// the plan cannot be built and a warning is returned.
	NameFromTags bool
}

// TagsChange replaces the tags of a file.
type TagsChange struct {
	Name   string // file name, unchanged
	Type   string // "mp3", "ogg", ...
	Title  string
	Artist string
	Album  string
	Number string // target TRCK
}

// RenameChange renames a file.
type RenameChange struct {
	OldName string
	NewName string
}

// Plan is the set of changes to apply, in three phases.
type Plan struct {
	// Intermediate is the playlist as it should look after all
	// changes are applied, but with the current file names (the
	// names before any renames). It is written to disk before any
	// changes are made, so that a crash at any later point can be
	// recovered from.
	Intermediate playlist.PlayList

	Tags     []TagsChange
	Renames  []RenameChange
	Finalize []RenameChange
}

// Warnings collects non-fatal problems found while building the
// plan. A non-empty Warnings means the caller must not apply the
// plan and must not rewrite the playlist.
type Warnings []string

// target is an intermediate representation: one file that needs to
// be brought to FinalName, with the tag values it should have.
type target struct {
	OldName   string // real name on disk, may include TempSuffix
	FinalName string // desired name in the final playlist
	Type      string
	Title     string
	Artist    string
	Album     string
	Number    string // target TRCK
	Hash      string // hash of desired tags, without the number
	TagsDirty bool
}

// BuildPlan compares the point of view from the directory (d) with
// the point of view from the playlist (p) and returns the changes
// needed to make them match.
//
// d is the directory snapshot: one Song per audio file, with tags
// as they are on disk. p is the playlist snapshot: one Song per
// entry in the CSV, in playlist order.
//
// If p is empty, the caller should pass a copy of d so that the
// first run builds the playlist from the directory.
func BuildPlan(d, p playlist.PlayList, opts Options) (Plan, Warnings, error) {
	matched, leftD, leftP := resolve(d, p)

	if len(leftP) > 0 {
		var w Warnings
		for _, song := range leftP {
			w = append(w,
				fmt.Sprintf("in playlist but not in directory: %s",
					song.FileName))
		}
		return Plan{}, w, nil
	}

	targets, warnings, err := buildTargets(matched, leftD, opts)
	if err != nil {
		return Plan{}, nil, err
	}

	intermediate := playlistFromTargets(targets)

	plan := Plan{Intermediate: intermediate}

	// Phase 1: tags.
	for _, t := range targets {
		if !t.TagsDirty {
			continue
		}
		plan.Tags = append(plan.Tags, TagsChange{
			Name:   t.OldName,
			Type:   t.Type,
			Title:  t.Title,
			Artist: t.Artist,
			Album:  t.Album,
			Number: t.Number,
		})
	}

	// Phase 2: renames.
	var candidates []RenameChange
	for _, t := range targets {
		if t.OldName == t.FinalName {
			continue
		}
		candidates = append(candidates, RenameChange{
			OldName: t.OldName,
			NewName: t.FinalName,
		})
	}
	renames, finalize := orderRenames(candidates)
	plan.Renames = renames
	plan.Finalize = finalize

	// Validation.
	warnings = append(warnings, validatePlan(plan)...)
	if len(warnings) > 0 {
		return Plan{}, warnings, nil
	}

	return plan, nil, nil
}

// playlistFromTargets builds the intermediate playlist: current
// file names with the target tag values and hashes.
func playlistFromTargets(targets []target) playlist.PlayList {
	out := make(playlist.PlayList, 0, len(targets))
	for _, t := range targets {
		out = append(out, playlist.Song{
			FileName: t.OldName,
			Number:   t.Number,
			Title:    t.Title,
			Artist:   t.Artist,
			Album:    t.Album,
			Type:     t.Type,
			Hash:     t.Hash,
		})
	}
	return out
}

// pair is one matched pair: a song from the directory and the song
// from the playlist that refers to the same file.
type pair struct {
	d playlist.Song // from directory
	p playlist.Song // from playlist
}

// resolve matches songs from d and p. It returns matched pairs in
// playlist order, and the leftovers from each side.
//
// Matching is done in two passes: first by full file name
// (case-insensitive), then by tag hash. If a song from p cannot be
// matched either way, it goes to leftP (this is a fatal error).
// Remaining songs from d go to leftD (new files, legal).
func resolve(d, p playlist.PlayList) (matched []pair, leftD, leftP playlist.PlayList) {
	usedD := make([]bool, len(d))

	for _, ps := range p {
		idx := matchByName(d, usedD, ps.FileName)
		if idx < 0 {
			idx = matchByHash(d, usedD, ps.Hash)
		}
		if idx < 0 {
			leftP = append(leftP, ps)
			continue
		}
		usedD[idx] = true
		matched = append(matched, pair{d: d[idx], p: ps})
	}

	for i, ds := range d {
		if !usedD[i] {
			leftD = append(leftD, ds)
		}
	}
	return matched, leftD, leftP
}

// matchByName looks for an unused song in d whose file name equals
// name (case-insensitive). Returns the index or -1.
func matchByName(d playlist.PlayList, used []bool, name string) int {
	lower := strings.ToLower(name)
	for i, ds := range d {
		if used[i] {
			continue
		}
		if strings.ToLower(ds.FileName) == lower {
			return i
		}
	}
	return -1
}

// matchByHash looks for an unused song in d whose hash equals hash.
// Returns the index or -1.
func matchByHash(d playlist.PlayList, used []bool, hash string) int {
	if hash == "" {
		return -1
	}
	for i, ds := range d {
		if used[i] {
			continue
		}
		if ds.Hash == hash {
			return i
		}
	}
	return -1
}

// buildTargets assigns track numbers to the matched pairs and the
// leftover directory files, in playlist order. It also checks for
// type mismatches between the directory and the playlist, and
// builds file names from tags when requested.
func buildTargets(matched []pair, leftD playlist.PlayList, opts Options) ([]target, Warnings, error) {
	total := len(matched) + len(leftD)
	if total == 0 {
		return nil, nil, nil
	}
	width, err := naming.PrefixWidth(total)
	if err != nil {
		return nil, nil, err
	}

	var warnings Warnings
	targets := make([]target, 0, total)
	index := 0

	// Matched pairs, in playlist order. Tags come from the
	// playlist; the base name comes from the directory.
	for _, m := range matched {
		index++

		if m.d.Type != m.p.Type {
			warnings = append(warnings, fmt.Sprintf(
				"type mismatch for %s: directory has %q, "+
					"playlist has %q",
				m.d.FileName, m.d.Type, m.p.Type))
		}

		number := naming.FormatNumber(index, width)
		title := naming.NormalizeSpaces(m.p.Title)
		artist := naming.NormalizeSpaces(m.p.Artist)
		album := naming.NormalizeSpaces(m.p.Album)

		base := baseName(m.d.FileName, title, artist, opts, &warnings)
		finalName := naming.MakeNewName(number, base)
		hash := naming.HashTags(title, artist, album)

		tagsDirty := hash != m.p.Hash || number != m.p.Number

		targets = append(targets, target{
			OldName:   m.d.FileName,
			FinalName: finalName,
			Type:      m.d.Type,
			Title:     title,
			Artist:    artist,
			Album:     album,
			Number:    number,
			Hash:      hash,
			TagsDirty: tagsDirty,
		})
	}

	// Leftover directory files, appended at the end. Tags come
	// from the directory itself; the base name too.
	for _, ds := range leftD {
		index++
		number := naming.FormatNumber(index, width)

		title := naming.NormalizeSpaces(ds.Title)
		artist := naming.NormalizeSpaces(ds.Artist)
		album := naming.NormalizeSpaces(ds.Album)

		base := baseName(ds.FileName, title, artist, opts, &warnings)
		finalName := naming.MakeNewName(number, base)

		targets = append(targets, target{
			OldName:   ds.FileName,
			FinalName: finalName,
			Type:      ds.Type,
			Title:     title,
			Artist:    artist,
			Album:     album,
			Number:    number,
			Hash:      naming.HashTags(title, artist, album),
			TagsDirty: true, // new file, at least TRCK must be written
		})
	}

	return targets, warnings, nil
}

// baseName returns the base part of the target file name, including
// the extension. If opts.NameFromTags is true, it builds the name
// from Title and Artist; otherwise it strips the prefix from the
// current file name. A missing Title or Artist is reported as a
// warning and falls back to the current file name.
func baseName(
	fileName, title, artist string,
	opts Options,
	warnings *Warnings,
) string {
	// The extension is taken from the permanent name, without the
	// temp suffix that may be present on disk.
	clean := strings.TrimSuffix(fileName, naming.TempSuffix)
	ext := filepath.Ext(clean)

	if !opts.NameFromTags {
		return naming.BasePart(fileName)
	}

	if title == "" || artist == "" {
		*warnings = append(*warnings, fmt.Sprintf(
			"cannot build name from tags for %s: "+
				"Title or Artist is empty",
			fileName))
		return naming.BasePart(fileName)
	}

	return vfat.SanitizeName(artist+" - "+title) + ext
}

// orderRenames takes a list of desired renames and returns them in
// an order that can be applied without overwriting any file.
//
// The algorithm is greedy: on each iteration it tries to find a
// rename whose target name is free; if none is found, it falls
// back to a temporary name. This terminates because the number of
// available names is twice the number of files, so eventually a
// free name is found (pigeonhole principle).
//
// If a rename is done through a temporary name, a matching entry
// is added to finalize, to be applied after all renames.
func orderRenames(candidates []RenameChange) (changes, finalize []RenameChange) {
	occupied := make(map[string]struct{}, 2*len(candidates))
	for _, c := range candidates {
		occupied[c.OldName] = struct{}{}
	}

	for len(candidates) > 0 {
		progress := false

		// Pass 1: direct rename, target name is free.
		for i, c := range candidates {
			if _, busy := occupied[c.NewName]; busy {
				continue
			}
			changes = append(changes, c)
			occupied[c.NewName] = struct{}{}
			delete(occupied, c.OldName)
			candidates = append(candidates[:i], candidates[i+1:]...)
			progress = true
			break
		}
		if progress {
			continue
		}

		// Pass 2: rename to a temporary name.
		for i, c := range candidates {
			temp := c.NewName + naming.TempSuffix
			if _, busy := occupied[temp]; busy {
				continue
			}
			changes = append(changes, RenameChange{
				OldName: c.OldName,
				NewName: temp,
			})
			finalize = append(finalize, RenameChange{
				OldName: temp,
				NewName: c.NewName,
			})
			occupied[temp] = struct{}{}
			delete(occupied, c.OldName)
			candidates = append(candidates[:i], candidates[i+1:]...)
			progress = true
			break
		}
		if progress {
			continue
		}

		// The pigeonhole principle guarantees progress. If we
		// get here, it is a bug in the algorithm, not bad data.
		panic("plan: orderRenames made no progress")
	}
	return changes, finalize
}

// validatePlan checks all target names for VFAT compatibility and
// collisions.
func validatePlan(p Plan) Warnings {
	var warnings Warnings

	seen := make(map[string]string) // lowercase target -> source

	check := func(oldName, newName string) {
		if err := vfat.Check(newName); err != nil {
			warnings = append(warnings,
				fmt.Sprintf("%s: %v", newName, err))
		}
		key := strings.ToLower(newName)
		if prev, ok := seen[key]; ok {
			warnings = append(warnings,
				fmt.Sprintf("multiple files target %s: %s, %s",
					newName, prev, oldName))
		} else {
			seen[key] = oldName
		}
	}

	for _, c := range p.Renames {
		check(c.OldName, c.NewName)
	}
	for _, c := range p.Finalize {
		check(c.OldName, c.NewName)
	}

	return warnings
}
