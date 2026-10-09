package main

import (
	"log"
	"net"
	"runtime/debug"
	"sync"
	"time"
)

type MessageHandler interface {
	Encode(msg interface{}) []byte
	Decode(buf []byte) (interface{}, int)
	MessageReceived(connHandler *ConnHandler, msg interface{})
	ConnSuccess(connHandler *ConnHandler)
	ConnError(connHandler *ConnHandler)
}

type ConnHandler struct {
	ReadTime       int64
	WriteTime      int64
	Active         bool
	NextConn       *ConnHandler
	conn           net.Conn
	readBuf        []byte
	messageHandler MessageHandler
	mu             sync.Mutex
}

var (
	bufPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024*8)
		},
	}
)

func (connHandler *ConnHandler) Listen(conn net.Conn, messageHandler interface{}) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run time panic: %v", err)
			debug.PrintStack()
			if connHandler.messageHandler != nil {
				connHandler.messageHandler.ConnError(connHandler)
			}
		}
		// Ensure connection cleanup on exit
		connHandler.cleanup()
	}()
	if conn == nil {
		return
	}
	connHandler.conn = conn
	var ok bool
	connHandler.messageHandler, ok = messageHandler.(MessageHandler)
	if !ok {
		log.Printf("Error: invalid MessageHandler type")
		return
	}
	connHandler.Active = true
	connHandler.ReadTime = time.Now().Unix()
	connHandler.WriteTime = connHandler.ReadTime
	connHandler.messageHandler.ConnSuccess(connHandler)
	for {
		// Get buffer from pool to reduce allocations
		buf := bufPool.Get().([]byte)
		defer bufPool.Put(buf)

		// 一个数据包大小不能超过2M
		connHandler.mu.Lock()
		if connHandler.readBuf != nil && len(connHandler.readBuf) > 1024*1024*2 {
			connHandler.mu.Unlock()
			log.Printf("Buffer overflow detected, closing connection")
			connHandler.conn.Close()
			break
		}
		connHandler.mu.Unlock()

		n, err := connHandler.conn.Read(buf)
		if err != nil || n == 0 {
			connHandler.Active = false
			if connHandler.messageHandler != nil {
				connHandler.messageHandler.ConnError(connHandler)
			}
			break
		}
		connHandler.ReadTime = time.Now().Unix()

		// Efficient buffer management
		connHandler.mu.Lock()
		if connHandler.readBuf == nil {
			// Make a copy of the data since we're returning buf to the pool
			newBuf := make([]byte, n)
			copy(newBuf, buf[:n])
			connHandler.readBuf = newBuf
		} else {
			connHandler.readBuf = append(connHandler.readBuf, buf[:n]...)
		}
		connHandler.mu.Unlock()

		for {
			connHandler.mu.Lock()
			if connHandler.readBuf == nil || len(connHandler.readBuf) == 0 {
				connHandler.mu.Unlock()
				break
			}

			msg, n := connHandler.messageHandler.Decode(connHandler.readBuf)
			if msg == nil || n <= 0 {
				connHandler.mu.Unlock()
				break
			}

			// Create a copy of the remaining data to avoid slice issues
			remaining := make([]byte, len(connHandler.readBuf)-n)
			copy(remaining, connHandler.readBuf[n:])
			connHandler.mu.Unlock()

			connHandler.messageHandler.MessageReceived(connHandler, msg)

			connHandler.mu.Lock()
			connHandler.readBuf = remaining
			connHandler.mu.Unlock()
		}

		// Clean up empty buffers to free memory
		connHandler.mu.Lock()
		if len(connHandler.readBuf) > 0 {
			// Compact buffer if it's mostly empty
			if cap(connHandler.readBuf) > len(connHandler.readBuf)*4 {
				newBuf := make([]byte, len(connHandler.readBuf))
				copy(newBuf, connHandler.readBuf)
				connHandler.readBuf = newBuf
			}
		} else {
			connHandler.readBuf = nil
		}
		connHandler.mu.Unlock()
	}
}

// cleanup safely closes and cleans up connection resources
func (connHandler *ConnHandler) cleanup() {
	connHandler.Active = false
	if connHandler.conn != nil {
		connHandler.conn.Close()
		connHandler.conn = nil
	}
	connHandler.readBuf = nil
	connHandler.messageHandler = nil
}

func (connHandler *ConnHandler) Write(msg interface{}) {
	if connHandler.messageHandler != nil && connHandler.Active {
		buf := connHandler.messageHandler.Encode(msg)
		if len(buf) > 0 {
			connHandler.WriteTime = time.Now().Unix()
			if connHandler.conn != nil {
				n, err := connHandler.conn.Write(buf)
				if err != nil {
					log.Printf("Write error: %v, expected %d bytes, wrote %d", err, len(buf), n)
					connHandler.Active = false
					return
				}
				if n != len(buf) {
					log.Printf("Incomplete write: expected %d bytes, wrote %d", len(buf), n)
					connHandler.Active = false
					return
				}
			}
		}
	}
}
