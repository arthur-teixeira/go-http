package context

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/arthur-teixeira/go-http/parser"
	"github.com/arthur-teixeira/go-http/status"
)

type ResponseWriter struct {
	bw         *bufio.Writer
	status     int
	Headers    parser.Headers
	contentLen int
}

func NewWriter(bw *bufio.Writer) ResponseWriter {
	return ResponseWriter{bw: bw}
}

type Context struct {
	Request     *parser.Request
	Response    ResponseWriter
	wroteHeader bool
}

func (c *Context) Status(code int) {
	c.Response.status = code
}

// Copied from https://github.com/gin-gonic/gin/blob/master/context.go#L985
func (c *Context) Header(key, value string) {
	if value == "" {
		c.Response.Headers.Del(key)
		return
	}
	c.Response.Headers.Set(key, value)
}

// Writes the response Status Line e.g: HTTP/1.1 200 OK
func (c *Context) WriteHeader(code int) {
	if c.wroteHeader {
		log.Printf("[WARNING]: Header was already written. Tried to overwrite %d with %d", c.Response.status, code)
		return
	}

	c.Status(code)
	c.writeStatusLine()
	c.Response.bw.Flush()
	c.wroteHeader = true
}

func writeHeaders(req *parser.Request, bw *bufio.Writer, contentLen int, hdrs parser.Headers) (int, error) {
	written := 0
	if req.Close {
		n, err := bw.WriteString("Connection: close\r\n")
		if err != nil {
			return -1, err
		}

		written += n
	}

	if contentLen > 0 {
		n, err := bw.WriteString("Content-Length: " + strconv.Itoa(contentLen) + "\r\n")
		if err != nil {
			return -1, err
		}
		written += n
	}

	// Assuming key is in canonical form when inserted
	for k, v := range hdrs {
		final := ""
		for _, vv := range v {
			trimmed := strings.TrimSpace(vv)
			if trimmed != "" {
				final += ", " + trimmed
			}
		}

		if k == "" || final == "" {
			continue
		}
		final = final[2:]

		n, err := bw.WriteString(strings.TrimSpace(k))
		if err != nil {
			return -1, err
		}
		written += n

		n, err = bw.WriteString(": ")
		if err != nil {
			return -1, err
		}
		written += n

		n, err = bw.WriteString(final)
		if err != nil {
			return -1, err
		}
		written += n

		n, err = bw.WriteString("\r\n")
		if err != nil {
			return -1, err
		}
		written += n
	}

	n, err := bw.WriteString("\r\n")
	if err != nil {
		return -1, err
	}
	return written + n, nil
}

// Writes the response headers to the underlying connection
func (c *Context) writeHeaders() (int, error) {
	return writeHeaders(c.Request, c.Response.bw, c.Response.contentLen, c.Request.Headers)
}

func (c *Context) Write(data []byte) (int, error) {
	c.Response.contentLen = len(data)
	fmt.Println("Content length is", c.Response.contentLen)
	if !c.wroteHeader {
		c.WriteHeader(status.OK) // Following Go default behavior
	}

	written := 0
	n, err := c.writeHeaders()
	if err != nil {
		return -1, err
	}
	written += n

	n, err = c.Response.bw.Write(data)
	if err != nil {
		return -1, err
	}
	written += n
	err = c.Response.bw.Flush()
	if err != nil {
		return -1, err
	}

	return written, nil
}

func (c *Context) WriteString(data string) (int, error) {
	return c.Write([]byte(data))
}

func (c *Context) writeStatusLine() {
	code := c.Response.status

	if c.Request.ProtoAtLeast(1, 1) {
		c.Response.bw.WriteString("HTTP/1.1 ")
	} else {
		c.Response.bw.WriteString("HTTP/1.0 ")
	}

	if text := status.Text(code); text != "" {
		c.Response.bw.WriteString(strconv.Itoa(code))
		c.Response.bw.WriteByte(' ')
		c.Response.bw.WriteString(text)
		c.Response.bw.WriteString("\r\n")
	} else {
		fmt.Fprintf(c.Response.bw, "%03d status code %d\r\n", code, code)
	}
}

func NewContext(conn net.Conn) (*Context, error) {
	b := bufio.NewReader(conn)
	bw := bufio.NewWriter(conn)
	req, err := parser.ParseRequest(b)
	if err != nil {
		return nil, err
	}

	return &Context{
		Request: req,
		Response: ResponseWriter{
			bw:     bw,
			status: 0,
		},
	}, nil
}

type Response struct {
	Url            *url.URL
	Status         string
	StatusCode     int
	Body           io.ReadCloser
	Header         parser.Headers
	ContentLength  int
	TransferCoding string
	Close          bool
	request        *parser.Request
}

func (r *Response) CloseBody() error {
	if r.Body == nil {
		return nil
	}
	return r.Body.Close()
}

func validateMethod(method string) error {
	switch method {
	case "OPTIONS":
	case "GET":
	case "HEAD":
	case "POST":
	case "PUT":
	case "DELETE":
	case "TRACE":
		return nil
	default:
		return errors.New("http: invalid method")
	}

	return nil
}

func writeRequestLine(wtr *bufio.Writer, req *parser.Request) error {
	err := validateMethod(req.Method)
	if err != nil {
		return err
	}

	reqLine := req.Method + " " + req.URL.Path + "\r\n"
	_, err = wtr.WriteString(reqLine)
	return err
}

func Do(req *parser.Request) (*Response, error) {
	sock, err := net.Dial("tcp", req.URL.Host)
	if err != nil {
		return nil, err
	}

	if req.URL == nil {
		return nil, errors.New("http: nil URL")
	}

	bw := bufio.NewWriter(sock)
	err = writeRequestLine(bw, req)
	if err != nil {
		return nil, err
	}

	switch v := req.Body.(type) {
	case *bytes.Buffer:
		req.ContentLength = int64(v.Len())
	case *bytes.Reader:
		req.ContentLength = int64(v.Len())
	case *strings.Reader:
		req.ContentLength = int64(v.Len())
	default:
		if req.ContentLength == 0 {
			req.Body = parser.NoBody
		}
	}

	_, err = writeHeaders(req, bw, int(req.ContentLength), req.Headers)
	if err != nil {
		return nil, err
	}

	nr, err := io.Copy(bw, io.LimitReader(req.Body, req.ContentLength))
	if err != nil {
		return nil, err
	}
	if nr < req.ContentLength {
		return nil, errors.New("http: Could not write whole body")
	}

	// TODO: Read response
	// TODO: Read response while writing request in case Server responds before we finish.

	return nil, nil
}
