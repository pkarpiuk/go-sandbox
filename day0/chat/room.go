package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const (
	socketBufferSize  = 1024
	mesasgeBufferSize = 256
)

var upgrader = &websocket.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize}

type room struct {
	// name to nazwa kanału
	name string

	// forward to kanał przechowujący nadsyłane komunikaty
	// które należy przesłać do przeglądarki użytkownika
	forward chan []byte

	// join to kanał dla klientów, którzy chcą dołączyć do pokoju
	join chan *client

	// leave to kanał dla klientów, którzy chcą opuścić pokój
	leave chan *client

	// clients zawiera wszystkich klientów, którzy znajdują się w pokoju
	clients map[*client]bool
}

func newRoom(name string) *room {
	return &room{
		name:    name,
		forward: make(chan []byte),
		join:    make(chan *client),
		leave:   make(chan *client),
		clients: make(map[*client]bool),
	}
}

func (r *room) run() {
	for {
		select {
		case client, ok := <-r.join:
			if ok {
				// dołączanie do pokoju
				r.clients[client] = true
				log.Println("Do pokoju dołączył nowy klient!")
			}
		case client, ok := <-r.leave:
			if ok {
				// opuszczanie pokoju
				delete(r.clients, client)
				close(client.send)
				log.Println("Klient opuścił pokój.")
				if len(r.clients) == 0 {
					// ostatni użytkownik opuścił pokój
					log.Println("To był ostatni klient, usuwamy pokój")
					close(r.forward)
					close(r.join)
					close(r.leave)
					removeRoom(r.name)
					return
				}
			}
		case msg, ok := <-r.forward:
			if ok {
				log.Println("Odebrano wiadomość: ", string(msg))
				// rozsyłanie wiadomości do wszystkich klientów
				for client := range r.clients {
					client.send <- msg
					log.Println(" -- wysłano do klienta.")
				}
			}
		}
	}
}

func (r *room) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	userNames, ok := req.URL.Query()["uname"]
	if !ok || len(userNames[0]) < 1 {
		userNames = []string{"Guest"}
		log.Println("Brak parametru 'uname', przyjęty 'Guest'")
	}
	username := userNames[0]
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatal("ServeHTTP:", err)
		return
	}
	client := &client{
		name:   username,
		socket: socket,
		send:   make(chan []byte, mesasgeBufferSize),
		room:   r,
	}
	r.join <- client
	defer func() { r.leave <- client }()
	go client.write()
	client.read()
}
