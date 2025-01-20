package parser_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/arthur-teixeira/go-http/parser"
	"github.com/stretchr/testify/assert"
)

func makeReader(s string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(s))
}

func TestParser(t *testing.T) {
	request :=
		"POST /test HTTP/1.1\r\n" +
			"Host: test-domain.com:7017\r\n" +
			"User-Agent: Mozilla/5.0 (Windows; U; Windows NT 5.1; en-US; rv:1.9.0.1) Gecko/2008070208 Firefox/3.0.1\r\n" +
			"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n" +
			"Accept-Language: en-us,en;q=0.5\r\n" +
			"Accept-Encoding: gzip,deflate\r\n" +
			"Accept-Charset: ISO-8859-1,utf-8;q=0.7,*;q=0.7\r\n" +

			"Keep-Alive: 300\r\n" +
			"Connection: keep-alive\r\n" +
			"Referer: http://test-domain.com:7017/index.html\r\n" +
			"Cookie: __utma=43166241.217413299.1220726314.1221171690.1221200181.16; __utmz=43166241.1220726314.1.1.utmccn=(direct)|utmcsr=(direct)|utmcmd=(none)\r\n" +
			"Cache-Control: max-age=0\r\n" +
			"Content-Type: application/x-www-form-urlencoded\r\n" +
			"Content-Length: 25\r\n" +
			"\r\n" +
			"field1=asfd&field2=a3f3f3\r\n"
	rdr := makeReader(request)
	r, err := parser.ParseRequest(rdr)

	assert.Nil(t, err)
	assert.Equal(t, r.Method, "POST")
	assert.Equal(t, r.RequestURI, "/test")
	assert.Equal(t, r.Major, 1)
	assert.Equal(t, r.Minor, 1)
	assert.Equal(t, r.Version, "HTTP/1.1")
	assert.Equal(t, r.Host, "test-domain.com:7017")
	assert.Equal(t, r.Headers, parser.Headers{
		"Host":            []string{"test-domain.com:7017"},
		"User-Agent":      []string{"Mozilla/5.0 (Windows; U; Windows NT 5.1; en-US; rv:1.9.0.1) Gecko/2008070208 Firefox/3.0.1"},
		"Accept":          []string{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		"Accept-Language": []string{"en-us,en;q=0.5"},
		"Accept-Encoding": []string{"gzip,deflate"},
		"Accept-Charset":  []string{"ISO-8859-1,utf-8;q=0.7,*;q=0.7"},
		"Keep-Alive":      []string{"300"},
		"Connection":      []string{"keep-alive"},
		"Referer":         []string{"http://test-domain.com:7017/index.html"},
		"Cookie":          []string{"__utma=43166241.217413299.1220726314.1221171690.1221200181.16; __utmz=43166241.1220726314.1.1.utmccn=(direct)|utmcsr=(direct)|utmcmd=(none)"},
		"Cache-Control":   []string{"max-age=0"},
		"Content-Type":    []string{"application/x-www-form-urlencoded"},
		"Content-Length":  []string{"25"},
	})

	body := make([]byte, 256)
	nb, err := r.Body.Read(body)
	body = body[:nb]
	assert.Nil(t, err)
	assert.Equal(t, string(body), "field1=asfd&field2=a3f3f3")
}

func TestInvalidContentLength(t *testing.T) {
	request := "POST /test HTTP/1.1\r\n" +
		"Content-Length: inv\r\n" +
		"\r\n" +
		"InvalidBody"
	reader := makeReader(request)
	r, err := parser.ParseRequest(reader)
	assert.Nil(t, r)
	assert.NotNil(t, err)

	assert.Equal(t, err.Error(), "Bad content length")
}

func TestMultiplecontentLengths(t *testing.T) {
	request := "POST /test HTTP/1.1\r\n" +
		"Content-Length: 25\r\n" +
		"Content-Length: 35\r\n" +
		"\r\n" +
		"InvalidBody"
	reader := makeReader(request)
	r, err := parser.ParseRequest(reader)
	assert.Nil(t, r)
	assert.NotNil(t, err)

	assert.Equal(t, err.Error(), "Request has multiple content lengths")
}
