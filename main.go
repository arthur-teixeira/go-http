package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	sock, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
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
