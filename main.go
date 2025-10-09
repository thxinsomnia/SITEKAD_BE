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
	"SITEKAD/controllers/penugasan"
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

	v1 := router.Group("/v1")
	{
		v1.POST("/login", authcontroller.LoginHandler)
		v1.POST("/aktivasi", authcontroller.Aktivasi)
		v1.GET("/logout", authcontroller.Logout)

			api := v1.Group("/api")
			api.Use(middlewares.AuthMiddleware())
		{
			api.POST("/absensi", absencontroller.ScanAbsensiHandler)
			api.GET("/histori", absencontroller.GetAllAbsen)
			api.GET("/lokasi", lokasicontroller.GetAllLokasi)
			api.GET("/uhistori", absencontroller.GetHistoryUser)
			api.GET("/profile", profilecontroller.GetUserProfile)
			api.POST("/lembur/start", lemburcontrollers.StartOvertimeHandler)
			api.PUT("/lembur/end", lemburcontrollers.EndOvertimeHandler)
			api.GET("/lembur/history", lemburcontrollers.GetHistoryLembur)
			api.POST("/patrol/start", penugasan.StartPatrolHandler)
			api.POST("/patrol/scan", penugasan.ScanCheckpointHandler)
		}
	}

	

	// 5. Jalankan Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server is running on port %s\n", port)

	router.Run(":" + port)
}