Output demo.gif

Set Shell bash
Set Theme "Catppuccin Mocha"
Set FontFamily "DejaVu Sans Mono"
Set FontSize 19
Set Width 1010
Set Height 655
Set Padding 26
Set WindowBar Colorful
Set Framerate 10
Set TypingSpeed 43ms
Set CursorBlink false

Hide
Type `export XDG_CACHE_HOME="$PWD/.cache"`
Enter
Ctrl+L
Type "./l1ner inspect music/01.mp3"
Enter
Wait+Screen /Title \[TIT2\]: first light/
Show
Sleep 4.5s

Ctrl+L
Type "./l1ner fix --release music/01.mp3"
Enter
Wait+Screen /Choose a recording/
Sleep 3s
Enter
Wait+Screen /Choose a release/
Sleep 3.5s
Enter
Wait+Screen /Album.*Loose tracks.*Night Signals/
Wait+Screen /❯ Skip song/
Sleep 6s
Down
Wait+Screen /❯ Apply changes to this MP3/
Sleep 1.5s
Enter
Wait+Screen /Backup: music.01[.]mp3[.]l1ner[.]bak/
Sleep 3.5s

Ctrl+L
Type "./l1ner inspect music/01.mp3"
Enter
Wait+Screen /Title \[TIT2\]: First Light/
Sleep 4s

Ctrl+L
Type "./l1ner organize --dry-run music/"
Enter
Wait+Screen /Summary: 2 planned/
Sleep 4s

Ctrl+L
Type "./l1ner organize --apply music/"
Enter
Wait+Screen /❯ Skip song/
Sleep 2s
Down
Wait+Screen /❯ Apply move/
Sleep 1.5s
Enter
Wait+Screen /Song 2 of 2/
Wait+Screen /❯ Skip song/
Sleep 2s
Enter
Wait+Screen /Summary: 1 moved, 1 skipped/
Sleep 3s

Ctrl+L
Type "find music -name '*.mp3' | sort"
Enter
Wait+Screen /Night Signals.01[.]mp3/
Sleep 4s
