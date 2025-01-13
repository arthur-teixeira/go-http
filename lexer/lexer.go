package lexer

import (
	"unicode"
)

const (
	CR = iota
	LF
	STR
	EOF
)

type Lexer struct {
	input []rune
	len   int
	pos   int
}

type Token struct {
	Type  int
	Value string
}

func NewLexer(input string) Lexer {
	i := []rune(input)
	return Lexer{
		input: i,
		len:   len(i),
		pos:   0,
	}
}

func (l *Lexer) skipWhitespace() {
	for l.input[l.pos] == ' ' && l.pos < l.len-1 {
		l.pos++
	}
}

func (l *Lexer) NextToken() Token {
	if l.pos >= l.len {
		return Token{Type: EOF}
	}

	l.skipWhitespace()

	start := l.pos
	ch := byte(l.input[l.pos])

	if ch == '\r' {
		l.pos++
		return Token{Type: CR}
	} else if ch == '\n' {
		l.pos++
		return Token{Type: LF}
	}

	for !unicode.IsSpace(l.input[l.pos]) && l.pos < l.len-1 {
		l.pos++
	}
	end := l.pos

	tok := l.input[start:end]

	return Token{
		Type:  STR,
		Value: string(tok),
	}
}
