package parser

import (
	"fmt"

	"github.com/arthur-teixeira/go-http/lexer"
	"github.com/arthur-teixeira/go-http/status"
)

type ParserError struct {
	code    int
	message string
}

func (pe *ParserError) Error() string {
	return fmt.Sprintf("%d %s", pe.code, pe.message)
}

func NewError(message string, code int) *ParserError {
	return &ParserError{
		message: message,
		code:    code,
	}
}

type Request struct {
	Method  string
	Headers map[string]string
	URI     string
	Version string
	Body    string
}

type RequestParser struct {
	l         lexer.Lexer
	curToken  lexer.Token
	peekToken lexer.Token
	result    Request
}

func NewParser(input string) RequestParser {
	p := RequestParser{
		l: lexer.NewLexer(input),
		result: Request{
			Headers: make(map[string]string),
		},
	}
	p.curToken = p.l.NextToken()
	p.peekToken = p.l.NextToken()

	return p
}

func (p *RequestParser) NextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *RequestParser) ExpectPeek(expected int) bool {
	r := p.peekToken.Type == expected
	if r {
		p.NextToken()
	}
	return r
}

func (p *RequestParser) Parse() (*Request, *ParserError) {
	err := p.ParseRequestLine()
	if err != nil {
		return nil, err
	}
	return &p.result, nil
}

func (p *RequestParser) parseMethod() *ParserError {
	p.NextToken()

	for {
		switch p.curToken.Type {
		case lexer.STR:
			return NewError(fmt.Sprintf("Got a string instead of a method: %s", p.curToken.Value), status.NotImplemented)
		case lexer.VERSION:
			return NewError("Got HTTP version before Method", status.BadRequest)
			// TODO:
		case lexer.CR:
		case lexer.LF:
			return NewError("Should handle this case", status.ExpectationFailed)
		case lexer.EOF:
			return NewError("Request truncated", status.BadRequest)
		case lexer.CONNECT:
			return NewError("Cannot handle Proxy functionality yet", status.NotImplemented)
		default:
			return nil
		}
	}
}

func (p *RequestParser) ParseRequestLine() *ParserError {
	err := p.parseMethod()
	if err != nil {
		return err
	}

	p.result.Method = p.curToken.Value

	// TODO: improve URI lexing
	if !p.ExpectPeek(lexer.STR) {
		return NewError("Expected request URI", status.BadRequest)
	}
	p.result.URI = p.curToken.Value

	if !p.ExpectPeek(lexer.VERSION) {
		return NewError("Expected Protocol version", status.BadRequest)
	}
	p.result.Version = p.curToken.Value

	if !p.ExpectPeek(lexer.CR) || !p.ExpectPeek(lexer.LF) {
		return NewError("Malformed request line", status.BadRequest)
	}

	return nil
}
