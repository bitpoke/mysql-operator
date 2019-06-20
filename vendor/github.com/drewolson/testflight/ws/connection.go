package ws

import (
	"time"

	"golang.org/x/net/websocket"
)

type Connection struct {
	RawConn          *websocket.Conn
	ReceivedMessages []string
	Timeout          time.Duration
}

func newConnection(conn *websocket.Conn) *Connection {
	connection := &Connection{
		RawConn: conn,
		Timeout: 1 * time.Second,
	}
	return connection
}

func (connection *Connection) Close() {
	connection.RawConn.Close()
}

func (connection *Connection) FlushMessages(number int) *TimeoutError {
	for i := 0; i < number; i++ {
		_, err := connection.ReceiveMessage()
		if err != nil {
			return err
		}
	}
	return nil
}

func (connection *Connection) ReceiveMessage() (string, *TimeoutError) {
	messageChan := make(chan string)

	go connection.receiveMessage(messageChan)

	select {
	case <-time.After(connection.Timeout):
		return "", &TimeoutError{}
	case message := <-messageChan:
		connection.ReceivedMessages = append(connection.ReceivedMessages, message)

		if message != "" {
			return message, nil
		}
	}

	return "", nil
}

func (connection *Connection) SendMessage(message string) {
	websocket.Message.Send(connection.RawConn, message)
}

func (connection *Connection) receiveMessage(messageChan chan string) {
	for {
		var message string
		websocket.Message.Receive(connection.RawConn, &message)

		if message != "" {
			messageChan <- message
			return
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
}
