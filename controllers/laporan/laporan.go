package laporan

import (
	"SITEKAD/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func GetLaporan(c *gin.Context) {
    userData, exists := c.Get("currentUser")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi pengguna tidak valid"})
        return
    }

    currentUser := userData.(models.Penempatan)
    bulan := c.DefaultQuery("bulan", time.Now().Format("2006-01"))
    bulanTime, err := time.Parse("2006-01", bulan)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format bulan tidak valid. Gunakan YYYY-MM"})
        return
    }

   
    tanggalAwal := bulanTime.Format("2006-01-02")
    tanggalAkhir := bulanTime.AddDate(0, 1, 0).AddDate(0, 0, -1).Format("2006-01-02")
    var attendances []models.Absensi
    err = models.DB.Where("penempatan_id = ? AND tgl_absen BETWEEN ? AND ?",
        currentUser.Id, tanggalAwal, tanggalAkhir).
        Order("tgl_absen ASC").
        Find(&attendances).Error

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data absensi"})
        return
    }

    summary := HitungKehadiran(attendances, bulanTime)
    c.JSON(http.StatusOK, gin.H{
        "summary": summary,
    })
}

