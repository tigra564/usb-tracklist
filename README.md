# usb-tracklist

Synchronize a folder of MP3 files with a CSV playlist, so that a
portable player or an old car stereo plays them in the order you
want.

## Why

Many players ignore the file name and sort by the ID3 track number
(`TRCK`), or by some combination of name and tag that is hard to
predict. If you copy a folder of MP3s to a USB stick and the player
plays them in the wrong order, this tool is for you.

It also fixes the common tag fields (`Title`, `Artist`, `Album`) at
the same time, so that the information shown on the player's display
is correct.

## Install

Download a prebuilt binary from the [Releases][releases] page.
Binaries are provided for Linux, macOS, and Windows on both x86-64
and ARM64. No runtime dependencies are needed.

[releases]: https://github.com/tigra564/usb-tracklist/releases

## Quick start

```
# 1. Look at what would change, without touching anything.
usb-tracklist /path/to/music -n

# 2. Apply the changes and create playlist.csv.
usb-tracklist /path/to/music -v

# 3. Edit playlist.csv if you want to reorder tracks or fix tags,
#    then run again.
usb-tracklist /path/to/music
```

## How it works

The tool works with two points of view:

- **The directory** — the MP3 files on disk, with their tags as
  they are right now.
- **The playlist** — the CSV file, which is the source of truth for
  the order of tracks and for the tag values.

On each run the tool compares the two, builds a plan, and applies
it in three phases:

1. **Tags** — rewrite `Title`, `Artist`, `Album`, and `TRCK` in
   the files whose tags differ from the playlist.
2. **Renames** — rename files so that their names match the order
   in the playlist.
3. **Finalize** — finish any renames that needed a temporary name
   to avoid collisions.

After a successful run, the directory and the playlist are in sync.
Running the tool again does virtually nothing: it only refreshes the
playlist.

### The playlist

The playlist is a CSV file with a semicolon separator, a UTF-8 BOM,
and the following columns:

```
File Name;Track Number;Title;Artist;Album;Type;Hash
```

- **File Name** — the name of the file on disk. Do not edit this
  column by hand; the tool relies on it to match files.
- **Track Number** — the track number as written to the file.
  Filled in by the tool, shown here for your reference.
- **Title**, **Artist**, **Album** — the tag values. Edit these
  freely.
- **Type** — `mp3` for now. More formats may be added later.
- **Hash** — a checksum of the tag values, used to detect changes.
  Do not edit.

To change the order of tracks, reorder the rows. To change tags,
edit the `Title`, `Artist`, or `Album` columns. Do not edit
`File Name`, `Track Number`, `Type`, or `Hash`.

### What is allowed

Between runs you may:

- Add new files to the directory. They will be appended to the end
  of the playlist, and you can then reorder them.
- Rename a file on disk, as long as its tag values still match an
  entry in the playlist. The tool will recognize it by its tags and
  keep the new name.

To **remove** a track, delete the file from the directory *and*
remove its row from the playlist. If you only remove the row, the
next run will see the file as a new addition and add it back.

You may **not**:

- Remove a file from the directory while its entry is still in the
  playlist. The tool will stop with an error. Remove the row from
  the playlist as well, and run again.
- Rename a file on disk if you have also changed its tags. The tool
  cannot tell whether the file is new or just moved.
- Edit the `File Name` or `Hash` column by hand.

If the tool stops with an error before applying the plan, nothing
has been changed. Errors during application may leave the directory
in a partially updated state: some tags may already be rewritten,
and some file names may have a `.usb-tracklist-tmp` suffix. This is
normal and expected. Fix the cause of the error (a power failure, a
full disk, a file locked by another program) and run the tool
again. It will recognize the temporary names and finish the job.

### Notes

The tool never deletes or corrupts your files. Even when tags are
rewritten, the audio data is left untouched. If something goes
wrong, you can always delete `playlist.csv` and start over: the
next run will rebuild it from the directory.

Run the tool from (or point it at) the directory that contains the
tracks. If you keep two directories with identically named files,
such as `01.mp3`, `02.mp3`, and so on, using the same playlist for
both will confuse the tool: it matches files by name, and will
happily rewrite the tags of the wrong file. This is easy to avoid:
one playlist per directory.

## Options

```
-p, --play-list <file>   playlist file name (default: playlist.csv)
-v, --verbose            verbose output
-n, --dry-run            show plan without changes (implies -v)
-h, --help               show help
```

## Building from source

Requires Go 1.22 or later.

```
git clone https://github.com/tigra564/usb-tracklist
cd usb-tracklist
go build .
```

## License

MIT. See [LICENSE](LICENSE).
