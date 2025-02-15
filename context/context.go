package context

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/arthur-teixeira/go-http/parser"
	"github.com/arthur-teixeira/go-http/status"
	"golang.org/x/net/idna"
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

func validateMethod(method string) error {
	switch method {
	case "OPTIONS":
	case "GET":
	case "HEAD":
	case "POST":
	case "PUT":
	case "DELETE":
	case "TRACE":
	case "CONNECT":
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

	ruri := req.URL.RequestURI()
	_, err = fmt.Fprintf(wtr, "%s %s HTTP/1.1\r\n", req.Method, ruri)
	return err
}

type Client struct {
	Timeout time.Duration
}

func (c *Client) deadline() time.Time {
	if c.Timeout > 0 {
		return time.Now().Add(c.Timeout)
	}

	return time.Time{}
}

var DefaultClient Client = Client{}

func (c *Client) Do(req *parser.Request) (*parser.Response, error) {
	if req.URL == nil {
		return nil, errors.New("http: nil URL")
	}
	var (
		deadline       = c.deadline()
		reqs           []*parser.Request
		res            *parser.Response
		includeBody    bool
		redirectMethod string
	)

	for {
		// All requests but first
		if len(reqs) > 0 {
			loc := res.Headers.Get("Location")
			if loc == "" {
				return res, nil
			}

			url, err := req.URL.Parse(loc)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse location header %q: %w", loc, err)
			}
			host := ""
			if req.Host != "" && req.Host != req.URL.Host {
				if u, _ := url.Parse(loc); u != nil && !u.IsAbs() {
					host = req.Host
				}
			}

			if url.Scheme == "https" {
				return nil, fmt.Errorf("Cannot make request to HTTPS host, only HTTP is supported")
			}

			initialReq := reqs[0]
			req = &parser.Request{
				Method:  redirectMethod,
				URL:     url,
				Headers: make(parser.Headers),
				Host:    host,
			}
			if includeBody && initialReq.ReadBody != nil {
				req.Body, err = initialReq.ReadBody()
				if err != nil {
					return nil, err
				}
			}
			req.ContentLength = initialReq.ContentLength
			copyHeaders(initialReq, req, false) // TODO: strip sensitive if going to another domain

			if len(reqs) > 10 {
				// TODO: cycle detection
				return nil, errors.New("Stopped after 10 redirects")
			}
		}

		var cancel context.CancelFunc
		ctx := context.Background()
		if c.Timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, c.Timeout)
			defer cancel()
		}

		reqs = append(reqs, req)
		var err error
		var didTimeout func() bool
		if res, didTimeout, err = send(req, deadline); err != nil {
			if !deadline.IsZero() && didTimeout() {
				err = fmt.Errorf("%w (Timeout exceeded while waiting for headers)", err)
			}
			return nil, err
		}

		var (
			shouldRedirect   bool
			includeBodyOnHop bool
		)
		redirectMethod, shouldRedirect, includeBodyOnHop = redirectBehavior(req.Method, res, reqs[0])
		fmt.Printf("Should redirect: %t, method: %s, body: %t\n", shouldRedirect, redirectMethod, includeBodyOnHop)
		if !shouldRedirect {
			return res, nil
		}

		if !includeBodyOnHop {
			includeBody = false
		}
	}
}

func copyHeaders(initialReq *parser.Request, req *parser.Request, stripSensitive bool) {
	for k, v := range initialReq.Headers {
		sensitive := false
		switch http.CanonicalHeaderKey(k) {
		case "Authorization", "Www-Authenticate", "Cookie", "Cookie2":
			sensitive = true
		}
		if !(sensitive && stripSensitive) {
			req.Headers[k] = v
		}
	}
}

func nop()              {}
func alwaysFalse() bool { return false }

func setRequestCancel(req *parser.Request, deadline time.Time) (func(), func() bool) {
	if deadline.IsZero() {
		return nop, alwaysFalse
	}

	oldCtx := req.Context()
	var cancelCtx func()
	req.Ctx, cancelCtx = context.WithDeadline(oldCtx, deadline)
	cancel := make(chan struct{})
	req.Cancel = cancel

	doCancel := func() {
		close(cancel)
	}

	stopTimerCh := make(chan struct{})
	stopTimer := sync.OnceFunc(func() {
		close(stopTimerCh)
		if cancelCtx != nil {
			cancelCtx()
		}
	})

	timer := time.NewTimer(time.Until(deadline))
	var didTimeout atomic.Bool

	go func() {
		select {
		case <-timer.C:
			didTimeout.Store(true)
			doCancel()
		case <-stopTimerCh:
			timer.Stop()
		}
	}()

	return stopTimer, didTimeout.Load
}

var portMap = map[string]string{
	"http":    "80",
	"https":   "443",
	"socks5":  "1080",
	"socks5h": "1080",
}

func isAscii(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func idnaASCII(v string) (string, error) {
	if isAscii(v) {
		return v, nil
	}
	return idna.Lookup.ToASCII(v)
}

func idnaASCIIFromURL(url *url.URL) string {
	addr := url.Hostname()
	if v, err := idnaASCII(addr); err == nil {
		addr = v
	}
	return addr
}

func canonicalAddr(url *url.URL) string {
	port := url.Port()
	if port == "" {
		port = portMap[url.Scheme]
	}
	return net.JoinHostPort(idnaASCIIFromURL(url), port)
}

func send(req *parser.Request, deadline time.Time) (*parser.Response, func() bool, error) {
	stopTimer, didTimeout := setRequestCancel(req, deadline)
	sock, err := net.Dial("tcp", canonicalAddr(req.URL))
	if err != nil {
		return nil, alwaysFalse, err
	}

	bw := bufio.NewWriter(sock)
	err = writeRequestLine(bw, req)
	if err != nil {
		return nil, alwaysFalse, err
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

	host := req.URL.Host
	_, err = fmt.Fprintf(bw, "Host: %s\r\n", host)
	if err != nil {
		return nil, alwaysFalse, err
	}

	_, err = writeHeaders(req, bw, int(req.ContentLength), req.Headers)
	if err != nil {
		return nil, alwaysFalse, err
	}

	nr, err := io.Copy(bw, io.LimitReader(req.Body, req.ContentLength))
	if err != nil {
		return nil, alwaysFalse, err
	}
	if nr < req.ContentLength {
		return nil, alwaysFalse, errors.New("http: Could not write whole body")
	}

	err = bw.Flush()

	// TODO: Read response while writing request in case Server responds before we finish.
	rdr := bufio.NewReader(sock)
	res, err := parser.ParseResponse(rdr)
	if err != nil {
		stopTimer()
		return nil, didTimeout, err
	}

	if res.Body == nil {
		res.Body = strings.NewReader("")
	}

	if !deadline.IsZero() {
		res.Body = &cancelTimerBody{
			stop:          stopTimer,
			rc:            io.NopCloser(res.Body),
			reqDidTimeout: didTimeout,
		}
	}

	return res, nil, nil
}

func redirectBehavior(reqMethod string, res *parser.Response) (redirectMethod string, shouldRedirect bool, includeBody bool) {
	switch res.StatusCode {
	case status.MovedPermanently, status.Found, status.SeeOther:
		redirectMethod = reqMethod
		shouldRedirect = true
		includeBody = false
		// RFC 2616: Section 10.3
		// The action required MAY be carried out by the user agent without interaction
		// with the user if and only if the method used in the second request is
		// GET or HEAD.
		if reqMethod != "GET" && reqMethod != "HEAD" {
			redirectMethod = "GET"
		}
	case status.TemporaryRedirect, status.PermanentRedirect:
		redirectMethod = reqMethod
		shouldRedirect = true
		includeBody = true
	}

	return redirectMethod, shouldRedirect, includeBody
}

type cancelTimerBody struct {
	stop          func()
	rc            io.ReadCloser
	reqDidTimeout func() bool
}

func (b *cancelTimerBody) Read(p []byte) (n int, err error) {
	n, err = b.rc.Read(p)
	if err == nil {
		return n, nil
	}

	if err == io.EOF {
		return n, err
	}

	if b.reqDidTimeout() {
		err = fmt.Errorf("%w (Request timeout or cancellation while reading body)", err)
	}

	return n, err
}

func (b *cancelTimerBody) Close() error {
	err := b.rc.Close()
	b.stop()
	return err
}
