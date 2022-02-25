// Package lexer contains a simple lexer for dealing with command line arguments. This package is used to allow us
// to split strings that are meant to be turned into exec.Cmd into the args it understands. This should only take input
// from a controlled environment, as I did not attempt to prevent the myriad of possible injection attacks I'm sure can occur.
// This is for DevOps use where we are reading in files controlled by the engineers and peer reviewed, not for taking from some
// random user.
package lexer

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ItemType describes the type of item being emitted by the Lexer. There are predefined ItemType(s)
// and the rest are defined by the user.
type ItemType int

const (
	// ItemUnknown indicates that the Item is an unknown. This should only happen on
	// a Item that is the zero type.
	ItemUnknown ItemType = iota
	// ItemWord indicates this is simply a word character in the line.
	ItemWord
	// ItemFlag indicates this word started with -- or -. It will not contain an = sign.
	ItemFlag
	// ItemFlagAndValue indicates this word started with a -- or - and has a = sign in it.
	ItemFlagAndValue
	// ItemErr indicates we had an error and the Value will be the output text.
	ItemErr
	// ItemEOL indicates we have reached the end of the line.
	ItemEOL
)

// Item describes a lexed item.
type Item struct {
	// Type is the type of item.
	Type ItemType
	// Value is the value.
	Value string
}

// Line is a line lexer targeted at command line arguments.
type Line struct {
	pos   int
	width int
	input string

	ch chan Item
}

// New is the constructor for Line.
func New() *Line {
	return &Line{}
}

// Parse parses our line into Item(s) and emits them on the returned channel.
func (l *Line) Parse(s string) chan Item {
	l.pos, l.width, l.input = 0, 0, strings.TrimSpace(s)
	l.ch = make(chan Item, 1)

	go func() {
		defer close(l.ch)
		for {
			l.skipSpaces()
			if err := l.parseString(); err != nil {
				if err == io.EOF {
					return
				}
				l.ch <- Item{Type: ItemErr, Value: err.Error()}
				return
			}
		}
	}()
	return l.ch
}

// parseStrings the next string.
func (l *Line) parseString() error {
	raw := strings.Builder{}

	var r rune
	for ; true; r = l.next() {
		switch {
		case string(r) == `'` || string(r) == `"`:
			l.backup()
			s, err := l.untilClose()
			if err != nil {
				return err
			}
			raw.WriteString(s)
			l.emit(raw.String())
			return nil
		case unicode.IsSpace(r):
			if raw.Len() > 0 {
				l.emit(raw.String())
			}
			return nil
		case r == rune(ItemEOL):
			l.emit(raw.String())
			return io.EOF
		case r == '\x00':
			continue
		default:
			raw.WriteRune(r)
		}
	}
	panic("should never get here")
}

// emit emits an Item from the string.
func (l *Line) emit(s string) {
	switch {
	case strings.HasPrefix(s, "-") || strings.HasPrefix(s, "--"):
		if strings.Contains(s, "=") {
			l.ch <- Item{Type: ItemFlagAndValue, Value: s}
			return
		}
		l.ch <- Item{Type: ItemFlag, Value: s}
		return
	}
	l.ch <- Item{Type: ItemWord, Value: s}
}

// untilClose is used when the next character is " or ' and then gathers all characters until the closing quote is seen.
// The next character after the quote must be a EOL or space.
func (l *Line) untilClose() (string, error) {
	raw := strings.Builder{}
	quote := l.next()
	raw.WriteRune(quote)

	for {
		r := l.next()
		switch {
		case r == quote:
			p := l.peek()
			if !unicode.IsSpace(p) && p != rune(ItemEOL) {
				return "", fmt.Errorf("open quote(%s) was not met with a closed quote followed by a space or EOL", string(quote))
			}
			raw.WriteRune(r)
			return raw.String(), nil
		case r == rune(ItemEOL):
			return "", fmt.Errorf("open quote(%s) was never closed", string(quote))
		default:
			raw.WriteRune(r)
		}
	}
}

// skipSpaces skips all spaces until the next non-space character.
func (l *Line) skipSpaces() {
	for {
		r := l.peek()
		if unicode.IsSpace(r) {
			l.next()
			continue
		}
		break
	}
}

// backup goes back a character.
func (l *Line) backup() {
	l.pos -= l.width
}

// next returns the next rune in the input.
func (l *Line) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return rune(ItemEOL)
	}
	var r rune
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

// Peek returns but does not consume the next rune in the input.
func (l *Line) peek() rune {
	if l.pos >= len(l.input) {
		return rune(ItemEOL)
	}
	r := l.next()
	l.backup()
	return r
}
