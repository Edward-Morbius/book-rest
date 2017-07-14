package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
)

type Book struct {
	Pages []string `json:"pages"`
}

type controller struct {
	mu   sync.RWMutex
	book *Book
}

const bookShelveFilename = "bookshelv.json"

func storeBook(book *Book) error {
	f, err := os.Create(bookShelveFilename)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	err = enc.Encode(book)
	if err != nil {
		return err
	}
	return nil
}

func newBook() (*Book, error) {
	var book Book
	f, err := os.Open(bookShelveFilename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	err = dec.Decode(&book)
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (c *controller) create(rw http.ResponseWriter, req *http.Request) {

	type page struct {
		Text string `json:"text"`
		Pos  *int   `json:"pos"`
	}

	var p page

	defer req.Body.Close()
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&p); err != nil {
		http.Error(rw, "invalid page document", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b := c.book

	var pos int
	if p.Pos != nil {
		pos = *p.Pos
	} else {
		pos = len(b.Pages)
	}

	if pos < 0 || pos > len(b.Pages) {
		http.Error(rw,
			fmt.Sprintf("page %d is out of range", pos),
			http.StatusBadRequest)
		return
	}

	b.Pages = append(b.Pages, "")
	copy(b.Pages[pos+1:], b.Pages[pos:])
	b.Pages[pos] = p.Text

	rw.Header().Set("Content-Type", "text/json")
	fmt.Fprintf(rw, `{"msg":"page %d inserted successfully."}\n`, pos)
}

func (c *controller) read(rw http.ResponseWriter, req *http.Request) {

	ctx := req.Context()
	fmt.Printf("user: %s\n", ctx.Value("user"))

	c.mu.RLock()
	defer c.mu.RUnlock()

	rw.Header().Set("Content-Type", "text/json")
	encoder := json.NewEncoder(rw)
	if err := encoder.Encode(&c.book); err != nil {
		log.Printf("warn: %v\n", err)
	}
}

func (c *controller) update(rw http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)
	id, _ := strconv.Atoi(vars["id"])

	type page struct {
		Text string `json:"text"`
	}

	var p page

	defer req.Body.Close()
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&p); err != nil {
		http.Error(rw, "invalid page document", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b := c.book
	if id >= len(b.Pages) {
		http.NotFound(rw, req)
		return
	}
	b.Pages[id] = p.Text

	rw.Header().Set("Content-Type", "text/json")
	fmt.Fprintf(rw, `{"msg":"page %d updated successfully."}\n`, id)
}

func (c *controller) delete(rw http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)
	id, _ := strconv.Atoi(vars["id"])

	c.mu.Lock()
	defer c.mu.Unlock()

	b := c.book

	if id >= len(b.Pages) {
		http.NotFound(rw, req)
		return
	}

	copy(b.Pages[id:], b.Pages[id+1:])
	b.Pages[len(b.Pages)-1] = ""
	b.Pages = b.Pages[:len(b.Pages)-1]

	rw.Header().Set("Content-Type", "text/json")
	fmt.Fprintf(rw, `{"msg":"page %d deleted successfully."}\n`, id)
}

func middleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(
		func(rw http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			// Do something more fancy here.
			ctx = context.WithValue(ctx, "user", "from_middleware")
			req = req.WithContext(ctx)
			next.ServeHTTP(rw, req)
		})
}

func main() {
	book, err := newBook()
	if err != nil {
		log.Fatalln(err)
	}
	c := controller{book: book}

	r := mux.NewRouter()
	r.Path("/pages").
		Methods("GET").
		Handler(middleware(http.HandlerFunc(c.read)))
	r.Path("/pages").
		Methods("PUT").
		Handler(http.HandlerFunc(c.create))
	r.Path("/pages/{id:[0-9]+}").
		Methods("UPDATE").
		Handler(http.HandlerFunc(c.update))
	r.Path("/pages/{id:[0-9]+}").
		Methods("DELETE").
		Handler(http.HandlerFunc(c.delete))
	//Ger server object.
	server := http.Server{Addr: "localhost:8080", Handler: r}
	//Make channel for signalling end.
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v\n", err)
		}
		log.Printf("server: %v\n")
		fmt.Println("Goroutine: server down")
	}()
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, os.Kill)
	//Block until we get a signal.
	<-sigChan
	fmt.Println("sigChan:", sigChan)
	server.Shutdown(context.Background())
	//Block until server shutdown finished, see goroutine above.
	<-done
	fmt.Println("Storing books...")
	err = storeBook(book)
	if err != nil {
		log.Fatalf("Can not write book shelv: %v\n", err)
	}
}
