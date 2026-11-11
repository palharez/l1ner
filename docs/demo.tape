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
Set TypingSpeed 35ms
Set CursorBlink false

Hide
Type `export XDG_CACHE_HOME="$PWD/.cache"`
Enter
Ctrl+L
Type "./l1ner inspect music/01.mp3"
Enter
Wait+Screen /Title \[TIT2\]: first light/
Show
Sleep 3s

Ctrl+L
Type "./l1ner fix --release music/01.mp3"
Enter
Wait+Screen /Choose a recording/
Sleep 1.8s
Enter
Wait+Screen /Choose a release/
Sleep 2.1s
Enter
Wait+Screen /Album.*Loose tracks.*Night Signals/
Wait+Screen /❯ Skip song/
Sleep 4.5s
Down
Wait+Screen /❯ Apply changes to this MP3/
Sleep 900ms
Enter
Wait+Screen /Backup: music.01[.]mp3[.]l1ner[.]bak/
Sleep 2.2s

Ctrl+L
Type "./l1ner inspect music/01.mp3"
Enter
Wait+Screen /Title \[TIT2\]: First Light/
Sleep 2.7s

Ctrl+L
Type "./l1ner organize --dry-run music/"
Enter
Wait+Screen /Summary: 2 planned/
Sleep 2.7s

Ctrl+L
Type "./l1ner organize --apply music/"
Enter
Wait+Screen /❯ Skip song/
Sleep 1.1s
Down
Wait+Screen /❯ Apply move/
Sleep 800ms
Enter
Wait+Screen /Song 2 of 2/
Wait+Screen /❯ Skip song/
Sleep 1.1s
Enter
Wait+Screen /Summary: 1 moved, 1 skipped/
Sleep 2s

Ctrl+L
Type "find music -name '*.mp3' | sort"
Enter
Wait+Screen /Night Signals.01[.]mp3/
Sleep 2.5s
