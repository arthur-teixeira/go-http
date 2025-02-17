package transport

import (
	"container/list"
	"net"
	"sync"
	"time"
)

type conn struct {
	mu         sync.Mutex // Protects the underlying connection
	sock       net.Conn
	targetAddr string
	idleMu     sync.Mutex // Protects the idle field
	idle       bool
	idleSince  time.Time
}

// Caller should hold idleMu
func (c *conn) Close() {
	c.sock.Close()
}

type ConnectionManager struct {
	MaxConnections             int // Max connections overall
	TotalConnections           int // Connection count for all hosts
	MaxSimultaneousConnections int // Max connections for same host
	IdleTimeout                time.Duration
	ConnectionMu               sync.RWMutex          // Blocks connections for adding/deleting
	Connections                map[string]*list.List // List of *conn
}

var Manager = ConnectionManager{
	MaxConnections:             100,
	MaxSimultaneousConnections: 10,
	IdleTimeout:                90 * time.Second,
	Connections:                map[string]*list.List{},
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

func (m *ConnectionManager) dial(host string) (*conn, error) {
	netConn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	return &conn{
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
			conn.idleMu.Unlock()
			conn.idle = false
			return conn
		}
		conn.idleMu.Unlock()
	}

	return nil
}

func (m *ConnectionManager) GetConnection(host string) (*conn, error) {
	m.ConnectionMu.RLock()
	conns := m.Connections[host]
	m.ConnectionMu.RUnlock()

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
