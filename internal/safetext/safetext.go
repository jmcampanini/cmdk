package safetext

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func EscapeTerminalControls(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, s[i])
			i++
			continue
		}

		switch r {
		case '\n':
			b.WriteByte('\n')
		case '\a':
			b.WriteString(`\a`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\v':
			b.WriteString(`\v`)
		case 0x1b:
			b.WriteString(`\x1b`)
		default:
			if unicode.IsControl(r) {
				writeEscapedControl(&b, r)
			} else {
				b.WriteRune(r)
			}
		}
		i += size
	}

	return b.String()
}

func writeEscapedControl(b *strings.Builder, r rune) {
	switch {
	case r <= 0xff:
		fmt.Fprintf(b, `\x%02x`, r)
	case r <= 0xffff:
		fmt.Fprintf(b, `\u%04x`, r)
	default:
		fmt.Fprintf(b, `\U%08x`, r)
	}
}
