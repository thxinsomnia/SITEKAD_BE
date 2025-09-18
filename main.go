package main

import (

	"log"
	"os"
	"SITEKAD/middlewares"
	"SITEKAD/controllers/absen"
	"SITEKAD/controllers/auth"
	"SITEKAD/controllers/lokasi"
	"SITEKAD/controllers/profile"
	"SITEKAD/controllers/lembur"
	"SITEKAD/models"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cors"
)


func main() {
	models.ConnectDatabase()
	router := gin.Default()
	router.MaxMultipartMemory = 8 << 20 

	//Cors
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))


	router.POST("/login", authcontroller.LoginHandler)
	router.POST("/aktivasi", authcontroller.Aktivasi)
	router.GET("/logout", authcontroller.Logout)

	api := router.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	{
		api.POST("/absensi", absencontroller.ScanAbsensiHandler)
		api.GET("/histori", absencontroller.GetAllAbsen)
		api.GET("/lokasi", lokasicontroller.GetAllLokasi)
		api.GET("/uhistori", absencontroller.GetHistoryUser)
		api.GET("/profile", profilecontroller.GetUserProfile)
		api.POST("/lembur/start", lemburcontrollers.StartOvertimeHandler)
		api.PUT("/lembur/end", lemburcontrollers.EndOvertimeHandler)
	}

	// 5. Jalankan Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server is running on port %s\n", port)

	router.Run(":" + port)
}