package main

import (
	"encoding/binary"
	"github.com/urfave/cli"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	/* 心跳消息 */
	TYPE_HEARTBEAT = 0x07

	/* 认证消息，检测clientKey是否正确 */
	C_TYPE_AUTH = 0x01

	/* 代理后端服务器建立连接消息 */
	TYPE_CONNECT = 0x03

	/* 代理后端服务器断开连接消息 */
	TYPE_DISCONNECT = 0x04

	/* 代理数据传输 */
	P_TYPE_TRANSFER = 0x05

	/* 用户与代理服务器以及代理客户端与真实服务器连接是否可写状态同步 */
	C_TYPE_WRITE_CONTROL = 0x06

	//协议各字段长度
	LEN_SIZE = 4

	TYPE_SIZE = 1

	SERIAL_NUMBER_SIZE = 8

	URI_LENGTH_SIZE = 1
)

type LPMessageHandler struct {
	connPool    *ConnHandlerPool
	connHandler *ConnHandler
	clientKey   string
	die         chan struct{}
}

type Message struct {
	Type         byte
	SerialNumber uint64
	Uri          string
	Data         []byte
}

type ProxyConnPooler struct {
	addr string
}

func main() {
	log.Println("lanproxy - help you expose a local server behind a NAT or firewall to the internet")
	app := cli.NewApp()
	app.Name = "lanproxy"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "k",
			Value: "",
			Usage: "client key",
		},
		cli.StringFlag{
			Name:  "s",
			Value: "127.0.0.1",
			Usage: "proxy server host",
		},
		cli.IntFlag{
			Name:  "p",
			Value: 4900,
			Usage: "proxy server port",
		}}
	app.Usage = "help you expose a local server behind a NAT or firewall to the internet"
	app.Action = func(c *cli.Context) error {
		log.Println("server addr:", c.String("s"))
		log.Println("server port:", c.Int("p"))
		start(c.String("k"), c.String("s"), c.Int("p"))
		return nil
	}

	app.Run(os.Args)
}

func start(key string, ip string, port int) {
	connPool := &ConnHandlerPool{Size: 100, Pooler: &ProxyConnPooler{addr: ip + ":" + strconv.Itoa(port)}}
	connPool.Init()
	connHandler := &ConnHandler{}
	for {
		//cmd connection
		conn := connect(key, ip, port)
		connHandler.conn = conn
		messageHandler := LPMessageHandler{connPool: connPool}
		messageHandler.connHandler = connHandler
		messageHandler.clientKey = key
		messageHandler.startHeartbeat()
		log.Println("start listen cmd message:", messageHandler)
		connHandler.Listen(conn, &messageHandler)
	}
}

func connect(key string, ip string, port int) net.Conn {
	for {
		p := strconv.Itoa(port)
		conn, err := net.Dial("tcp", ip+":"+p)
		if err != nil {
			log.Println("Error dialing", err.Error())
			time.Sleep(time.Second * 3)
			continue
		}

		return conn
	}
}

func (messageHandler *LPMessageHandler) Encode(msg interface{}) []byte {
	if msg == nil {
		return []byte{}
	}

	message := msg.(Message)
	uriBytes := []byte(message.Uri)
	bodyLen := TYPE_SIZE + SERIAL_NUMBER_SIZE + URI_LENGTH_SIZE + len(uriBytes) + len(message.Data)
	data := make([]byte, LEN_SIZE, bodyLen+LEN_SIZE)
	binary.BigEndian.PutUint32(data, uint32(bodyLen))
	data = append(data, message.Type)
	snBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(snBytes, message.SerialNumber)
	data = append(data, snBytes...)
	data = append(data, byte(len(uriBytes)))
	data = append(data, uriBytes...)
	data = append(data, message.Data...)
	return data
}

func (messageHandler *LPMessageHandler) Decode(buf []byte) (interface{}, int) {
	lenBytes := buf[0:LEN_SIZE]
	bodyLen := binary.BigEndian.Uint32(lenBytes)
	if uint32(len(buf)) < bodyLen+LEN_SIZE {
		return nil, 0
	}
	n := int(bodyLen + LEN_SIZE)
	body := buf[LEN_SIZE:n]
	msg := Message{}
	msg.Type = body[0]
	msg.SerialNumber = binary.BigEndian.Uint64(body[TYPE_SIZE : SERIAL_NUMBER_SIZE+TYPE_SIZE])
	uriLen := uint8(body[SERIAL_NUMBER_SIZE+TYPE_SIZE])
	msg.Uri = string(body[SERIAL_NUMBER_SIZE+TYPE_SIZE+URI_LENGTH_SIZE : SERIAL_NUMBER_SIZE+TYPE_SIZE+URI_LENGTH_SIZE+uriLen])
	msg.Data = body[SERIAL_NUMBER_SIZE+TYPE_SIZE+URI_LENGTH_SIZE+uriLen:]
	return msg, n
}

func (messageHandler *LPMessageHandler) MessageReceived(connHandler *ConnHandler, msg interface{}) {
	message := msg.(Message)
	switch message.Type {
	case TYPE_CONNECT:
		go func() {
			log.Println("received connect message:", message.Uri, "=>", string(message.Data))
			addr := string(message.Data)
			realServerMessageHandler := &RealServerMessageHandler{LpConnHandler: connHandler, ConnPool: messageHandler.connPool, UserId: message.Uri, ClientKey: messageHandler.clientKey}
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				realServerMessageHandler.ConnFailed()
			} else {
				connHandler := &ConnHandler{}
				connHandler.conn = conn
				connHandler.Listen(conn, realServerMessageHandler)
			}
		}()
	case P_TYPE_TRANSFER:
		if connHandler.NextConn != nil {
			connHandler.NextConn.Write(message.Data)
		}
	case TYPE_DISCONNECT:
		if connHandler.NextConn != nil {
			connHandler.NextConn.conn.Close()
		}
	}
}

func (messageHandler *LPMessageHandler) ConnSuccess(connHandler *ConnHandler) {
	log.Println("connSuccess, clientkey:", messageHandler.clientKey)
	if messageHandler.clientKey != "" {
		msg := Message{Type: C_TYPE_AUTH}
		msg.Uri = messageHandler.clientKey
		connHandler.Write(msg)
	}
}

func (messageHandler *LPMessageHandler) ConnError(connHandler *ConnHandler) {
	log.Println("connError:", connHandler)
	close(messageHandler.die)
	time.Sleep(time.Second * 3)
}

func (messageHandler *LPMessageHandler) startHeartbeat() {
	log.Println("start heartbeat:", messageHandler.connHandler)
	messageHandler.die = make(chan struct{})
	go func() {
		for {
			select {
			case <-time.After(time.Second * 5):
				msg := Message{Type: TYPE_HEARTBEAT}
				messageHandler.connHandler.Write(msg)
			case <-messageHandler.die:
				return
			}
		}
	}()
}

func (pooler *ProxyConnPooler) Create() (*ConnHandler, error) {
	conn, err := net.Dial("tcp", pooler.addr)
	if err != nil {
		log.Println("Error dialing", err.Error())
		return nil, err
	} else {
		messageHandler := LPMessageHandler{}
		connHandler := &ConnHandler{}
		connHandler.Active = true
		connHandler.conn = conn
		connHandler.messageHandler = interface{}(&messageHandler).(MessageHandler)
		messageHandler.connHandler = connHandler
		messageHandler.startHeartbeat()
		go func() {
			connHandler.Listen(conn, &messageHandler)
		}()
		return connHandler, nil
	}
}

func (pooler *ProxyConnPooler) Remove(conn *ConnHandler) {
	conn.conn.Close()
}

func (pooler *ProxyConnPooler) IsActive(conn *ConnHandler) bool {
	return conn.Active
}
