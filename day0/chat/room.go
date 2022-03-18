package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

const (
	socketBufferSize  = 1024
	mesasgeBufferSize = 256
)

var upgrader = &websocket.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize}

type message struct {
	subroomName string
	text        []byte
	client      *client
	fromServer  bool
}

type subroom struct {
	name  string
	owner *client
}

type room struct {
	// name to nazwa kanału
	name string

	// forward to kanał przechowujący nadsyłane komunikaty
	// które należy przesłać do przeglądarki użytkownika
	forward chan *message

	// join to kanał dla klientów, którzy chcą dołączyć do pokoju
	join chan *client

	// leave to kanał dla klientów, którzy chcą opuścić pokój
	leave chan *client

	// clients zawiera wszystkich klientów, którzy znajdują się w pokoju
	clients map[*client]bool

	allSubrooms map[string]*subroom
}

func newRoom(name string) *room {
	return &room{
		name:        name,
		forward:     make(chan *message),
		join:        make(chan *client),
		leave:       make(chan *client),
		clients:     make(map[*client]bool),
		allSubrooms: make(map[string]*subroom),
	}
}

func sendMsgFromServer(message *message, text string) {
	message.text = []byte(text)
	message.fromServer = true
	message.client.send <- message
}

func (r *room) getUsersInChannel(subroomName string) []string {
	list := make([]string, 0)
	for client := range r.clients {
		if client.subroomName == subroomName {
			list = append(list, client.name)
		}
	}
	return list
}

func (r *room) run() {
	for {
		select {
		case client, ok := <-r.join:
			if ok { // not ok to pusty kanał
				// dołączanie do pokoju
				r.clients[client] = true
				message := &message{
					subroomName: "",
					client:      client,
				}
				sendMsgFromServer(message, "Dostępne polecenia: /create, /join, /unjoin, /list, /who")

				log.Println("Do pokoju dołączył nowy klient!")
			}
		case client, ok := <-r.leave:
			if ok { // not ok to pusty kanał
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
		case message, ok := <-r.forward:
			if ok { // not ok to pusty kanał
				messageStr := strings.TrimSpace(string(message.text))
				log.Println("Odebrano wiadomość: ", messageStr, " z podkanału ", message.subroomName)
				if strings.HasPrefix(messageStr, "/") {
					chunks := strings.Split(messageStr[1:], " ")
					cmd := chunks[0]
					log.Printf("COMMAND: %v %v", chunks, len(chunks))
					params := ""
					if len(chunks) > 1 {
						params = chunks[1]
					}
					if cmd == "create" && len(params) > 0 {
						subroomName := params
						_, ok := r.allSubrooms[subroomName]
						if !ok {
							subroom := &subroom{
								name:  subroomName,
								owner: message.client,
							}
							r.allSubrooms[subroomName] = subroom
							sendMsgFromServer(message, fmt.Sprintf("Kanał %s utworzony", subroomName))
						} else {
							sendMsgFromServer(message, fmt.Sprintf("Błąd: istnieje już kanał o takiej nazwie (%s)", subroomName))
						}
					} else if cmd == "join" && len(params) > 0 {
						subroomName := params
						log.Printf("Subroom name: '%s'", subroomName)
						subroom, ok := r.allSubrooms[subroomName]
						if ok {
							message.subroomName = subroom.name
							sendMsgFromServer(message, fmt.Sprintf("Udane przejście do kanału %s", subroomName))
						} else {
							sendMsgFromServer(message, fmt.Sprintf("Błąd: nie istnieje kanał o takiej nazwie (%s)", subroomName))
						}
					} else if cmd == "unjoin" {
						message.subroomName = ""
						sendMsgFromServer(message, "Udany powrót do kanału głównego")
					} else if cmd == "list" {
						list := make([]string, 0)
						for name := range r.allSubrooms {
							list = append(list, name)
						}
						sendMsgFromServer(message, fmt.Sprintf("Dostępne kanały (%d): %s", len(r.allSubrooms), strings.Join(list, ", ")))
					} else if cmd == "who" {
						list := r.getUsersInChannel(message.subroomName)
						sendMsgFromServer(message, fmt.Sprintf("W bieżącym kanale są użytkownicy (%d): %s", len(list), strings.Join(list, ", ")))
					} else {
						sendMsgFromServer(message, fmt.Sprintf("Błąd: nieznane polecenie: %s", messageStr))
					}
				} else {
					// rozsyłanie wiadomości do wszystkich klientów
					for client := range r.clients {
						if client.subroomName == message.subroomName {
							client.send <- message
							log.Println(" -- wysłano do klienta", client.name)
						}
					}
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
		send:   make(chan *message, mesasgeBufferSize),
		room:   r,
	}
	r.join <- client
	defer func() { r.leave <- client }()
	go client.write()
	client.read()
}
