package lexer_test

import (
	"testing"

	"github.com/arthur-teixeira/go-http/lexer"
	"github.com/stretchr/testify/assert"
)

func assertEqualToken(t *testing.T, expected lexer.Token, actual lexer.Token) {
  assert.Equal(t, expected.Type, actual.Type)
  assert.Equal(t, expected.Value, actual.Value)
}

func TestLexer(t *testing.T) {
  input := "GET / HTTP/1.1\r\n"
  l := lexer.NewLexer(input)

  assertEqualToken(t, lexer.Token{Type: lexer.STR, Value: "GET"}, l.NextToken())
  assertEqualToken(t, lexer.Token{Type: lexer.STR, Value: "/"}, l.NextToken())
  assertEqualToken(t, lexer.Token{Type: lexer.STR, Value: "HTTP/1.1"}, l.NextToken())
  assertEqualToken(t, lexer.Token{Type: lexer.CR}, l.NextToken())
  assertEqualToken(t, lexer.Token{Type: lexer.LF}, l.NextToken())
  assertEqualToken(t, lexer.Token{Type: lexer.EOF}, l.NextToken())
}
