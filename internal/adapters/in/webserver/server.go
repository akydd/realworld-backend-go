package webserver

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type Server struct {
	port   string
	router http.Handler
}

type ServerHandlers interface {
	RegisterUser(http.ResponseWriter, *http.Request)
	LoginUser(http.ResponseWriter, *http.Request)
}

func NewServer(port string, h ServerHandlers) (*Server, error) {
	r := mux.NewRouter()
	r.HandleFunc("/api/users", h.RegisterUser).Methods("POST")
	r.HandleFunc("/api/users/login", h.LoginUser).Methods("POST")

	// logging!
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	return &Server{
		router: loggedRouter,
		port:   port,
	}, nil
}

func (s *Server) Start() {
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", s.port), s.router))
}

func (s *Server) Stop() {

}
