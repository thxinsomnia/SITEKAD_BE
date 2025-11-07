package laporancontrollers

import (
	"SITEKAD/models"
	"net/http"
	"time"
     "github.com/jung-kurt/gofpdf"
    "github.com/xuri/excelize/v2"
    "fmt"

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


// Export Summary to PDF
func ExportSummaryToPDF(c *gin.Context) {
    userData, exists := c.Get("currentUser")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi pengguna tidak valid"})
        return
    }
    currentUser := userData.(models.Penempatan)
    if err := models.DB.Preload("Pkwt.Tad").First(&currentUser, currentUser.Id).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pengguna"})
        return
    }

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

    summary := HitungKehadiran(attendances, bulanTime)

    // Create PDF
    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.AddPage()
    
    // Set font
    pdf.SetFont("Arial", "B", 16)
    pdf.Cell(40, 10, "Laporan Kehadiran")
    pdf.Ln(12)

    // Add user info
    pdf.SetFont("Arial", "", 12)
    pdf.Cell(40, 10, fmt.Sprintf("Nama: %s", currentUser.Pkwt.Tad.Nama))
    pdf.Ln(8)
    pdf.Cell(40, 10, fmt.Sprintf("Bulan: %s", summary.Bulan))
    pdf.Ln(12)

    // Add summary table
    pdf.SetFont("Arial", "B", 12)
    pdf.Cell(90, 10, "Metrik")
    pdf.Cell(90, 10, "Nilai")
    pdf.Ln(10)

    pdf.SetFont("Arial", "", 11)
    
    // Table rows
    metrics := []struct {
        label string
        value string
    }{
        {"Total Hari Kerja", fmt.Sprintf("%d hari", summary.TotalHariKerja)},
        {"Total Hadir", fmt.Sprintf("%d hari", summary.TotalHadir)},
        {"Total Absen", fmt.Sprintf("%d hari", summary.TotalAbsen)},
        {"Hari Telat", fmt.Sprintf("%d hari", summary.HariTelat)},
        {"Checkout Lebih Awal", fmt.Sprintf("%d hari", summary.CheckoutLebihAwal)},
        {"Total Jam Kerja", fmt.Sprintf("%.2f jam", summary.TotalJamKerja)},
        {"Rata-rata Jam Kerja", fmt.Sprintf("%.2f jam/hari", summary.RataRataJamKerja)},
        {"Waktu Lembur", fmt.Sprintf("%.2f jam", summary.WaktuLembur)},
        {"Persentase Kehadiran", fmt.Sprintf("%.2f%%", summary.PersentaseKehadiran)},
    }

    for _, m := range metrics {
        pdf.Cell(90, 8, m.label)
        pdf.Cell(90, 8, m.value)
        pdf.Ln(8)
    }

    // Add footer
    pdf.Ln(10)
    pdf.SetFont("Arial", "I", 9)
    pdf.Cell(0, 10, fmt.Sprintf("Dibuat pada: %s", time.Now().Format("02 January 2006 15:04:05")))

    // Output to buffer
    filename := fmt.Sprintf("Laporan_Kehadiran_%s_%s.pdf", 
        currentUser.Pkwt.Tad.Nama, bulanTime.Format("2006-01"))

    c.Header("Content-Type", "application/pdf")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
    
    err = pdf.Output(c.Writer)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat PDF"})
        return
    }
}

// Export Summary to Excel
func ExportSummaryToExcel(c *gin.Context) {
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

    summary := HitungKehadiran(attendances, bulanTime)

    // Create Excel file
    f := excelize.NewFile()
    defer f.Close()

    sheetName := "Ringkasan"
    index, _ := f.NewSheet(sheetName)
    f.SetActiveSheet(index)

    // Set headers with styling
    headerStyle, _ := f.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 12},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })

    // Title
    f.SetCellValue(sheetName, "A1", "LAPORAN KEHADIRAN")
    f.MergeCell(sheetName, "A1", "B1")
    f.SetCellStyle(sheetName, "A1", "B1", headerStyle)
    f.SetRowHeight(sheetName, 1, 25)

    // User info
    f.SetCellValue(sheetName, "A3", "Nama:")
    f.SetCellValue(sheetName, "B3", currentUser.Pkwt.Tad.Nama)
    f.SetCellValue(sheetName, "A4", "Bulan:")
    f.SetCellValue(sheetName, "B4", summary.Bulan)

    // Summary table headers
    f.SetCellValue(sheetName, "A6", "Metrik")
    f.SetCellValue(sheetName, "B6", "Nilai")
    f.SetCellStyle(sheetName, "A6", "B6", headerStyle)

    // Summary data
    row := 7
    summaryData := [][]interface{}{
        {"Total Hari Kerja", summary.TotalHariKerja},
        {"Total Hadir", summary.TotalHadir},
        {"Total Absen", summary.TotalAbsen},
        {"Hari Telat", summary.HariTelat},
        {"Checkout Lebih Awal", summary.CheckoutLebihAwal},
        {"Total Jam Kerja", fmt.Sprintf("%.2f jam", summary.TotalJamKerja)},
        {"Rata-rata Jam Kerja", fmt.Sprintf("%.2f jam/hari", summary.RataRataJamKerja)},
        {"Waktu Lembur", fmt.Sprintf("%.2f jam", summary.WaktuLembur)},
        {"Persentase Kehadiran", fmt.Sprintf("%.2f%%", summary.PersentaseKehadiran)},
    }

    for _, data := range summaryData {
        f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), data[0])
        f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), data[1])
        row++
    }

    // Auto-fit columns
    f.SetColWidth(sheetName, "A", "A", 25)
    f.SetColWidth(sheetName, "B", "B", 20)

    // Add timestamp
    row += 2
    f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), 
        fmt.Sprintf("Dibuat pada: %s", time.Now().Format("02 January 2006 15:04:05")))

    // Delete default sheet
    f.DeleteSheet("Sheet1")

    // Output to response
    filename := fmt.Sprintf("Laporan_Kehadiran_%s_%s.xlsx", 
        currentUser.Pkwt.Tad.Nama, bulanTime.Format("2006-01"))

    c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

    if err := f.Write(c.Writer); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat Excel"})
        return
    }
}

// Export Daily Report to Excel
func ExportDailyToExcel(c *gin.Context) {
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

    // Build daily data
    var LaporanHarian []models.LaporanAbsensiHarian
    tahun, bulanNum, _ := bulanTime.Date()
    hariPertama := time.Date(tahun, bulanNum, 1, 0, 0, 0, 0, time.Local)
    hariTerakhir := hariPertama.AddDate(0, 1, 0).AddDate(0, 0, -1)

    const standardWorkStart = 8 * 60
    const standardJamKerja = 8.0

    for d := hariPertama; !d.After(hariTerakhir); d = d.AddDate(0, 0, 1) {
        if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
            continue
        }

        dateStr := d.Format("2006-01-02")
        daily := models.LaporanAbsensiHarian{
            Tanggal:  dateStr,
            NamaHari: d.Format("Monday"),
        }

        if absen, found := attendanceMap[dateStr]; found {
            daily.JamMasuk = absen.Jam_masuk
            daily.JamKeluar = absen.Jam_keluar

            checkInTime, _ := time.Parse("15:04:05", absen.Jam_masuk)
            checkInMinutes := checkInTime.Hour()*60 + checkInTime.Minute()

            if checkInMinutes > standardWorkStart {
                daily.Status = "telat"
            } else {
                daily.Status = "tepat waktu"
            }

            if absen.Jam_keluar != nil {
                checkOutTime, _ := time.Parse("15:04:05", *absen.Jam_keluar)
                checkOutMinutes := checkOutTime.Hour()*60 + checkOutTime.Minute()

                workMinutes := float64(checkOutMinutes - checkInMinutes)
                if workMinutes < 0 {
                    workMinutes += 24 * 60
                }

                daily.JamKerja = workMinutes / 60

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

    // Create Excel
    f := excelize.NewFile()
    defer f.Close()

    sheetName := "Laporan Harian"
    index, _ := f.NewSheet(sheetName)
    f.SetActiveSheet(index)

    // Header styling
    headerStyle, _ := f.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 11},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })

    // Title
    f.SetCellValue(sheetName, "A1", "LAPORAN KEHADIRAN HARIAN")
    f.MergeCell(sheetName, "A1", "G1")
    f.SetCellStyle(sheetName, "A1", "G1", headerStyle)

    // User info
    f.SetCellValue(sheetName, "A2", fmt.Sprintf("Nama: %s", currentUser.Pkwt.Tad.Nama))
    f.SetCellValue(sheetName, "A3", fmt.Sprintf("Bulan: %s", bulanTime.Format("January 2006")))

    // Table headers
    headers := []string{"Tanggal", "Hari", "Jam Masuk", "Jam Keluar", "Jam Kerja", "Status", "Lembur"}
    for i, h := range headers {
        cell := fmt.Sprintf("%s5", string(rune('A'+i)))
        f.SetCellValue(sheetName, cell, h)
        f.SetCellStyle(sheetName, cell, cell, headerStyle)
    }

    // Data rows
    row := 6
    for _, daily := range LaporanHarian {
        jamKeluar := ""
        if daily.JamKeluar != nil {
            jamKeluar = *daily.JamKeluar
        }

        lembur := "Tidak"
        if daily.IsLembur {
            lembur = fmt.Sprintf("Ya (%.2f jam)", daily.JamLembur)
        }

        f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), daily.Tanggal)
        f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), daily.NamaHari)
        f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), daily.JamMasuk)
        f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), jamKeluar)
        f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("%.2f", daily.JamKerja))
        f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), daily.Status)
        f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), lembur)
        row++
    }

    // Auto-fit columns
    f.SetColWidth(sheetName, "A", "A", 12)
    f.SetColWidth(sheetName, "B", "B", 12)
    f.SetColWidth(sheetName, "C", "D", 12)
    f.SetColWidth(sheetName, "E", "E", 10)
    f.SetColWidth(sheetName, "F", "F", 18)
    f.SetColWidth(sheetName, "G", "G", 18)

    f.DeleteSheet("Sheet1")

    filename := fmt.Sprintf("Laporan_Harian_%s_%s.xlsx",
        currentUser.Pkwt.Tad.Nama, bulanTime.Format("2006-01"))

    c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

    if err := f.Write(c.Writer); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat Excel"})
        return
    }
}