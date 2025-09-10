package main

import (

	"log"
	"net/http"
	"os"
	"SITEKAD/middlewares"
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

	router.POST("/login", absencontroller.LoginHandler)
	router.GET("/histori", absencontroller.GetAllAbsen)
	router.GET("/lokasi", lokasicontroller.GetAllLokasi)
	router.POST("/aktivasi", authcontroller.Aktivasi)
	router.GET("/logout", authcontroller.Logout)
	

	api := router.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	api.POST("/absensi", absencontroller.ScanAbsensiHandler)
	api.GET("/histori", absencontroller.GetAllAbsen)
	api.GET("/lokasi", lokasicontroller.GetAllLokasi)

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