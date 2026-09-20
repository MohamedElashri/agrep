# Terminal highlighting

`agrep --color` highlights byte spans in the original, unnormalized line. For
Arabic or Hebrew spans it emits this sequence:

```text
RLI  ANSI-red-start  original matched text  ANSI-reset  PDI
```

RLI is U+2067 RIGHT-TO-LEFT ISOLATE and PDI is U+2069 POP DIRECTIONAL
ISOLATE. The isolate contains both ANSI controls and matched text so a terminal's
bidirectional layout does not move the color controls into surrounding text.
Latin-only spans use the ANSI controls without an RTL isolate.

`--no-bidi-isolate` omits RLI/PDI for terminals or fonts that render directional
controls as visible boxes. It does not disable color. `NO_COLOR` or `TERM=dumb`
disables `--color=auto`; `--color=always` remains an explicit override.

## Verification matrix

The automated tests verify exact output bytes for isolated Arabic highlights,
the opt-out form, overlapping spans, original-text offsets, tashkil at both
match boundaries, and presentation-form expansion. They also ensure JSON never
contains ANSI controls.

| Terminal | Binary available in development environment | Automated byte-level check | Manual graphical check |
| --- | --- | --- | --- |
| xterm | Yes | Pass | Requires a graphical session |
| kitty | No | Pass at protocol level | Not run |
| Alacritty | Yes | Pass | Requires a graphical session |
| tmux | Yes | Pass at protocol level | Requires an attached client |

The development container has `TERM=dumb` and no graphical display, so claiming
a visual result for xterm, Alacritty, or tmux would be misleading. To complete a
manual check in each terminal, run:

```sh
printf '%s\n' 'هذه مَدِينَة جميلة' | agrep --color=always 'مدينه'
printf '%s\n' 'قَالَ ٱللَّهُۖ غَفُورٌ' | agrep --color=always 'الله'
printf '%s\n' 'هذه مَدِينَة جميلة' | agrep --color=always --no-bidi-isolate 'مدينه'
```

The first two commands should color the complete original word, including
tashkil and the Quranic pause mark, without reordering adjacent Arabic. The
third should preserve the color but remove the invisible RLI/PDI pair.
