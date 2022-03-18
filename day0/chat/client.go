package main

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

// Typ client reprezentuje pojedynczego użytkownika
// prowadzącego konwersację z użyciem komunikatora
type client struct {
	// nazwa użytkownika
	name string
	// socket to gniazdo internetowe do obsługi danego klienta
	socket *websocket.Conn
	// send to kanał, którym są przesyłane komunikaty
	send chan []byte
	// room to pokój rozmów używany przez klienta
	room *room
}

func (c *client) read() {
	defer c.socket.Close()
	for {
		_, msg, err := c.socket.ReadMessage()
		if err != nil {
			log.Println("Client socket read error: %v", err)
			return
		}
		msg = []byte(fmt.Sprintf("%s: %s", c.name, string(msg)))
		c.room.forward <- msg
	}
}

func (c *client) write() {
	defer c.socket.Close()
	for msg := range c.send {
		err := c.socket.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("Client socket write error: %v", err)
			return
		}
	}
}
