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

	// subroomName to nazwa podkanału w którym aktualnie jest użytkownik
	// Pusta wartość oznacza podkanał domyślny od którego wszyscy zaczynają,
	// potem można zmienić podkanał poleceniem "/join #subroomName-name"
	subroomName string

	// socket to gniazdo internetowe do obsługi danego klienta
	socket *websocket.Conn
	// send to kanał, którym są przesyłane komunikaty
	send chan *message
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
		message := &message{
			subroomName: c.subroomName,
			text:        msg,
			client:      c,
		}
		c.room.forward <- message
	}
}

func (c *client) write() {
	defer c.socket.Close()
	for message := range c.send {
		c.subroomName = message.subroomName
		msgText := message.text
		if !message.fromServer {
			msgText = []byte(fmt.Sprintf("%s: %s", message.client.name, string(message.text)))
		}
		err := c.socket.WriteMessage(websocket.TextMessage, msgText)
		if err != nil {
			log.Println("Client socket write error: %v", err)
			return
		}
	}
}
