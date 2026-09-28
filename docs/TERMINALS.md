# Terminal color

`--color=auto` highlights original-text matches when output is a terminal.
`NO_COLOR` and `TERM=dumb` disable automatic color; `--color=always` overrides
them. JSON never includes ANSI color codes.

For Arabic or Hebrew, agrep wraps each colored span in an RTL isolate:

```text
RLI  ANSI-color-start  original text  ANSI-reset  PDI
```

RLI is U+2067 and PDI is U+2069. Use `--no-bidi-isolate` if your terminal or
font shows the controls as boxes. It keeps the color. Latin-only matches do not
need the isolates.

To check a terminal visually:

```sh
printf '%s\n' 'هذه مَدِينَة جميلة' | agrep --color=always 'مدينه'
printf '%s\n' 'هذه مَدِينَة جميلة' | agrep --color=always --no-bidi-isolate 'مدينه'
```

The first command should color the complete original word, including tashkil,
without moving adjacent text. The second should keep color and omit the
isolates. Automated tests cover output bytes; graphical checks should be run
in the terminals you support.
