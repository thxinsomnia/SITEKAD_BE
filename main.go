package main

import (

	"log"
	"net/http"
	"os"
	"SITEKAD/controllers/absen"
	"SITEKAD/controllers/auth"
	"SITEKAD/models"
	"github.com/gin-gonic/gin"
)

func init() {	
	models.ConnectDatabase()
	// Define your routes and handlers here
}

func Handler(w http.ResponseWriter, r *http.Request) {
	router := gin.Default()

	// Register routes setelah router diinisialisasi
	router.POST("/login", authcontroller.Login)
	router.GET("/GA", absencontroller.GetAllAbsen)

	router.ServeHTTP(w, r)
}

func main() {
	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server is running on port %s\n", port)

	// Gunakan handler untuk menangani request HTTP
	http.HandleFunc("/", Handler) // Memetakan path "/" ke Handler

	// Mulai server
	log.Fatal(http.ListenAndServe(":"+port, nil))
}