package parser

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/arthur-teixeira/go-http/textreader"
	"github.com/arthur-teixeira/go-http/transport"
)

type Response struct {
	Url            *url.URL
	Status         string
	StatusCode     int
	Body           io.ReadCloser
	Headers        Headers
	ContentLength  int64
	TransferCoding string
	Close          bool
	Major          int
	Minor          int
	Version        string
	request        *Request
	Trailer        Headers
	Chunked        bool
}

func ParseResponse(src transport.Reusable) (*Response, error) {
	reader := bufio.NewReader(src)
	tr := textreader.NewTextReader(reader)
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
	var r Response

	version, statusCode, reason, ok := parseStatusLine(line)
	if !ok {
		return nil, StringError("Malformed HTTP response", version)
	}

	major, minor, ok := ParseHttpVersion(version)
	if !ok {
		return nil, StringError("Invalid protocol version", version)
	}
	r.Version = version
	r.Major = major
	r.Minor = minor

	headers, err := tr.ReadHeaders()
	if err != nil {
		return nil, err
	}
	r.Headers = headers

	r.StatusCode = statusCode
	r.Status = fmt.Sprintf("%d %s", statusCode, reason)
	err = setBody(&r, reader, src)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

func parseStatusLine(line string) (version string, statusCode int, reason string, ok bool) {
	version, rest, ok1 := strings.Cut(line, " ")
	status, reason, ok2 := strings.Cut(rest, " ")
	if !ok1 || !ok2 {
		return "", 0, "", false
	}

	statusCode, err := strconv.Atoi(status)
	if err != nil {
		return "", 0, "", false
	}

	return version, statusCode, reason, true
}
