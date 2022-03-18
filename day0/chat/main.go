package main

import (
	"flag"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"text/template"
)

var allRooms = make(map[string]*room)
var mu sync.RWMutex

// typ reprezentujący pojedynczy szablon
type templateHandler struct {
	once     sync.Once
	filename string
	templ    *template.Template
}

// metoda ServeHTTP obsługuje żądania HTTP
func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join("templates", t.filename)))
	})
	t.templ.Execute(w, r)
}

func getRoom(w http.ResponseWriter, req *http.Request) {
	names, ok := req.URL.Query()["name"]
	if !ok || len(names[0]) < 1 {
		log.Fatal("Brak parametru 'names'")
		return
	}
	name := names[0]
	mu.Lock()
	room, ok := allRooms[name]
	if !ok {
		log.Println("Nowy pokój: ", name)
		room = newRoom(name)
		allRooms[name] = room
		mu.Unlock()
		// uruchomienie pokoju rozmów
		go room.run()
	} else {
		mu.Unlock()
		log.Println("Dołączam do pokoju: ", name)
	}
	room.ServeHTTP(w, req)
}

func removeRoom(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(allRooms, name)
}

func main() {
	var addr = flag.String("addr", ":8080", "Adres aplikacji internetowej.")
	flag.Parse() // analiza flag wiersza poleceń
	http.Handle("/chat", &templateHandler{filename: "chat.html"})
	http.HandleFunc("/room", getRoom)
	// uruchomienie serwera WWW
	log.Println("Uruchamianie serwera WWW pod adresem", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
