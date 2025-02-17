package parser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"github.com/arthur-teixeira/go-http/chunkedreader"
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

func (h Headers) Set(k string, v string) {
	kk := textproto.CanonicalMIMEHeaderKey(k)
	h[kk] = []string{v}
}

func StringError(what, how string) error {
	return fmt.Errorf("%s %q", what, how)
}

type Request struct {
	Ctx           context.Context
	Cancel        <-chan struct{}
	Close         bool
	ContentLength int64
	Chunked       bool
	Method        string
	Headers       Headers
	RequestURI    string
	URL           *url.URL
	Version       string
	Major         int
	Minor         int
	Body          io.Reader
	ReadBody      func() (io.ReadCloser, error)
	Host          string
	Trailer       Headers
}

func (r *Request) Context() context.Context {
	if r.Ctx != nil {
		return r.Ctx
	}

	return context.Background()
}

func (r Request) ProtoAtLeast(maj int, min int) bool {
	return maj > r.Major || (maj == r.Major && min >= r.Minor)
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
		return nil, StringError("Malformed HTTP request", method)
	}

	r.Major, r.Minor, ok = ParseHttpVersion(proto)
	if !ok {
		return nil, StringError("Invalid protocol version", uri)
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
	r.setClose()

	return &r, nil
}

var NoBody = noBody{}

type noBody struct{}

func (noBody) Read([]byte) (int, error)         { return 0, io.EOF }
func (noBody) Close() error                     { return nil }
func (noBody) WriteTo(io.Writer) (int64, error) { return 0, nil }

func parseTransferCoding(r *Transfer) error {
	val, ok := r.Headers["Transfer-Encoding"]
	if !ok {
		return nil
	}
	delete(r.Headers, "Transfer-Encoding")

	// Chunked transfer is not supported in HTTP/1.0
	if !r.ProtoAtLeast(1, 1) {
		return nil
	}

	if len(val) > 1 {
		return errors.New("Too many values for Transfer-Encoding")
	}

	if !strings.EqualFold(val[0], "chunked") {
		return errors.New("Unsupported transfer encoding")
	}

	r.Chunked = true
	return nil
}

type Transfer struct {
	Body          io.Reader
	Trailer       Headers
	Headers       Headers
	ContentLength int64
	Major         int
	Minor         int
	Version       string
	Chunked       bool
}

func (t *Transfer) ProtoAtLeast(maj int, min int) bool {
	return maj > t.Major || (maj == t.Major && min >= t.Minor)
}

// takes in either *Request or *Response
func setBody(r any, rdr *bufio.Reader) error {
	tr := Transfer{}

	switch rr := r.(type) {
	case *Request:
		tr.Headers = rr.Headers
		tr.Major = rr.Major
		tr.Minor = rr.Minor
		tr.Version = rr.Version
	case *Response:
		tr.Headers = rr.Headers
		tr.Major = rr.Major
		tr.Minor = rr.Minor
		tr.Version = rr.Version
	}

	if err := parseTransferCoding(&tr); err != nil {
		return err
	}

	cl, err := GetContentLength(&tr)
	if err != nil {
		return err
	}

	tr.Trailer, err = getTrailer(tr.Headers, tr.Chunked)
	if err != nil {
		return err
	}

	switch {
	case tr.Chunked:
		tr.Body = chunkedreader.NewChunkedReader(rdr)
	case cl == 0:
		tr.Body = NoBody
	default:
		tr.Body = io.LimitReader(rdr, cl)
	}

	switch rr := r.(type) {
	case *Request:
		rr.Body = tr.Body
		rr.Trailer = tr.Trailer
		rr.Chunked = tr.Chunked
		rr.ContentLength = tr.ContentLength
	case *Response:
		rr.Body = tr.Body
		rr.Trailer = tr.Trailer
		rr.Chunked = tr.Chunked
		if rr.Chunked {
			rr.TransferCoding = "chunked"
		}
		rr.ContentLength = tr.ContentLength
	}

	return nil
}

func forEachHeaderElement(v string, cb func(string)) {
	v = textproto.TrimString(v)
	if v == "" {
		return
	}
	if !strings.Contains(v, ",") {
		cb(v)
		return
	}

	for _, f := range strings.Split(v, ",") {
		if f = textproto.TrimString(f); f != "" {
			cb(f)
		}
	}
}

func getTrailer(headers Headers, chunked bool) (Headers, error) {
	vv, ok := headers["Trailer"]
	if !ok {
		return nil, nil
	}
	if !chunked {
		return nil, nil
	}
	headers.Del("Trailer")
	trailer := make(Headers)
	var err error
	for _, v := range vv {
		forEachHeaderElement(v, func(key string) {
			key = http.CanonicalHeaderKey(key)
			switch key {
			case "Transfer-Encoding", "Trailer", "Content-Length":
				if err == nil {
					err = StringError("bad trailer key", key)
					return
				}
			}
			trailer[key] = nil
			return
		})
	}

	if err != nil {
		return nil, err
	}

	if len(trailer) == 0 {
		return nil, nil
	}

	return trailer, nil
}

func (r *Request) setClose() {
	r.Close = strings.Contains(r.Headers.Get("Connection"), "close")
}

func GetContentLength(r *Transfer) (int64, error) {
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

	if r.Chunked {
		r.Headers.Del("Content-Length")
		return -1, nil
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
