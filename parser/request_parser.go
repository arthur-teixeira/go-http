package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/arthur-teixeira/go-http/textreader"
)

type Headers map[string][]string

func (h Headers) Get(k string) string {
	vs := h[k]
	if len(vs) > 0 {
		return vs[0]
	}

	return ""
}

func (h Headers) Del(k string) {
	delete(h, k)
}

func (h Headers) Add(k string, v string) {
	kk := textproto.CanonicalMIMEHeaderKey(k)
	h[kk] = append(h[kk], v)
}

type ParserError string

func stringError(what, how string) error {
	return fmt.Errorf("%s %q", what, how)
}

type body struct {
	mu  sync.Mutex
	src io.Reader
}

func (b *body) Read(bs []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.src.Read(bs)
}

type Request struct {
	Method     string
	Headers    Headers
	RequestURI string
	URL        *url.URL
	Version    string
	Major      int
	Minor      int
	Body       *body
	Host       string
}

// Copyright 2009 The Go Authors.
// Copied from https://cs.opensource.google/go/x/net/+/master:http/httpguts/httplex.go
var isTokenTable = [127]bool{
	'!':  true,
	'#':  true,
	'$':  true,
	'%':  true,
	'&':  true,
	'\'': true,
	'*':  true,
	'+':  true,
	'-':  true,
	'.':  true,
	'0':  true,
	'1':  true,
	'2':  true,
	'3':  true,
	'4':  true,
	'5':  true,
	'6':  true,
	'7':  true,
	'8':  true,
	'9':  true,
	'A':  true,
	'B':  true,
	'C':  true,
	'D':  true,
	'E':  true,
	'F':  true,
	'G':  true,
	'H':  true,
	'I':  true,
	'J':  true,
	'K':  true,
	'L':  true,
	'M':  true,
	'N':  true,
	'O':  true,
	'P':  true,
	'Q':  true,
	'R':  true,
	'S':  true,
	'T':  true,
	'U':  true,
	'W':  true,
	'V':  true,
	'X':  true,
	'Y':  true,
	'Z':  true,
	'^':  true,
	'_':  true,
	'`':  true,
	'a':  true,
	'b':  true,
	'c':  true,
	'd':  true,
	'e':  true,
	'f':  true,
	'g':  true,
	'h':  true,
	'i':  true,
	'j':  true,
	'k':  true,
	'l':  true,
	'm':  true,
	'n':  true,
	'o':  true,
	'p':  true,
	'q':  true,
	'r':  true,
	's':  true,
	't':  true,
	'u':  true,
	'v':  true,
	'w':  true,
	'x':  true,
	'y':  true,
	'z':  true,
	'|':  true,
	'~':  true,
}

func IsTokenRune(r rune) bool {
	i := int(r)
	return i < len(isTokenTable) && isTokenTable[i]
}

func isNotToken(r rune) bool {
	return !IsTokenRune(r)
}

func isValidMethod(method string) bool {
	return len(method) > 0 && strings.IndexFunc(method, isNotToken) == -1
}

func ParseHttpVersion(version string) (major, minor int, ok bool) {
	switch version {
	case "HTTP/1.1":
		return 1, 1, true
	case "HTTP/1.0":
		return 1, 0, true
	}
	if !strings.HasPrefix(version, "HTTP/") {
		return 0, 0, false
	}

	if len(version) != len("HTTP/X.Y") {
		return 0, 0, false
	}

	maj, err := strconv.ParseUint(version[5:6], 10, 0)
	if err != nil {
		return 0, 0, false
	}

	minv, err := strconv.ParseUint(version[7:8], 10, 0)
	if err != nil {
		return 0, 0, false
	}

	return int(maj), int(minv), true
}

func ParseRequest(request *bufio.Reader) (*Request, error) {
	tr := textreader.NewTextReader(request)
	defer textreader.PutTextReader(tr)

	line, err := tr.ReadLine()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()
	var r Request

	method, uri, proto, ok := parseRequestLine(line)
	if !ok {
		return nil, stringError("Malformed HTTP request", method)
	}

	r.Major, r.Minor, ok = ParseHttpVersion(proto)
	if !ok {
		return nil, stringError("Invalid protocol version", uri)
	}
	r.Method = method
	r.Version = proto
	r.RequestURI = uri

	rawUrl := r.RequestURI
	justAuthority := r.Method == "CONNECT" && !strings.HasPrefix(rawUrl, "/")
	if justAuthority {
		rawUrl = "http://" + rawUrl
	}

	if r.URL, err = url.ParseRequestURI(rawUrl); err != nil {
		return nil, err
	}

	if justAuthority {
		r.URL.Scheme = ""
	}

	headers, err := tr.ReadHeaders()
	if err != nil {
		return nil, err
	}

	r.Headers = Headers(headers)
	r.Host = r.URL.Host
	if r.Host == "" {
		r.Host = r.Headers.Get("Host")
	}
	err = setBody(&r, request)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

var NoBody = noBody{}

type noBody struct{}

func (noBody) Read([]byte) (int, error) { return 0, io.EOF }

func setBody(r *Request, rdr *bufio.Reader) error {
	cl, err := GetContentLength(r)
	if err != nil {
		return err
	}

	if cl <= 0 {
		r.Body = &body{
			src: NoBody,
		}

	} else {

		r.Body = &body{
			src: io.LimitReader(rdr, cl),
		}
	}

	return nil
}

func GetContentLength(r *Request) (int64, error) {
	contentLens := r.Headers["Content-Length"]
	if len(contentLens) == 0 {
		return -1, nil
	}
	if len(contentLens) > 1 {
		first := textproto.TrimString(contentLens[0])
		for _, ct := range contentLens[1:] {
			if first != textproto.TrimString(ct) {
				return 0, errors.New("Request has multiple content lengths")
			}
		}

		r.Headers.Del("Content-Length")
		r.Headers.Add("Content-Length", first)
		contentLens = r.Headers["Content-Length"]
	}
	cl := textproto.TrimString(contentLens[0])
	if cl == "" {
		return -1, errors.New("Invalid empty Content length")
	}
	n, err := strconv.ParseUint(cl, 10, 63)
	if err != nil {
		return -1, errors.New("Bad content length")
	}

	return int64(n), nil
}

func parseRequestLine(line string) (method, uri, proto string, ok bool) {
	method, rest, ok1 := strings.Cut(line, " ")
	uri, proto, ok2 := strings.Cut(rest, " ")
	if !ok1 || !ok2 {
		return "", "", "", false
	}

	return method, uri, proto, true
}
