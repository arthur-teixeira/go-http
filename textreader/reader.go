package textreader

import (
	"bufio"
	"errors"
	"net/http"
	"strings"
	"sync"
)

var readerPool sync.Pool

type TextReader struct {
	R *bufio.Reader
}

func NewTextReader(rv *bufio.Reader) *TextReader {
	if r := readerPool.Get(); r != nil {
		tr := r.(*TextReader)
		tr.R = rv
		return tr
	}
	return &TextReader{R: rv}
}

func PutTextReader(r *TextReader) {
	r.R = nil
	readerPool.Put(r)
}

func (tr *TextReader) ReadLine() (string, error) {
	line, err := tr.readLineSlice()
	return string(line), err
}

func (tr *TextReader) readLineSlice() ([]byte, error) {
	var line []byte
	for {
		l, more, err := tr.R.ReadLine()
		if err != nil {
			return nil, err
		}
		if line == nil && !more {
			return l, nil
		}
		line = append(line, l...)
		if !more {
			break
		}
	}

	return line, nil
}

func (tr *TextReader) ReadHeaders() (map[string][]string, error) {
	headers := make(map[string][]string)
	for {
		line, err := tr.ReadLine()
		if err != nil {
			return nil, err
		}

    if line == "" {
      break
    }

		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			return nil, errors.New("malformed Header, missing key")
		}
    val := strings.TrimSpace(rest)
		headers[http.CanonicalHeaderKey(key)] = append(headers[http.CanonicalHeaderKey(key)], val)
	}

  return headers, nil
}
