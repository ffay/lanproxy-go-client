package main

import (
	"crypto/tls"
	"log"
	"net"
)

type RealServerMessageHandler struct {
	LpConnHandler *ConnHandler
	ConnPool      *ConnHandlerPool
	UserId        string
	ClientKey     string
	ServerAddr    string
	TLSConfig     *tls.Config
}

func (messageHandler *RealServerMessageHandler) Encode(msg interface{}) []byte {
	if msg == nil {
		return []byte{}
	}

	return msg.([]byte)
}

func (messageHandler *RealServerMessageHandler) Decode(buf []byte) (interface{}, int) {
	return buf, len(buf)
}

func (messageHandler *RealServerMessageHandler) MessageReceived(connHandler *ConnHandler, msg interface{}) {
	if connHandler.NextConn != nil {
		data := msg.([]byte)
		message := Message{Type: P_TYPE_TRANSFER}
		message.Data = data
		connHandler.NextConn.Write(message)
	}
}

func createDirectConnection(userId, clientKey, serverAddr string, tlsConfig *tls.Config) (*ConnHandler, error) {
	var conn net.Conn
	var err error

	if tlsConfig != nil {
		conn, err = tls.Dial("tcp", serverAddr, tlsConfig)
	} else {
		conn, err = net.Dial("tcp", serverAddr)
	}

	if err != nil {
		log.Println("Error creating direct connection:", err.Error())
		return nil, err
	}

	// Create a simple connection handler without connection pool
	connHandler := &ConnHandler{}
	connHandler.Active = true
	connHandler.conn = conn

	// Use the same logic as ProxyConnPooler.Create but without pool
	messageHandler := LPMessageHandler{connPool: nil}
	connHandler.messageHandler = &messageHandler
	messageHandler.connHandler = connHandler

	// Start heartbeat in a separate goroutine
	messageHandler.startHeartbeat()

	// Start listening for messages
	go func() {
		connHandler.Listen(conn, &messageHandler)
	}()

	return connHandler, nil
}

func (messageHandler *RealServerMessageHandler) ConnSuccess(connHandler *ConnHandler) {
	log.Println("get proxy connection:", messageHandler.UserId)
	var proxyConnHandler *ConnHandler
	var err error

	if messageHandler.ConnPool != nil {
		// Use connection pool
		proxyConnHandler, err = messageHandler.ConnPool.Get()
		if err != nil {
			log.Println("get proxy connection err:", err, "uri:", messageHandler.UserId)
			message := Message{Type: TYPE_DISCONNECT}
			message.Uri = messageHandler.UserId
			messageHandler.LpConnHandler.Write(message)
			connHandler.conn.Close()
			return
		}
	} else {
		// Create direct connection (no connection pool)
		proxyConnHandler, err = createDirectConnection(messageHandler.UserId, messageHandler.ClientKey, messageHandler.ServerAddr, messageHandler.TLSConfig)
		if err != nil {
			log.Println("create direct connection err:", err, "uri:", messageHandler.UserId)
			message := Message{Type: TYPE_DISCONNECT}
			message.Uri = messageHandler.UserId
			messageHandler.LpConnHandler.Write(message)
			connHandler.conn.Close()
			return
		}
	}

	proxyConnHandler.NextConn = connHandler
	connHandler.NextConn = proxyConnHandler
	message := Message{Type: TYPE_CONNECT}
	message.Uri = messageHandler.UserId + "@" + messageHandler.ClientKey
	proxyConnHandler.Write(message)
	log.Println("realserver connect success, notify proxyserver:", message.Uri)
}

func (messageHandler *RealServerMessageHandler) ConnError(connHandler *ConnHandler) {
	conn := connHandler.NextConn
	if conn != nil {
		message := Message{Type: TYPE_DISCONNECT}
		message.Uri = messageHandler.UserId
		conn.Write(message)
		conn.NextConn = nil
	}

	connHandler.messageHandler = nil
}

func (messageHandler *RealServerMessageHandler) ConnFailed() {
	message := Message{Type: TYPE_DISCONNECT}
	message.Uri = messageHandler.UserId
	messageHandler.LpConnHandler.Write(message)
}
