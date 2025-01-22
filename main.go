package main

import (
	"bufio"
	"fmt"
	"log"
	"net"

	"github.com/arthur-teixeira/go-http/parser"
)

func main() {
	listenAndServe(":8080")
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
	defer conn.Close()
	b := bufio.NewReader(conn)
	req, err := parser.ParseRequest(b)
	if err != nil {
		log.Println("Error parsing request: ", err)
	}

	fmt.Println("Got request: ", req.Headers)
	body := make([]byte, 512)
	nb, err := req.Body.Read(body)
	body = body[:nb]
	if err != nil {
		log.Println("Error reading request body: ", err)
	}

	fmt.Println("Request body: ", string(body))
}
