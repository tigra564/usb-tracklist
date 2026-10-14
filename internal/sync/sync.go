// Package sync orchestrates the whole operation: read the directory
// and the playlist, build a plan, apply it in three phases, and
// rewrite the playlist to reflect the result.
package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tigra564/usb-tracklist/internal/fsops"
	"github.com/tigra564/usb-tracklist/internal/naming"
	"github.com/tigra564/usb-tracklist/internal/plan"
	"github.com/tigra564/usb-tracklist/internal/playlist"
	"github.com/tigra564/usb-tracklist/internal/tags"
)

// Options controls a single run.
type Options struct {
	Directory    string
	PlaylistPath string
	DryRun       bool
	Verbose      bool
	Stdout       io.Writer
	Stderr       io.Writer
}

// Run performs one synchronization pass.
func Run(opts Options) error {
	reader := tags.ID3Reader{}

	// 1. Directory snapshot.
	d, err := buildPlaylist(opts.Directory, reader)
	if err != nil {
		return err
	}

	// 2. Playlist snapshot.
	p, err := loadPlaylist(opts.PlaylistPath)
	if err != nil {
		return err
	}
	if len(p) == 0 {
		// No playlist yet: treat it as a copy of d, so that the
		// plan starts from an already consistent state.
		p = d
	}

	// 3. Plan.
	pl, warnings, err := plan.BuildPlan(d, p)
	if err != nil {
		return err
	}
	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Fprintln(opts.Stderr, w)
		}
		return fmt.Errorf("plan has warnings, nothing was changed")
	}

	// 4. Dry run: print and stop.
	if opts.DryRun {
		printPlan(opts.Stdout, pl)
		return nil
	}

	// 5. Apply in three phases.
	if err := applyPlan(opts, pl); err != nil {
		return err
	}

	// 6. Rewrite the playlist from the directory.
	if err := writePlaylist(opts, reader); err != nil {
		return err
	}

	return nil
}

// buildPlaylist reads the directory and returns a playlist snapshot.
func buildPlaylist(dir string, reader tags.Reader) (playlist.PlayList, error) {
	files, err := fsops.ListFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("list directory: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no audio files in %s", dir)
	}
	var list playlist.PlayList
	if err := list.FromFileList(dir, files, reader); err != nil {
		return nil, err
	}
	return list, nil
}

// loadPlaylist reads the playlist from disk. A missing or empty
// file yields an empty playlist, no error.
func loadPlaylist(path string) (playlist.PlayList, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open playlist: %w", err)
	}
	defer f.Close()
	list, err := playlist.Read(f)
	if err != nil {
		return nil, fmt.Errorf("read playlist: %w", err)
	}
	return list, nil
}

// applyPlan applies the plan in three phases: tags, renames,
// finalize.
func applyPlan(opts Options, pl plan.Plan) error {
	// Phase 1: tags.
	for _, c := range pl.Tags {
		if opts.Verbose {
			fmt.Fprintf(opts.Stdout, "tags: %s\n", c.Name)
		}
		if err := writeTags(opts.Directory, c); err != nil {
			return err
		}
	}

	// Phase 2: renames.
	if len(pl.Renames) > 0 {
		pairs := make([]fsops.RenamePair, 0, len(pl.Renames))
		for _, c := range pl.Renames {
			pairs = append(pairs, fsops.RenamePair{
				Old: c.OldName,
				New: c.NewName,
			})
			if opts.Verbose {
				fmt.Fprintf(opts.Stdout, "rename: %s → %s\n",
					c.OldName, c.NewName)
			}
		}
		if err := fsops.RenameAll(opts.Directory, pairs); err != nil {
			return err
		}
	}

	// Phase 3: finalize.
	for _, c := range pl.Finalize {
		if opts.Verbose {
			fmt.Fprintf(opts.Stdout, "finalize: %s → %s\n",
				c.OldName, c.NewName)
		}
		oldPath := filepath.Join(opts.Directory, c.OldName)
		newPath := filepath.Join(opts.Directory, c.NewName)
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("finalize %s: %w", c.OldName, err)
		}
	}

	return nil
}

// writeTags writes tags to a file safely: it copies the file to a
// temporary name, writes tags to the copy, and renames the copy
// back. This way the original file is never left half-written.
func writeTags(dir string, c plan.TagsChange) error {
	src := filepath.Join(dir, c.Name)
	tmp := filepath.Join(dir, c.Name+naming.TagsTempSuffix)

	// Copy src to tmp.
	if err := copyFile(src, tmp); err != nil {
		return fmt.Errorf("copy %s: %w", c.Name, err)
	}

	// Write tags to tmp.
	t := tags.Tags{
		Title:  c.Title,
		Artist: c.Artist,
		Album:  c.Album,
		Track:  c.Number,
	}
	if err := tags.Write(tmp, c.Type, t); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("write tags to %s: %w", c.Name, err)
	}

	// Rename tmp back to src.
	if err := os.Rename(tmp, src); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename %s back: %w", c.Name, err)
	}

	return nil
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// writePlaylist rebuilds the directory snapshot and writes it to
// the playlist file.
func writePlaylist(opts Options, reader tags.Reader) error {
	list, err := buildPlaylist(opts.Directory, reader)
	if err != nil {
		return err
	}
	f, err := os.Create(opts.PlaylistPath)
	if err != nil {
		return fmt.Errorf("create playlist: %w", err)
	}
	defer f.Close()
	if err := playlist.Write(f, list); err != nil {
		return fmt.Errorf("write playlist: %w", err)
	}
	if opts.Verbose {
		fmt.Fprintf(opts.Stdout, "playlist written: %s\n",
			opts.PlaylistPath)
	}
	return nil
}

// printPlan prints a human-readable plan to w.
func printPlan(w io.Writer, pl plan.Plan) {
	if len(pl.Tags) > 0 {
		fmt.Fprintln(w, "Tags:")
		for _, c := range pl.Tags {
			fmt.Fprintf(w, "  %s\n", c.Name)
		}
	}
	if len(pl.Renames) > 0 {
		fmt.Fprintln(w, "Renames:")
		for _, c := range pl.Renames {
			fmt.Fprintf(w, "  %s → %s\n", c.OldName, c.NewName)
		}
	}
	if len(pl.Finalize) > 0 {
		fmt.Fprintln(w, "Finalize:")
		for _, c := range pl.Finalize {
			fmt.Fprintf(w, "  %s → %s\n", c.OldName, c.NewName)
		}
	}
}
