// Package parser provides a line parser that takes strings representing
// commands and turns them into a list of args. It does not attempt to do things like
// shell interpretation, simply a straight conversion.
package parser

import (
	"errors"
	"fmt"

	"github.com/element-of-surprise/runme/internal/lexer"
)

type Line struct {
	text  string
	lexer *lexer.Line
}

func (l *Line) Parse(s string) ([]string, error) {
	if l.lexer == nil {
		l.lexer = lexer.New()
	}
	ch := l.lexer.Parse(s)

	args := []string{}

	for item := range ch {
		switch item.Type {
		case lexer.ItemErr:
			return nil, errors.New(item.Value)
		case lexer.ItemWord, lexer.ItemFlag, lexer.ItemFlagAndValue:
			args = append(args, item.Value)
		}
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("must have at least the program name")
	}

	return args, nil
}
