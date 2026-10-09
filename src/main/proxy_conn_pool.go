package main

import (
	"log"
	"sync"
)

type Pooler interface {
	Create(pool *ConnHandlerPool) (*ConnHandler, error)
	Remove(conn *ConnHandler)
	IsActive(conn *ConnHandler) bool
}

type ConnHandlerPool struct {
	Size   int
	Pooler Pooler
	mu     sync.Mutex
	conns  []*ConnHandler
}

func (connPool *ConnHandlerPool) Init() {
	connPool.conns = make([]*ConnHandler, 0, connPool.Size)
	log.Printf("init connection pool, len %d, cap %d", len(connPool.conns), cap(connPool.conns))
}

func (connPool *ConnHandlerPool) Get() (*ConnHandler, error) {
	for {
		if len(connPool.conns) == 0 {
			conn, err := connPool.Pooler.Create(connPool)
			log.Println("create connection: ", conn, err)
			if err != nil {
				return nil, err
			}

			return conn, nil
		} else {
			conn, err := connPool.getConn()
			if conn != nil {
				return conn, err
			}
		}
	}
}

func (connPool *ConnHandlerPool) getConn() (*ConnHandler, error) {
	connPool.mu.Lock()
	defer connPool.mu.Unlock()
	if len(connPool.conns) == 0 {
		return nil, nil
	}
	conn := connPool.conns[len(connPool.conns)-1]
	connPool.conns = connPool.conns[:len(connPool.conns)-1]
	if connPool.Pooler.IsActive(conn) {
		log.Println("get connection from pool: ", conn)
		return conn, nil
	} else {
		// Clean up inactive connection to prevent resource leaks
		log.Println("cleaning up inactive connection: ", conn)
		connPool.Pooler.Remove(conn)
		return nil, nil
	}
}

func (connPool *ConnHandlerPool) Return(conn *ConnHandler) {
	connPool.mu.Lock()
	defer connPool.mu.Unlock()
	if len(connPool.conns) >= connPool.Size {
		log.Println("pool is full, remove connection: ", conn)
		connPool.Pooler.Remove(conn)
	} else {
		// Ensure connection is still active before returning to pool
		if connPool.Pooler.IsActive(conn) {
			connPool.conns = append(connPool.conns, conn)
			log.Println("return connection:", conn, ", poolsize is ", len(connPool.conns))
		} else {
			log.Println("connection is no longer active, removing: ", conn)
			connPool.Pooler.Remove(conn)
		}
	}
}

// Close gracefully closes all connections in the pool
func (connPool *ConnHandlerPool) Close() {
	connPool.mu.Lock()
	defer connPool.mu.Unlock()

	log.Printf("Closing connection pool with %d connections", len(connPool.conns))
	for _, conn := range connPool.conns {
		if conn != nil {
			connPool.Pooler.Remove(conn)
		}
	}
	connPool.conns = nil
}
