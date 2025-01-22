package context

import (
	"bufio"
	"fmt"
	"net"
	"strconv"

	"github.com/arthur-teixeira/go-http/parser"
	"github.com/arthur-teixeira/go-http/status"
)

type ResponseWriter struct {
	bw     *bufio.Writer
	status int
}

type Context struct {
	Request  *parser.Request
	Response ResponseWriter
}

func (c *Context) Status(code int) {
	c.Response.status = code
}

func (c *Context) WriteHeader(code int) {
	c.Status(code)
	c.writeStatusLine()

	// TODO: Write remaining headers
	if c.Request.Close {
		c.Response.bw.WriteString("Connection: close\r\n")
	}

	c.Response.bw.WriteString("\r\n")
	c.Response.bw.Flush()
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
