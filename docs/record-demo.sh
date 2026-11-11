#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
demo_dir=$(mktemp -d)
trap 'rm -r -- "$demo_dir"' EXIT

cd "$repo_root"
go build -o "$demo_dir/l1ner" .
mkdir "$demo_dir/music"

for track in 01 02; do
  title="First Light"
  if [[ "$track" == 02 ]]; then
    title="Blue Hour"
  fi
  ffmpeg -nostdin -loglevel error -f lavfi -i anullsrc=r=44100:cl=mono -t 1 \
    -c:a libmp3lame -fflags +bitexact -id3v2_version 3 -metadata title="$title" \
    -metadata artist="Paper Satellites" -metadata album_artist="Paper Satellites" \
    -metadata album="Night Signals" -metadata track="$track" "$demo_dir/music/$track.mp3"
done

(
  cd "$demo_dir"
  vhs "$repo_root/docs/demo.tape"
)

test -f "$demo_dir/music/Paper Satellites/Night Signals/01.mp3"
test ! -e "$demo_dir/music/01.mp3"
test -f "$demo_dir/music/02.mp3"
install -m 0644 "$demo_dir/demo.gif" "$repo_root/docs/demo.gif"
