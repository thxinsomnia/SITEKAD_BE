package penugasan 

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"SITEKAD/models"
	"gorm.io/gorm"
	"time"	
)

func StartPatrolHandler(c *gin.Context) {

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var activePengerjaan models.PengerjaanTugas
	err := models.DB.Where("penempatan_id = ? AND status = ?", currentUser.Id, "berlangsung").First(&activePengerjaan).Error

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Anda sudah memiliki sesi patroli yang sedang berlangsung",
			"ptid":  activePengerjaan.Ptid,
		})
		return
	}

	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memeriksa sesi patroli"})
		return
	}

	newPengerjaan := models.PengerjaanTugas{
		PenempatanId: currentUser.Id,
		WaktuMulai:   time.Now(),
		Status:       "berlangsung",
	}

	if err := models.DB.Create(&newPengerjaan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai sesi patroli"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Sesi patroli berhasil dimulai",
		"ptid": newPengerjaan.Ptid,
	})
}

func GetPenugasan(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Get Penugasan"})
}
