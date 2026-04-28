package agent

import (
	"strings"
)

type Command struct {
	Args []string
}

func ParseCommand(s string) Command {
	if s == "" {
		return Command{}
	}
	return Command{Args: parseCommandParts(s)}
}

func (c Command) Binary() string {
	if len(c.Args) == 0 {
		return ""
	}
	return c.Args[0]
}

func (c Command) IsZero() bool {
	return len(c.Args) == 0
}

func parseCommandParts(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			if ch == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(ch)
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inQuote = true
			quoteChar = ch
		case ' ', '\t':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
