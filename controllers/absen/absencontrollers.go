package absencontroller

import (

	"SITEKAD/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

//fungsi ini untuk mendapatkan semua data user
func GetAllUser(c *gin.Context) {
    var users []models.User

    if err := models.DB.Find(&users).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"Message": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"Users": users})
}
