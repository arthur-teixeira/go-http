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

	"github.com/arthur-teixeira/go-http/textreader"
)

type Response struct {
	Url            *url.URL
	Status         string
	StatusCode     int
	Body           io.Reader
	Headers        Headers
	ContentLength  int64
	TransferCoding string
	Close          bool
	request        *Request
}

func ParseResponse(reader *bufio.Reader) (*Response, error) {
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

	headers, err := tr.ReadHeaders()
	if err != nil {
		return nil, err
	}
	r.Headers = headers

	r.StatusCode = statusCode
	r.Status = fmt.Sprintf("%d %s", statusCode, reason)
	err = setBodyResponse(&r, reader)
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

func setBodyResponse(r *Response, rdr *bufio.Reader) error {
	cl, err := GetContentLengthResponse(r)
	if err != nil {
		return err
	}
  r.ContentLength = cl

	if cl <= 0 {
		r.Body = NoBody
	} else {
		r.Body = io.LimitReader(rdr, cl) // TODO: Close body with a ReadCloser
	}

	return nil
}

func GetContentLengthResponse(r *Response) (int64, error) {
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
