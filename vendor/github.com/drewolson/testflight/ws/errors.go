package ws

type TimeoutError struct {
}

func (e TimeoutError) Error() string {
	return "Timeout receiving from websocket"
}
