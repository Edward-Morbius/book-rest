package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
)

type Book struct {
	Pages []string `json:"pages"`
}

type controller struct {
	mu   sync.RWMutex
	book *Book
}

func newBook() *Book {
	return &Book{Pages: []string{
		"This is the first page.",
		"This is the second page.",
		"This is the third page.",
	}}
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

	c := controller{book: newBook()}

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

	log.Fatalf("error: %v\n", http.ListenAndServe("localhost:8080", r))
}
