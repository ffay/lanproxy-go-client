package main

import (
	"log"
)

type RealServerMessageHandler struct {
	LpConnHandler *ConnHandler
	ConnPool      *ConnHandlerPool
	UserId        string
	ClientKey     string
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

func (messageHandler *RealServerMessageHandler) ConnSuccess(connHandler *ConnHandler) {
	proxyConnHandler, err := messageHandler.ConnPool.Get()
	if err != nil {
		log.Println(err)
		message := Message{Type: TYPE_DISCONNECT}
		message.Uri = messageHandler.UserId
		messageHandler.LpConnHandler.Write(message)
		connHandler.conn.Close()
	} else {
		proxyConnHandler.NextConn = connHandler
		connHandler.NextConn = proxyConnHandler
		message := Message{Type: TYPE_CONNECT}
		message.Uri = messageHandler.UserId + "@" + messageHandler.ClientKey
		proxyConnHandler.Write(message)
		log.Println("realserver connect success, notify proxyserver:", message.Uri)
	}
}

func (messageHandler *RealServerMessageHandler) ConnError(connHandler *ConnHandler) {
	conn := connHandler.NextConn
	if conn != nil {
		message := Message{Type: TYPE_DISCONNECT}
		message.Uri = messageHandler.UserId
		conn.Write(message)
		conn.NextConn = nil
	}
}

func (messageHandler *RealServerMessageHandler) ConnFailed() {
	message := Message{Type: TYPE_DISCONNECT}
	message.Uri = messageHandler.UserId
	messageHandler.LpConnHandler.Write(message)
}
