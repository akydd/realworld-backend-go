package webserver

import (
	"fmt"
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
}

func NewServer(port string, h ServerHandlers) (*Server, error) {
	r := mux.NewRouter()
	r.HandleFunc("/api/users", h.RegisterUser).Methods("POST")

	// logging!
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	return &Server{
		router: loggedRouter,
		port:   port,
	}, nil
}

func (s *Server) Start() {
	http.ListenAndServe(fmt.Sprintf(":%s", s.port), s.router)
}

func (s *Server) Stop() {

}
