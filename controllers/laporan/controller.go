package laporancontrollers

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

func HitungKehadiran(attendances []models.Absensi, bulan time.Time) models.LaporanAbsensi {
    summary := models.LaporanAbsensi{
        Bulan: bulan.Format("January 2006"),
    }

    // Define standard work hours
    const standardWorkStart = 8 * 60  // 08:00 in minutes
    const standardWorkEnd = 17 * 60   // 17:00 in minutes
    const standardJamKerja = 8.0

    var totalWorkMinutes float64
    var completeDays int

    for _, absen := range attendances {
        summary.TotalHadir++

        // Check if late (after 08:00)
        checkInTime, _ := time.Parse("15:04:05", absen.Jam_masuk)
        checkInMinutes := checkInTime.Hour()*60 + checkInTime.Minute()
        
        if checkInMinutes > standardWorkStart {
            summary.HariTelat++
        }

        // Calculate work hours if checked out
        if absen.Jam_keluar != nil {
            completeDays++
            
            checkOutTime, _ := time.Parse("15:04:05", *absen.Jam_keluar)
            checkOutMinutes := checkOutTime.Hour()*60 + checkOutTime.Minute()

            // Calculate work duration in minutes
            workMinutes := float64(checkOutMinutes - checkInMinutes)
            
            // Handle overnight work (if checkout is before checkin)
            if workMinutes < 0 {
                workMinutes += 24 * 60
            }

            totalWorkMinutes += workMinutes

            // Check early checkout (before 17:00)
            if checkOutMinutes < standardWorkEnd {
                summary.CheckoutLebihAwal++
            }

            // Calculate overtime (work hours > 8)
            JamKerja := workMinutes / 60
            if JamKerja > standardJamKerja {
                summary.WaktuLembur += (JamKerja - standardJamKerja)
            }
        }
    }

    // Calculate totals
    summary.TotalJamKerja = totalWorkMinutes / 60
    
    if completeDays > 0 {
        summary.RataRataJamKerja = summary.TotalJamKerja / float64(completeDays)
    }

    // Calculate total work days in bulan (excluding weekends)
    summary.TotalHariKerja = hitungHariKerjaPerbulan(bulan)
    summary.TotalAbsen = summary.TotalHariKerja - summary.TotalHadir
    
    // Calculate attendance rate
    if summary.TotalHariKerja > 0 {
        summary.PersentaseKehadiran = (float64(summary.TotalHadir) / float64(summary.TotalHariKerja)) * 100
    }

    return summary
}

func hitungHariKerjaPerbulan(bulan time.Time) int {
    tahun, bulanNum, _ := bulan.Date()
    hariPertama := time.Date(tahun, bulanNum, 1, 0, 0, 0, 0, time.Local)
    hariTerakhir := hariPertama.AddDate(0, 1, 0).AddDate(0, 0, -1)

    hariKerja := 0
    for d := hariPertama; !d.After(hariTerakhir); d = d.AddDate(0, 0, 1) {
        // Count only weekdays (Monday-Friday)
        if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
            hariKerja++
        }
    }

    return hariKerja
}

func LaporanAbsensiHarian(c *gin.Context) {
    userData, exists := c.Get("currentUser")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi pengguna tidak valid"})
        return
    }
    currentUser := userData.(models.Penempatan)

  
    bulan := c.DefaultQuery("bulan", time.Now().Format("2006-01"))
    bulanTime, err := time.Parse("2006-01", bulan)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format bulan tidak valid"})
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
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
        return
    }

    attendanceMap := make(map[string]models.Absensi)
    for _, a := range attendances {
        attendanceMap[a.Tgl_absen] = a
    }

    // Build daily breakdown
    var LaporanHarian []models.LaporanAbsensiHarian
    
    tahun, bulanNum, _ := bulanTime.Date()
    hariPertama := time.Date(tahun, bulanNum, 1, 0, 0, 0, 0, time.Local)
    hariTerakhir := hariPertama.AddDate(0, 1, 0).AddDate(0, 0, -1)

    const standardWorkStart = 8 * 60
    const standardJamKerja = 8.0

    for d := hariPertama; !d.After(hariTerakhir); d = d.AddDate(0, 0, 1) {
        dateStr := d.Format("2006-01-02")
        NamaHari := d.Format("Monday")
        
        // Skip weekends
        if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
            continue
        }

        daily := models.LaporanAbsensiHarian{
            Tanggal:  dateStr,
            NamaHari: NamaHari,
        }

        if absen, found := attendanceMap[dateStr]; found {
            daily.JamMasuk = absen.Jam_masuk
            daily.JamKeluar = absen.Jam_keluar

            // Determine status
            checkInTime, _ := time.Parse("15:04:05", absen.Jam_masuk)
            checkInMinutes := checkInTime.Hour()*60 + checkInTime.Minute()
            
            if checkInMinutes > standardWorkStart {
                daily.Status = "telat"
            } else {
                daily.Status = "tepat waktu"
            }

            // Calculate work hours
            if absen.Jam_keluar != nil {
                checkOutTime, _ := time.Parse("15:04:05", *absen.Jam_keluar)
                checkOutMinutes := checkOutTime.Hour()*60 + checkOutTime.Minute()
                
                workMinutes := float64(checkOutMinutes - checkInMinutes)
                if workMinutes < 0 {
                    workMinutes += 24 * 60
                }
                
                daily.JamKerja = workMinutes / 60

                // Check overtime
                if daily.JamKerja > standardJamKerja {
                    daily.IsLembur = true
                    daily.JamLembur = daily.JamKerja - standardJamKerja
                }
            }
        } else {
            daily.Status = "absen/tidak hadir"
        }

        LaporanHarian = append(LaporanHarian, daily)
    }

    c.JSON(http.StatusOK, gin.H{
        "bulan": bulan,
        "kehadiran": LaporanHarian,
    })
}