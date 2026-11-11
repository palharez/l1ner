#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
demo_dir=$(mktemp -d)
trap 'rm -r -- "$demo_dir"' EXIT

cd "$repo_root"
go build -o "$demo_dir/l1ner" docs/demo_main.go
mkdir "$demo_dir/music"

for track in 1 2; do
  title="first light"
  album="Loose tracks"
  if [[ "$track" == 2 ]]; then
    title="Blue Hour"
    album="Night Signals"
  fi
  ffmpeg -nostdin -loglevel error -f lavfi -i anullsrc=r=44100:cl=mono -t 1 \
    -c:a libmp3lame -fflags +bitexact -id3v2_version 3 -metadata title="$title" \
    -metadata artist="Paper Satellites" -metadata album_artist="Paper Satellites" \
    -metadata album="$album" -metadata track="$track" -metadata disc=1 \
    "$demo_dir/music/0$track.mp3"
done
cp "$demo_dir/music/01.mp3" "$demo_dir/original.mp3"

(
  cd "$demo_dir"
  vhs "$repo_root/docs/demo.tape"
)

fixed="$demo_dir/music/Paper Satellites/Night Signals/01.mp3"
tags=$("$demo_dir/l1ner" inspect "$fixed")
grep -Fxq 'Title [TIT2]: First Light' <<< "$tags"
grep -Fxq 'Album [TALB]: Night Signals' <<< "$tags"
cmp "$demo_dir/original.mp3" "$demo_dir/music/01.mp3.l1ner.bak"
test ! -e "$demo_dir/music/01.mp3"
test -f "$demo_dir/music/02.mp3"
install -m 0644 "$demo_dir/demo.gif" "$repo_root/docs/demo.gif"
