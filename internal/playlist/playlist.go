// Package playlist defines the point of view: a list of songs with
// the tag fields that describe them. It reads and writes the CSV
// representation of a playlist and can build a playlist from a
// directory listing.
package playlist

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tigra564/usb-tracklist/internal/fsops"
	"github.com/tigra564/usb-tracklist/internal/naming"
	"github.com/tigra564/usb-tracklist/internal/tags"
)

// csvFields is the fixed column order of the playlist file.
// Human-readable fields come first; service fields (Type, Hash)
// come last.
var csvFields = []string{
	"File Name",
	"Track Number",
	"Title",
	"Artist",
	"Album",
	"Type",
	"Hash",
}

// bom is the UTF-8 byte order mark. LibreOffice Calc needs it to
// detect UTF-8 correctly.
var bom = []byte{0xEF, 0xBB, 0xBF}

// Song is one entry in the point of view.
type Song struct {
	FileName string
	Number   string
	Title    string
	Artist   string
	Album    string
	Type     string // "mp3", "ogg", ...
	Hash     string // hash of Title|Artist|Album
}

// PlayList is the full point of view.
type PlayList []Song

// Read reads a playlist from r. If r is empty, Read returns an
// empty list. The column order must match csvFields exactly.
func Read(r io.Reader) (PlayList, error) {
	br := bufio.NewReader(r)
	if head, err := br.Peek(3); err == nil && bytes.Equal(head, bom) {
		if _, err := br.Discard(3); err != nil {
			return nil, err
		}
	}

	cr := csv.NewReader(br)
	cr.Comma = ';'
	cr.FieldsPerRecord = len(csvFields)

	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	header := rows[0]
	for i, want := range csvFields {
		if header[i] != want {
			return nil, fmt.Errorf(
				"unexpected column %d: got %q, want %q",
				i+1, header[i], want)
		}
	}

	list := make(PlayList, 0, len(rows)-1)
	for _, row := range rows[1:] {
		list = append(list, Song{
			FileName: row[0],
			Number:   row[1],
			Title:    row[2],
			Artist:   row[3],
			Album:    row[4],
			Type:     row[5],
			Hash:     row[6],
		})
	}
	return list, nil
}

// Write writes a playlist to w with a leading BOM, semicolon
// separators and CRLF line endings.
func Write(w io.Writer, list PlayList) error {
	if _, err := w.Write(bom); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cw.UseCRLF = true

	if err := cw.Write(csvFields); err != nil {
		return err
	}
	for _, s := range list {
		if err := cw.Write([]string{
			s.FileName,
			s.Number,
			s.Title,
			s.Artist,
			s.Album,
			s.Type,
			s.Hash,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// FromFileList builds a playlist from a directory listing. The
// files are sorted naturally by their permanent name. Tags are
// read from each file; the file type is passed through to the
// reader so that it can route to the right library.
//
// If a file's tags cannot be read, FromFileList returns an error
// and the caller must not proceed.
func (list *PlayList) FromFileList(
	dir string,
	files []fsops.AudioFile,
	reader tags.Reader,
) error {
	sorted := make([]fsops.AudioFile, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return naming.NaturalCompare(
			sorted[i].PermanentName,
			sorted[j].PermanentName) < 0
	})

	width, err := naming.PrefixWidth(len(sorted))
	if err != nil {
		return err
	}

	out := make(PlayList, 0, len(sorted))
	for _, f := range sorted {
		path := filepath.Join(dir, f.Name)
		t, err := reader.Read(path, f.Type)
		if err != nil {
			return fmt.Errorf("read tags from %s: %w", f.Name, err)
		}

		song := Song{
			FileName: f.PermanentName,
			Number:   normalizeTrack(t.Track, width),
			Title:    t.Title,
			Artist:   t.Artist,
			Album:    t.Album,
			Type:     f.Type,
		}
		song.Hash = naming.HashTags(song.Title, song.Artist, song.Album)
		out = append(out, song)
	}

	*list = out
	return nil
}

// normalizeTrack brings a TRCK value to the canonical form used by
// this tool: a zero-padded number of the given width, or an empty
// string if the value is not a number. "5/12", "5", and "05" all
// become "05" with width 2.
func normalizeTrack(track string, width int) string {
	// Strip the "/total" part if present.
	if i := strings.IndexByte(track, '/'); i >= 0 {
		track = track[:i]
	}
	n, err := strconv.Atoi(track)
	if err != nil {
		return ""
	}
	return naming.FormatNumber(n, width)
}
