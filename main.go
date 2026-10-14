package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"

	"github.com/tigra564/usb-tracklist/internal/sync"
)

const defaultPlaylist = "playlist.csv"

func main() {
	var (
		playlistName string
		verbose      bool
		dryRun       bool
	)

	pflag.StringVarP(&playlistName, "play-list", "p", defaultPlaylist,
		"playlist file name")
	pflag.BoolVarP(&verbose, "verbose", "v", false,
		"verbose output")
	pflag.BoolVarP(&dryRun, "dry-run", "n", false,
		"show plan without changes (implies -v)")

	pflag.Usage = func() {
		printUsage(os.Stdout)
	}

	pflag.Parse()

	if pflag.NArg() != 1 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	directory := pflag.Arg(0)

	if dryRun {
		verbose = true
	}

	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: not a directory: %s\n", directory)
		os.Exit(1)
	}

	// Verify that the directory is writable. The only reliable
	// way to check is to actually create a file.
	if !isWritable(directory) {
		fmt.Fprintf(os.Stderr, "error: directory is not writable: %s\n",
			directory)
		os.Exit(1)
	}

	playlistPath := playlistName
	if !filepath.IsAbs(playlistPath) {
		playlistPath = filepath.Join(".", playlistPath)
	}

	opts := sync.Options{
		Directory:    directory,
		PlaylistPath: playlistPath,
		DryRun:       dryRun,
		Verbose:      verbose,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
	}

	if err := sync.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// isWritable reports whether a file can be created in dir. The only
// reliable check is to actually try.
func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".usb-tracklist-check-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// printUsage writes the help text to w. Called with os.Stdout when
// the user asked for help explicitly, and with os.Stderr when the
// command line could not be parsed.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, `usb-tracklist — synchronize a folder of MP3 files with a playlist

Usage:
  usb-tracklist <directory> [flags]

Flags:
  -p, --play-list <file>   playlist file name (default: %s)
  -v, --verbose            verbose output
  -n, --dry-run            show plan without changes (implies -v)
  -h, --help               show this help

The playlist is always the source of truth. If it does not exist or is
empty, it is created from the directory contents.

`, defaultPlaylist)
}
