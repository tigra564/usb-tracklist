// Package fsops provides filesystem operations for the tool:
// listing audio files, recognizing files left over from an
// interrupted rename, and renaming files in one or two phases.
package fsops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tigra564/usb-tracklist/internal/naming"
)

// AudioFile describes one audio file on disk.
//
// Name is the real file name, which may end with naming.TempSuffix
// if the previous run was interrupted. PermanentName is the name
// without the temp suffix, still with its prefix and extension.
// Type is the lowercase extension without the dot: "mp3", "ogg",
// and so on.
type AudioFile struct {
	Name          string
	PermanentName string
	Type          string
}

// ListFiles walks dir and returns the audio files it finds. A file
// is considered audio if, after stripping the temp suffix, its
// extension is one of the recognized audio extensions. The
// returned Name is the real file name on disk, with the temp
// suffix if present.
//
// This is the only place where the extension is treated as a
// separate entity. Everywhere else it is just part of the name.
func ListFiles(dir string) ([]AudioFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []AudioFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		clean := strings.TrimSuffix(name, naming.TempSuffix)
		if !naming.HasAudioExtension(clean) {
			continue
		}
		ext := filepath.Ext(clean)
		typ := strings.ToLower(strings.TrimPrefix(ext, "."))
		files = append(files, AudioFile{
			Name:          name,
			PermanentName: clean,
			Type:          typ,
		})
	}
	return files, nil
}

// RenamePair is a single rename operation. Names only, not full
// paths: the directory is the same for every pair.
type RenamePair struct {
	Old string
	New string
}

// RenameAll renames files in dir. If no target name collides with
// the current name of another file being renamed, the renames are
// done in a single pass. Otherwise a two-phase rename is used:
// every file first moves to New + TempSuffix, then to New.
func RenameAll(dir string, pairs []RenamePair) error {
	if len(pairs) == 0 {
		return nil
	}

	oldSet := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		oldSet[p.Old] = true
	}

	needsTwoPhase := false
	for _, p := range pairs {
		if p.Old == p.New {
			continue
		}
		if oldSet[p.New] {
			needsTwoPhase = true
			break
		}
	}

	if !needsTwoPhase {
		for _, p := range pairs {
			if p.Old == p.New {
				continue
			}
			if err := rename(dir, p.Old, p.New); err != nil {
				return err
			}
		}
		return nil
	}

	// Phase 1: every file moves to New + TempSuffix.
	temps := make([]RenamePair, 0, len(pairs))
	for _, p := range pairs {
		if p.Old == p.New {
			continue
		}
		temp := p.New + naming.TempSuffix
		if err := rename(dir, p.Old, temp); err != nil {
			return err
		}
		temps = append(temps, RenamePair{Old: temp, New: p.New})
	}

	// Phase 2: every temporary name moves to its final target.
	for _, p := range temps {
		if err := rename(dir, p.Old, p.New); err != nil {
			return err
		}
	}

	return nil
}

// rename joins dir with both names and calls os.Rename.
func rename(dir, old, new string) error {
	if err := os.Rename(
		filepath.Join(dir, old),
		filepath.Join(dir, new),
	); err != nil {
		return fmt.Errorf("rename %s to %s: %w", old, new, err)
	}
	return nil
}
