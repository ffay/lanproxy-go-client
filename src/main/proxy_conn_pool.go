package main

import (
	"log"
	"sync"
	"time"
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
	close(conn.HbChan)
	connPool.conns = connPool.conns[:len(connPool.conns)-1]
	if connPool.Pooler.IsActive(conn) {
		log.Println("get connection from pool: ", conn)
		return conn, nil
	} else {
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
		connPool.conns = connPool.conns[:len(connPool.conns)+1]
		connPool.conns[len(connPool.conns)-1] = conn
		log.Println("return connection:", conn, ", poolsize is ", len(connPool.conns))
		connPool.startHeartbeat(conn)
	}
}

func (connPool *ConnHandlerPool) startHeartbeat(conn *ConnHandler) {
	log.Println("start proxy connection heartbeat:", conn)
	if time.Now().Unix()-conn.WriteTime > HEARTBEAT_INTERVAL {
		msg := Message{Type: TYPE_HEARTBEAT}
		conn.Write(msg)
	}
	conn.HbChan = make(chan struct{})
	go func() {
		for {
			select {
			case <-time.After(time.Second * HEARTBEAT_INTERVAL):
				if time.Now().Unix()-conn.ReadTime >= 2*HEARTBEAT_INTERVAL {
					log.Println("proxy connection timeout:", conn)
					conn.conn.Close()
					return
				}
				msg := Message{Type: TYPE_HEARTBEAT}
				conn.Write(msg)
			case <-conn.HbChan:
				log.Println("stop proxy connection heartbeat:", conn)
				return
			}
		}
	}()
}
