package transport

import (
	"container/list"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Reusable interface {
	io.ReadCloser
	Release()
}

type conn struct {
	id         string     // For testing purposes
	mu         sync.Mutex // Protects the underlying connection
	sock       net.Conn
	targetAddr string
	idleMu     sync.Mutex // Protects the idle field
	idle       bool
	idleSince  time.Time
	closed     bool
}

func (c *conn) Close() error {
	fmt.Println("Closed connection ", c.id)
	c.closed = true
	return c.sock.Close()
}

func (c *conn) Release() {
	fmt.Println("Connection ", c.id, " Released")
	c.idleMu.Lock()
	c.idle = true
	c.idleSince = time.Now()
	c.idleMu.Unlock()
}

func (c *conn) Read(b []byte) (int, error) {
	return c.sock.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	return c.sock.Write(b)
}

type ConnectionManager struct {
	MaxConnections        int // Max connections overall
	TotalConnections      int // Connection count for all hosts
	MaxConnectionsPerHost int
	IdleTimeout           time.Duration
	ConnectionMu          sync.RWMutex          // Blocks connections for adding/deleting
	Connections           map[string]*list.List // List of *conn
}

var Manager = ConnectionManager{
	MaxConnections:        100,
	MaxConnectionsPerHost: 10,
	IdleTimeout:           90 * time.Second,
	Connections:           map[string]*list.List{},
}

func (m *ConnectionManager) ClearIdleConnections() {
	m.ConnectionMu.Lock()
	defer m.ConnectionMu.Unlock()

	for _, conns := range m.Connections {
		collect := []*list.Element{}
		for node := conns.Front(); node != nil; node = node.Next() {
			conn, ok := node.Value.(*conn)
			if !ok {
				panic("Item in connection manager is not a valid connection")
			}
			if conn.closed {
				collect = append(collect, node)
				continue
			}

			conn.idleMu.Lock()
			if conn.idle && time.Since(conn.idleSince) >= m.IdleTimeout {
				conn.Close()
				collect = append(collect, node)
			}
			conn.idleMu.Unlock()
		}
		for _, n := range collect {
			conns.Remove(n)
		}
	}
}

func randId() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func (m *ConnectionManager) dial(host string) (*conn, error) {
	fmt.Println("Dialing host")
	netConn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	return &conn{
		id:         randId(),
		targetAddr: host,
		sock:       netConn,
		idle:       false,
	}, nil
}

func (m *ConnectionManager) findConnection(conns *list.List) *conn {
	for node := conns.Front(); node != nil; node = node.Next() {
		conn, ok := node.Value.(*conn)
		if !ok {
			panic("Item in connection manager is not a valid connection")
		}

		conn.idleMu.Lock()
		if conn.idle {
      fmt.Printf("Found idle connection %s, reusing\n", conn.id)
			conn.idle = false
			conn.idleMu.Unlock()
			return conn
		}
		conn.idleMu.Unlock()
	}

	return nil
}

func (m *ConnectionManager) GetConnection(host string) (*conn, error) {
	m.ConnectionMu.Lock()
	conns := m.Connections[host]
	if conns == nil {
		conns = list.New()
		m.Connections[host] = conns
	}
	m.ConnectionMu.Unlock()

	conn := m.findConnection(conns)
	if conn != nil {
		return conn, nil
	}

	// TODO: if MaxConnections is reached, wait until another connection is dropped

	// TODO: if MaxSimultaneousConnections is reached for this host,
	// wait until an existing connection is released or closed.
	// If closed, create new one. If released, reuse.

	m.ConnectionMu.Lock()
	defer m.ConnectionMu.Unlock()

	// Try to find connection creating after releasing read lock
	conn = m.findConnection(conns)
	if conn != nil {
		return conn, nil
	}

	// No available connection, creating new one
	newConn, err := m.dial(host)
	if err != nil {
		return nil, err
	}
	m.TotalConnections += 1
	m.Connections[host].PushFront(newConn)
	return newConn, nil
}
