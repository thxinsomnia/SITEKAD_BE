package absencontroller

import (

	"SITEKAD/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

//Histori Absensi
func GetAllAbsen(c *gin.Context) {
    var absensi []models.Absensi

    if err := models.DB.Order("created_at desc").Find(&absensi).Limit(10).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"Message": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"Absen": absensi})
}
