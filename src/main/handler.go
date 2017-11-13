package main

import (
	"log"
	"net"
	"runtime/debug"
)

type MessageHandler interface {
	Encode(msg interface{}) []byte
	Decode(buf []byte) (interface{}, int)
	MessageReceived(connHandler *ConnHandler, msg interface{})
	ConnSuccess(connHandler *ConnHandler)
	ConnError(connHandler *ConnHandler)
}

type ConnHandler struct {
	Active         bool
	NextConn       *ConnHandler
	conn           net.Conn
	readBuf        []byte
	messageHandler MessageHandler
}

func (connHandler *ConnHandler) Listen(conn net.Conn, messageHandler interface{}) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run time panic: %v", err)
			debug.PrintStack()
			connHandler.messageHandler.ConnError(connHandler)
		}
	}()
	if conn == nil {
		return
	}
	connHandler.conn = conn
	connHandler.messageHandler = messageHandler.(MessageHandler)
	connHandler.Active = true
	connHandler.messageHandler.ConnSuccess(connHandler)
	for {
		buf := make([]byte, 1024*8)
		// 一个数据包大小不能超过2M
		if connHandler.readBuf != nil && len(connHandler.readBuf) > 1024*1024*2 {
			connHandler.conn.Close()
		}
		n, err := connHandler.conn.Read(buf)
		if err != nil || n == 0 {
			connHandler.Active = false
			connHandler.messageHandler.ConnError(connHandler)
			break
		}
		if connHandler.readBuf == nil {
			connHandler.readBuf = buf[0:n]
		} else {
			connHandler.readBuf = append(connHandler.readBuf, buf[0:n]...)
		}

		for {
			msg, n := connHandler.messageHandler.Decode(connHandler.readBuf)
			if msg == nil {
				break
			}

			connHandler.messageHandler.MessageReceived(connHandler, msg)
			connHandler.readBuf = connHandler.readBuf[n:]
			if len(connHandler.readBuf) == 0 {
				break
			}
		}

		if len(connHandler.readBuf) > 0 {
			buf := make([]byte, len(connHandler.readBuf))
			copy(buf, connHandler.readBuf)
			connHandler.readBuf = buf
		}
	}
}

func (connHandler *ConnHandler) Write(msg interface{}) {
	buf := connHandler.messageHandler.Encode(msg)
	connHandler.conn.Write(buf)
}
