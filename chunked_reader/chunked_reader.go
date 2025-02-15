package chunked_reader

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

type ChunkedReader struct {
	rdr      *bufio.Reader
	buf      [2]byte
	err      error
	n        uint64
	checkEnd bool
}

func NewChunkedReader(rdr io.Reader) io.Reader {
	r, ok := rdr.(*bufio.Reader)
	if !ok {
		r = bufio.NewReader(rdr)
	}
	return &ChunkedReader{
		rdr: r,
	}
}

func (c *ChunkedReader) chunkHeaderAvailable() bool {
	n := c.rdr.Buffered()
	if n > 0 {
		peek, _ := c.rdr.Peek(n)
		return bytes.IndexByte(peek, '\n') >= 0
	}

	return false
}

func (c *ChunkedReader) Read(b []byte) (n int, err error) {
	for c.err == nil {
		if c.checkEnd {
			if n > 0 && c.rdr.Buffered() < 2 {
				// Return early instead of blocking
				break
			}

			if _, c.err = io.ReadFull(c.rdr, c.buf[:2]); c.err == nil {
				if string(c.buf[:]) != "\r\n" {
					c.err = errors.New("malformed chunk")
					break
				}
			} else {
				if c.err == io.EOF {
					c.err = io.ErrUnexpectedEOF
				}
				break
			}
			c.checkEnd = false
		}
		if c.n == 0 {
			if n > 0 && !c.chunkHeaderAvailable() {
				break
			}
			c.beginChunk()
			continue
		}
		if len(b) == 0 {
			break
		}

		rbuf := b
		if uint64(len(rbuf)) > c.n {
			rbuf = rbuf[:c.n]
		}
		var n0 int
		n0, c.err = c.rdr.Read(rbuf)
		n += n0
		b = b[n0:]
		c.n -= uint64(n0)
		if c.n == 0 && c.err == nil {
			c.checkEnd = true
		} else if c.err == io.EOF {
			c.err = io.ErrUnexpectedEOF
		}
	}
	return n, c.err
}

func trimTrailingWhitespace(b []byte) []byte {
	for len(b) > 0 && isASCIISpace(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return b
}

func isASCIISpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func (c *ChunkedReader) beginChunk() {
	var line []byte
	line, c.err = readChunkLine(c.rdr)
	if c.err != nil {
		return
	}
	line = trimTrailingWhitespace(line)
	line, _, _ = bytes.Cut(line, []byte(";"))
	c.n, c.err = parseUint(line)
	if c.err != nil {
		return
	}

	if c.n == 0 {
		c.err = io.EOF
	}
}

func parseUint(line []byte) (uint64, error) {
	if len(line) == 0 {
		return 0, errors.New("Empty hex for chunk length")
	}

	n := uint64(0)

	for _, b := range line {
		switch {
		case '0' <= b && b <= '9':
			b = b - '0'
		case 'A' <= b && b <= 'F':
			b = b - 'A' + 10
		case 'a' <= b && b <= 'f':
			b = b - 'a' + 10
		default:
			return 0, errors.New("Invalid Hex character")
		}

		n <<= 4
		n |= uint64(b)
	}

	return n, nil
}

const maxLineLength = 4096

func readChunkLine(b *bufio.Reader) ([]byte, error) {
	p, err := b.ReadSlice('\n')
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	if len(p) >= maxLineLength {
		return nil, errors.New("line too long")
	}

	return p, nil
}
