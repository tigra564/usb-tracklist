// Package tags reads and writes the tag fields that this tool
// manages. Reading is done with dhowden/tag, which is tolerant of
// unusual frames (e.g. PRIV frames written by Windows Media Player)
// that can break stricter parsers. Writing is done with
// bogem/id3v2/v2, which can preserve unknown frames while updating
// only the ones we care about.
package tags

import (
	"fmt"
	"os"

	"github.com/bogem/id3v2/v2"
	"github.com/dhowden/tag"
)

// Tags holds the tag fields that this tool manages.
type Tags struct {
	Title  string
	Artist string
	Album  string
	Track  string
}

// Reader reads tags from a single file of the given type.
type Reader interface {
	Read(path string, typ string) (Tags, error)
}

// ID3Reader implements Reader using dhowden/tag for reading and
// bogem/id3v2 for writing.
type ID3Reader struct{}

// Read reads tags from a file. Returns an error for unsupported
// types.
func (r ID3Reader) Read(path string, typ string) (Tags, error) {
	switch typ {
	case "mp3":
		return readMP3(path)
	default:
		return Tags{}, fmt.Errorf("unsupported file type: %s", typ)
	}
}

// Write writes tags to a file. Unsupported types are ignored
// silently, because the caller may not have known that this file
// type is not supported when the plan was built.
func Write(path string, typ string, t Tags) error {
	switch typ {
	case "mp3":
		return writeMP3(path, t)
	default:
		return nil
	}
}

// readMP3 reads TIT2, TPE1, TALB and TRCK from an MP3 file using
// dhowden/tag.
func readMP3(path string) (Tags, error) {
	f, err := os.Open(path)
	if err != nil {
		return Tags{}, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return Tags{}, err
	}

	// Track() returns (number, total). We render it the way the
	// ID3 spec describes: "number" or "number/total".
	track, total := m.Track()
	trackStr := ""
	if total > 0 {
		trackStr = fmt.Sprintf("%d/%d", track, total)
	} else if track > 0 {
		trackStr = fmt.Sprintf("%d", track)
	}

	return Tags{
		Title:  m.Title(),
		Artist: m.Artist(),
		Album:  m.Album(),
		Track:  trackStr,
	}, nil
}

// writeMP3 writes TIT2, TPE1, TALB and TRCK to an MP3 file using
// bogem/id3v2, leaving all other frames untouched.
func writeMP3(path string, t Tags) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	tag.SetTitle(t.Title)
	tag.SetArtist(t.Artist)
	tag.SetAlbum(t.Album)

	// AddTextFrame appends; delete any existing TRCK first so we
	// do not end up with two frames with the same ID.
	tag.DeleteFrames(tag.CommonID("Track number/Position in set"))
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"),
		id3v2.EncodingUTF8, t.Track)

	return tag.Save()
}
