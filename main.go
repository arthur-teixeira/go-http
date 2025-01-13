package main

import (
	"fmt"
	"log"
	"net"
	"strings"
)

func main() {
  err := connectto("www.google.com:80")
  if err != nil {
    log.Fatal(err)
  }
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
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)
	nb, err := conn.Read(buf)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Printf("Received %s\n", buf[:nb])
}

func connectto(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte("GET /\r\nHTTP/1.1\r\n\r\n"))
	if err != nil {
		return err
	}

  nb := 0
  first := true
  all := make([]byte, 100000)
  for nb > 0 || first {
    first = false
    buf := make([]byte, 2048)
    nb, err = conn.Read(buf)
    if err != nil {
      return err
    }
    all = append(all, buf...)
    if strings.Contains(string(buf), "\r\n\r\n"){
      break
    }
  }

	fmt.Printf("Got response: %s\nwith %d bytes\n", all, nb)

	return nil
}
