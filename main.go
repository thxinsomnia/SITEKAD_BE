package main

import (

	"log"
	"os"
	"time"
	"SITEKAD/middlewares"
	"SITEKAD/controllers/absen"
	"SITEKAD/controllers/auth"
	"SITEKAD/controllers/cuti"
	"SITEKAD/controllers/lokasi"
	"SITEKAD/controllers/profile"
	"SITEKAD/controllers/lembur"
	"SITEKAD/controllers/penugasan"
	"SITEKAD/controllers/scheduler"
	"SITEKAD/controllers/frauddetect"
	"SITEKAD/models"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cors"
	"github.com/go-co-op/gocron"
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
			api.GET("/pcheck", absencontroller.PrediksiCheckout)
			api.GET("/histori", absencontroller.GetAllAbsen)
			api.GET("/lokasi", lokasicontroller.GetAllLokasi)
			api.GET("/uhistori", absencontroller.GetHistoryUser)
			api.GET("/profile", profilecontroller.GetUserProfile)
			api.POST("/lembur/start", lemburcontrollers.StartOvertimeHandler)
			api.PUT("/lembur/end", lemburcontrollers.EndOvertimeHandler)
			api.GET("/lembur/history", lemburcontrollers.GetHistoryLembur)
			api.POST("/patrol/start", penugasancontrollers.StartPatrolHandler)
			api.POST("/patrol/scan", penugasancontrollers.ScanCheckpointHandler)
			api.POST("/patrol/end", penugasancontrollers.EndPatrolHandler)
			api.POST("/cuti", cuticontrollers.CreateCutiHandler)

				fraud := api.Group("/fa")
				{
					fraud.POST("/train", fraudcontrollers.TrainAnomalyModel)
					fraud.GET("/detect", fraudcontrollers.DetectAttendanceAnomalies)
					fraud.GET("/employee/:id", fraudcontrollers.GetEmployeeAnomalyHistory)
					fraud.GET("/dashboard", fraudcontrollers.GetAnomalyDashboard)
				}

				fakegps := api.Group("/fg")
				{
					fakegps.POST("/train", fraudcontrollers.LatihModelDeteksi)
					fakegps.GET("/detect", fraudcontrollers.DeteksiFakeGps)
					fakegps.GET("/dashboard", fraudcontrollers.GetFakeGpsDashboard)
					fakegps.GET("/health", fraudcontrollers.CekStatusModel)
				}
		}
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	s := gocron.NewScheduler(loc)
	s.Every(1).Hour().Do(scheduler.CleanupStalePatrols)
	s.StartAsync()
	log.Println("Automatic Scheduler Berhasil Dijalankan")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server Berhasil Berjalan Pada Port %s\n", port)

	router.Run(":" + port)
}