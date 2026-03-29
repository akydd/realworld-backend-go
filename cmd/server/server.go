package main

import (
	"flag"
	"log"
	"os"

	"realworld-backend-go/internal/adapters/in/webserver"
	"realworld-backend-go/internal/adapters/out/db"
	"realworld-backend-go/internal/domain"

	"github.com/joho/godotenv"
)

func main() {
	// load configs
	envFile := flag.String("env", ".env", "path to env file")
	flag.Parse()
	if _, err := os.Stat(*envFile); err == nil {
		if err := godotenv.Load(*envFile); err != nil {
			log.Fatal(err)
		}
	}

	// Setup all dependencies
	database, err := db.New(&db.DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
	})
	if err != nil {
		log.Fatal(err)
	}
	userController := domain.New(database, os.Getenv("JWT_SECRET"))
	profileController := domain.NewProfileController(database)
	articleController := domain.NewArticleController(database)
	tagController := domain.NewTagController(database)
	commentController := domain.NewCommentController(database)
	handlers := webserver.NewHandler(userController, profileController, articleController, tagController, commentController)

	port := os.Getenv("SERVER_PORT")

	log.Printf("starting server on port %s...\n", port)

	s, err := webserver.NewServer(port, handlers, os.Getenv("JWT_SECRET"))
	if err != nil {
		log.Fatal(err)
	}

	s.Start()
}
