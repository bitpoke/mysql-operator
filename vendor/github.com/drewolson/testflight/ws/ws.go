package ws

import (
	"github.com/drewolson/testflight"
	"golang.org/x/net/websocket"
)

func Connect(r *testflight.Requester, route string) *Connection {
	connection, err := websocket.Dial(websocketRoute(r, route), "", "http://localhost/")
	if err != nil {
		panic(err)
	}

	return newConnection(connection)
}

func websocketRoute(r *testflight.Requester, route string) string {
	return "ws://" + r.Url(route)
}
