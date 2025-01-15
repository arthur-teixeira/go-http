package lexer

import (
	"strings"
	"unicode"
)

const (
	CR = iota
	LF
	STR
	EOF
	TEXT
	VERSION
	OPTIONS
	GET
	HEAD
	POST
	PUT
	DELETE
	TRACE
	CONNECT
	R_OPTIONS = "OPTIONS"
	R_GET     = "GET"
	R_HEAD    = "HEAD"
	R_POST    = "POST"
	R_PUT     = "PUT"
	R_DELETE  = "DELETE"
	R_TRACE   = "TRACE"
	R_CONNECT = "CONNECT"
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

func GetTextToken(str string) Token {
	if strings.HasPrefix(str, "HTTP/") {
		version := strings.TrimPrefix(str, "HTTP/")
		return Token{
			Type:  VERSION,
			Value: version,
		}
	}

	if str == R_CONNECT {
		return Token{
			Type:  CONNECT,
			Value: str,
		}
	}

	if str == R_TRACE {
		return Token{
			Type:  TRACE,
			Value: str,
		}
	}

	if str == R_DELETE {
		return Token{
			Type:  DELETE,
			Value: str,
		}
	}

	if str == R_PUT {
		return Token{
			Type:  PUT,
			Value: str,
		}
	}

	if str == R_POST {
		return Token{
			Type:  POST,
			Value: str,
		}
	}

	if str == R_HEAD {
		return Token{
			Type:  HEAD,
			Value: str,
		}
	}

	if str == R_GET {
		return Token{
			Type:  GET,
			Value: str,
		}
	}

	if str == R_OPTIONS {
		return Token{
			Type:  OPTIONS,
			Value: str,
		}
	}

	return Token{
		Type:  STR,
		Value: str,
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

	for l.pos < l.len && !unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}

	end := l.pos

	tok := string(l.input[start:end])
	return GetTextToken(tok)
}

func (l *Lexer) NextLine() Token {
	if l.pos >= l.len {
		return Token{Type: EOF}
	}

	start := l.pos
	for l.pos < l.len && l.input[l.pos] != '\r' && l.input[l.pos] != '\n' {
		l.pos++
	}

	end := l.pos

	return Token{
		Type:  STR,
		Value: string(l.input[start:end]),
	}
}

func (l *Lexer) Body() Token {
	// TODO: Get all text after first empty line
	// This function should be called by the parser after an empty line (\r\n) is found. Returns the rest of the request
	return Token{}
}
