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
	GetUser(http.ResponseWriter, *http.Request)
	UpdateUser(http.ResponseWriter, *http.Request)
	GetProfile(http.ResponseWriter, *http.Request)
	FollowUser(http.ResponseWriter, *http.Request)
	UnfollowUser(http.ResponseWriter, *http.Request)
	CreateArticle(http.ResponseWriter, *http.Request)
}

func NewServer(port string, h ServerHandlers, jwtSecret string) (*Server, error) {
	r := mux.NewRouter()
	r.HandleFunc("/api/users", h.RegisterUser).Methods("POST")
	r.HandleFunc("/api/users/login", h.LoginUser).Methods("POST")

	protected := r.NewRoute().Subrouter()
	protected.Use(authMiddleware(jwtSecret))
	protected.HandleFunc("/api/user", h.GetUser).Methods("GET")
	protected.HandleFunc("/api/user", h.UpdateUser).Methods("PUT")
	protected.HandleFunc("/api/profiles/{username}/follow", h.FollowUser).Methods("POST")
	protected.HandleFunc("/api/profiles/{username}/follow", h.UnfollowUser).Methods("DELETE")
	protected.HandleFunc("/api/articles", h.CreateArticle).Methods("POST")

	optionalAuth := r.NewRoute().Subrouter()
	optionalAuth.Use(optionalAuthMiddleware(jwtSecret))
	optionalAuth.HandleFunc("/api/profiles/{username}", h.GetProfile).Methods("GET")

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
