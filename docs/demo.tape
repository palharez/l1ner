Output demo.gif

Set Shell bash
Set Theme "Catppuccin Mocha"
Set FontFamily "DejaVu Sans Mono"
Set FontSize 19
Set Width 1010
Set Height 515
Set Padding 26
Set WindowBar Colorful
Set Framerate 15
Set TypingSpeed 29ms
Set CursorBlink false

Hide
Type "./l1ner inspect music/01.mp3"
Enter
Wait+Screen /Title \[TIT2\]: First Light/
Show
Sleep 2.6s

Ctrl+L
Type "./l1ner organize --dry-run music/"
Enter
Wait+Screen /Summary: 2 planned/
Sleep 3s

Ctrl+L
Type "./l1ner organize --apply music/"
Enter
Wait+Screen /❯ Skip song/
Sleep 1.2s
Down
Wait+Screen /❯ Apply move/
Sleep 1.1s
Enter
Wait+Screen /Song 2 of 2/
Wait+Screen /❯ Skip song/
Sleep 1.5s
Enter
Wait+Screen /Summary: 1 moved, 1 skipped/
Sleep 2s

Ctrl+L
Type "find music -name '*.mp3' | sort"
Enter
Wait+Screen /Night Signals.01[.]mp3/
Sleep 3s
