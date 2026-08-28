# FF-16-TUI (Find Frequent 16-bit Text User Interface)

## What does FF-16-TUI do?

FF-16-TUI is an interactive static analysis tool that finds frequently occurring local 16-bit patterns across the entire file. It can help to locate structures from frequent patterns and understand file layout.

## Text User Interface

<img width="937" height="746" alt="ff-16-tui" src="https://github.com/user-attachments/assets/8da1c4a1-9b66-46ac-88cb-b69bbe3f9df3" />

## Command line usage

```
ff-16-tui [filename] [-d <filename>]

  <filename>      Target file
  -d <filename>   Dictionary file  (Default: dict.csv)
```

## Keyboard shortcuts

Hexdump panel:

| Key | Function |
| --- | --- |
| ← | Previous byte |
| → | Next byte |
| ↑ | Previous line |
| ↓ | Next line |
| PgUp | Previous page |
| PgDn | Next page |
| Home | Beginning of file |
| End | End of file |

Pattern panel:

| Key | Function |
| --- | --- |
| 0 | Highlight pattern (most frequent) |
| 1 | Highlight pattern |
| 2 | Highlight pattern |
| 3 | Highlight pattern |
| 4 | Highlight pattern |
| 5 | Highlight pattern |
| 6 | Highlight pattern |
| 7 | Highlight pattern |
| 8 | Highlight pattern |
| 9 | Highlight pattern (least frequent) |

Filter panel:

| Key | Function |
| --- | --- |
| q | Increase the minimum gap |
| s | Decrease the minimum gap |
| w | Increase the maximum gap |
| s | Decrease the maximum gap |
| e | Increase the minimum set bits |
| d | Decrease the minimum set bits |
| r | Increase the maximum set bits |
| f | Decrease the maximum set bits |
| t | Increase the threshold |
| g | Decrease the threashold |

## Terminologies

| Term | Description |
| --- | --- |
| Ones | The number of bits set in the pattern. |
| Freq | The frequency of the pattern. |
| Dict | The dictionary entry for the pattern. |
