package main

import (

	"log"
	"net/http"
	"os"
	"SITEKAD/controllers/absen"
	"SITEKAD/controllers/auth"
	"SITEKAD/controllers/lokasi"
	"SITEKAD/models"
	"github.com/gin-gonic/gin"
)

func init() {	
	models.ConnectDatabase()
}

func Handler(w http.ResponseWriter, r *http.Request) {
	router := gin.Default()

	router.POST("/login", authcontroller.Login)
	router.GET("/histori", absencontroller.GetAllAbsen)
	router.GET("/lokasi", lokasicontroller.GetAllLokasi)
	router.POST("/aktivasi", authcontroller.Aktivasi)
	router.GET("/logout", authcontroller.Logout)

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