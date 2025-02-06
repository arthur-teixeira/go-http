package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"

	"github.com/arthur-teixeira/go-http/context"
	"github.com/arthur-teixeira/go-http/parser"
	"github.com/arthur-teixeira/go-http/status"
)

func main() {
	// listenAndServe(":8080")

	// res, err :=  http.DefaultClient.Get("http://yahoo.com")

	url, err := url.ParseRequestURI("http://google.com")
	if err != nil {
		log.Fatal(err)
	}

	res, err := context.DefaultClient.Do(&parser.Request{
		URL:     url,
		Method:  "GET",
		Version: "HTTP/1.1",
		Major:   1,
		Minor:   1,
	})

	if err != nil {
		log.Fatal("Error on request", err)
	}

	fmt.Printf("Response: %#v\n", res)

	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)
	fmt.Printf("Response body: %s\n", buf.String())
}

func listenAndServe(addr string) error {
	sock, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for {
		conn, err := sock.Accept()
		if err != nil {
			log.Println("Error accepting connection: ", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	// TODO: Create timeout for persistent connections
	for {
		context, err := context.NewContext(conn)
		if err != nil {
			if err == io.EOF {
				continue
			}
			log.Println("Error building context: ", err)
		}

		handleRequest(context)
		if context.Request.Close {
			conn.Close()
			break
		}
	}
}

func handleRequest(c *context.Context) {
	req := c.Request
	fmt.Println("Got request: ", req.Headers)
	body := make([]byte, 512)
	nb, err := req.Body.Read(body)
	body = body[:nb]
	if err != nil {
		log.Println("Error reading request body: ", err)
	}

	fmt.Println("Request body: ", string(body))
	c.WriteHeader(status.OK)
	res, _ := json.Marshal(map[string]string{
		"Hello": "World!",
	})
	c.Write(res)
}
