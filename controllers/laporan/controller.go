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

 
    const standardWorkStart = 8 * 60  
    const standardWorkEnd = 17 * 60   
    const standardJamKerja = 8.0
    var totalWorkMinutes float64
    var completeDays int

    for _, absen := range attendances {
        summary.TotalHadir++

        checkInTime, _ := time.Parse("15:04:05", absen.Jam_masuk)
        checkInMinutes := checkInTime.Hour()*60 + checkInTime.Minute()
        if checkInMinutes > standardWorkStart {
            summary.HariTelat++
        }
        if absen.Jam_keluar != nil {
            completeDays++
            
            checkOutTime, _ := time.Parse("15:04:05", *absen.Jam_keluar)
            checkOutMinutes := checkOutTime.Hour()*60 + checkOutTime.Minute()
            workMinutes := float64(checkOutMinutes - checkInMinutes)
            if workMinutes < 0 {
                workMinutes += 24 * 60
            }

            totalWorkMinutes += workMinutes
            if checkOutMinutes < standardWorkEnd {
                summary.CheckoutLebihAwal++
            }

   
            JamKerja := workMinutes / 60
            if JamKerja > standardJamKerja {
                summary.WaktuLembur += (JamKerja - standardJamKerja)
            }
        }
    }


    summary.TotalJamKerja = totalWorkMinutes / 60 
    if completeDays > 0 {
        summary.RataRataJamKerja = summary.TotalJamKerja / float64(completeDays)
    }


    summary.TotalHariKerja = hitungHariKerjaPerbulan(bulan)
    summary.TotalAbsen = summary.TotalHariKerja - summary.TotalHadir
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

    var LaporanHarian []models.LaporanAbsensiHarian
    tahun, bulanNum, _ := bulanTime.Date()
    hariPertama := time.Date(tahun, bulanNum, 1, 0, 0, 0, 0, time.Local)
    hariTerakhir := hariPertama.AddDate(0, 1, 0).AddDate(0, 0, -1)

    const standardWorkStart = 8 * 60
    const standardJamKerja = 8.0

    for d := hariPertama; !d.After(hariTerakhir); d = d.AddDate(0, 0, 1) {
        dateStr := d.Format("2006-01-02")
        NamaHari := d.Format("Monday")
        
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

    c.JSON(http.StatusOK, gin.H{
        "bulan": bulan,
        "kehadiran": LaporanHarian,
    })
}


func EksporRekapPDF(c *gin.Context) {
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

 
    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.AddPage()
    pdf.SetFont("Arial", "B", 16)
    pdf.Cell(40, 10, "Laporan Kehadiran")
    pdf.Ln(12)

 
    pdf.SetFont("Arial", "", 12)
    pdf.Cell(40, 10, fmt.Sprintf("Nama: %s", currentUser.Pkwt.Tad.Nama))
    pdf.Ln(8)
    pdf.Cell(40, 10, fmt.Sprintf("Bulan: %s", summary.Bulan))
    pdf.Ln(12)

   
    pdf.SetFont("Arial", "B", 12)
    pdf.Cell(90, 10, "Metrik")
    pdf.Cell(90, 10, "Nilai")
    pdf.Ln(10)

    pdf.SetFont("Arial", "", 11)
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


    pdf.Ln(10)
    pdf.SetFont("Arial", "I", 9)
    pdf.Cell(0, 10, fmt.Sprintf("Dibuat pada: %s", time.Now().Format("02 January 2006 15:04:05")))
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


func EksporRekapExcel(c *gin.Context) {
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
    f := excelize.NewFile()
    defer f.Close()
    sheetName := "Ringkasan"
    index, _ := f.NewSheet(sheetName)
    f.SetActiveSheet(index)


    headerStyle, _ := f.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 12},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })
    f.SetCellValue(sheetName, "A1", "LAPORAN KEHADIRAN")
    f.MergeCell(sheetName, "A1", "B1")
    f.SetCellStyle(sheetName, "A1", "B1", headerStyle)
    f.SetRowHeight(sheetName, 1, 25)

 
    f.SetCellValue(sheetName, "A3", "Nama:")
    f.SetCellValue(sheetName, "B3", currentUser.Pkwt.Tad.Nama)
    f.SetCellValue(sheetName, "A4", "Bulan:")
    f.SetCellValue(sheetName, "B4", summary.Bulan)


    f.SetCellValue(sheetName, "A6", "Metrik")
    f.SetCellValue(sheetName, "B6", "Nilai")
    f.SetCellStyle(sheetName, "A6", "B6", headerStyle)


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


    f.SetColWidth(sheetName, "A", "A", 25)
    f.SetColWidth(sheetName, "B", "B", 20)
    row += 2
    f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), 
        fmt.Sprintf("Dibuat pada: %s", time.Now().Format("02 January 2006 15:04:05")))


    f.DeleteSheet("Sheet1")
    filename := fmt.Sprintf("Laporan_Kehadiran_%s_%s.xlsx", 
        currentUser.Pkwt.Tad.Nama, bulanTime.Format("2006-01"))

    c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

    if err := f.Write(c.Writer); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat Excel"})
        return
    }
}


func EksporHarianExcel(c *gin.Context) {
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


    f := excelize.NewFile()
    defer f.Close()
    sheetName := "Laporan Harian"
    index, _ := f.NewSheet(sheetName)
    f.SetActiveSheet(index)
    headerStyle, _ := f.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 11},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })

   
    f.SetCellValue(sheetName, "A1", "LAPORAN KEHADIRAN HARIAN")
    f.MergeCell(sheetName, "A1", "G1")
    f.SetCellStyle(sheetName, "A1", "G1", headerStyle)
    f.SetCellValue(sheetName, "A2", fmt.Sprintf("Nama: %s", currentUser.Pkwt.Tad.Nama))
    f.SetCellValue(sheetName, "A3", fmt.Sprintf("Bulan: %s", bulanTime.Format("January 2006")))
    headers := []string{"Tanggal", "Hari", "Jam Masuk", "Jam Keluar", "Jam Kerja", "Status", "Lembur"}
    for i, h := range headers {
        cell := fmt.Sprintf("%s5", string(rune('A'+i)))
        f.SetCellValue(sheetName, cell, h)
        f.SetCellStyle(sheetName, cell, cell, headerStyle)
    }

   
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


func GetLaporanCustomTanggal(c *gin.Context) {
    userData, exists := c.Get("currentUser")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi pengguna tidak valid"})
        return
    }
    currentUser := userData.(models.Penempatan)
    tanggalAwal := c.Query("tanggal_awal") 
    tanggalAkhir := c.Query("tanggal_akhir")     
    if tanggalAwal == "" || tanggalAkhir == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Harap Masukkan Tanggal Awal dan Tanggal Akhir Rekap Absensi!"})
        return
    }

    start, err := time.Parse("2006-01-02", tanggalAwal)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal_awal tidak valid! (gunakan YYYY-MM-DD Contoh 2025-10-21)"})
        return
    }
    end, err := time.Parse("2006-01-02", tanggalAkhir)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal_akhir tidak valid! (gunakan YYYY-MM-DD Contoh 2025-12-21)"})
        return
    }


    if end.Before(start) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Tanggal Akhir harus setelah Tanggal Awal!"})
        return
    }

   
    var attendances []models.Absensi
    err = models.DB.Where("penempatan_id = ? AND tgl_absen BETWEEN ? AND ?",
        currentUser.Id, tanggalAwal, tanggalAkhir).
        Order("tgl_absen ASC").
        Find(&attendances).Error
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data absensi"})
        return
    }

    summary := HitungKehadiranCustomTanggal(attendances, start, end)
    c.JSON(http.StatusOK, gin.H{
        "summary": summary,
        "date_range": gin.H{
            "start": tanggalAwal,
            "end":   tanggalAkhir,
            "days":  int(end.Sub(start).Hours()/24) + 1,
        },
    })
}


func HitungKehadiranCustomTanggal(attendances []models.Absensi, tanggalAwal, tanggalAkhir time.Time) models.LaporanAbsensi {
    summary := models.LaporanAbsensi{
        Bulan: fmt.Sprintf("%s - %s", 
            tanggalAwal.Format("02 Jan 2006"), 
            tanggalAkhir.Format("02 Jan 2006")),
    }

    const standardWorkStart = 8 * 60
    const standardWorkEnd = 17 * 60
    const standardJamKerja = 8.0
    var totalWorkMinutes float64
    var completeDays int

    for _, absen := range attendances {
        summary.TotalHadir++
        checkInTime, _ := time.Parse("15:04:05", absen.Jam_masuk)
        checkInMinutes := checkInTime.Hour()*60 + checkInTime.Minute()   
        if checkInMinutes > standardWorkStart {
            summary.HariTelat++
        }

        if absen.Jam_keluar != nil {
            completeDays++
            checkOutTime, _ := time.Parse("15:04:05", *absen.Jam_keluar)
            checkOutMinutes := checkOutTime.Hour()*60 + checkOutTime.Minute()
            workMinutes := float64(checkOutMinutes - checkInMinutes)
            
            if workMinutes < 0 {
                workMinutes += 24 * 60
            }

            totalWorkMinutes += workMinutes

            if checkOutMinutes < standardWorkEnd {
                summary.CheckoutLebihAwal++
            }

            JamKerja := workMinutes / 60
            if JamKerja > standardJamKerja {
                summary.WaktuLembur += (JamKerja - standardJamKerja)
            }
        }
    }

    summary.TotalJamKerja = totalWorkMinutes / 60
    if completeDays > 0 {
        summary.RataRataJamKerja = summary.TotalJamKerja / float64(completeDays)
    }
    summary.TotalHariKerja = hitungJarakHariKerja(tanggalAwal, tanggalAkhir)
    summary.TotalAbsen = summary.TotalHariKerja - summary.TotalHadir
    
    if summary.TotalHariKerja > 0 {
        summary.PersentaseKehadiran = (float64(summary.TotalHadir) / float64(summary.TotalHariKerja)) * 100
    }

    return summary
}

func hitungJarakHariKerja(tanggalAwal, tanggalAkhir time.Time) int {
    hariKerja := 0
    for d := tanggalAwal; !d.After(tanggalAkhir); d = d.AddDate(0, 0, 1) {
        if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
            hariKerja++
        }
    }
    return hariKerja
}


func EksporCustomTanggalExcel(c *gin.Context) {
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

    tanggalAwal := c.Query("tanggal_awal")
    tanggalAkhir := c.Query("tanggal_akhir")
    if tanggalAwal == "" || tanggalAkhir == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter tanggal_awal dan tanggal_akhir diperlukan"})
        return
    }

    start, err := time.Parse("2006-01-02", tanggalAwal)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid"})
        return
    }

    end, err := time.Parse("2006-01-02", tanggalAkhir)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid"})
        return
    }

    if end.Before(start) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal_akhir harus setelah tanggal_awal"})
        return
    }

    var attendances []models.Absensi
    err = models.DB.Where("penempatan_id = ? AND tgl_absen BETWEEN ? AND ?",
        currentUser.Id, tanggalAwal, tanggalAkhir).
        Order("tgl_absen ASC").
        Find(&attendances).Error

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
        return
    }

    summary := HitungKehadiranCustomTanggal(attendances, start, end)
    f := excelize.NewFile()
    defer f.Close()

    sheetName := "Ringkasan Kehadiran"
    index, _ := f.NewSheet(sheetName)
    f.SetActiveSheet(index)
    headerStyle, _ := f.NewStyle(&excelize.Style{
        Font: &excelize.Font{Bold: true, Size: 12},
        Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
        Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
    })


    f.SetCellValue(sheetName, "A1", "LAPORAN KEHADIRAN - RENTANG TANGGAL")
    f.MergeCell(sheetName, "A1", "B1")
    f.SetCellStyle(sheetName, "A1", "B1", headerStyle)
    f.SetRowHeight(sheetName, 1, 25)


    f.SetCellValue(sheetName, "A3", "Nama:")
    f.SetCellValue(sheetName, "B3", currentUser.Pkwt.Tad.Nama)
    f.SetCellValue(sheetName, "A4", "Periode:")
    f.SetCellValue(sheetName, "B4", summary.Bulan)

 
    f.SetCellValue(sheetName, "A6", "Metrik")
    f.SetCellValue(sheetName, "B6", "Nilai")
    f.SetCellStyle(sheetName, "A6", "B6", headerStyle)

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

    f.SetColWidth(sheetName, "A", "A", 25)
    f.SetColWidth(sheetName, "B", "B", 20)

    row += 2
    f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), 
        fmt.Sprintf("Dibuat pada: %s", time.Now().Format("02 January 2006 15:04:05")))

    f.DeleteSheet("Sheet1")

    filename := fmt.Sprintf("Laporan_Kehadiran_%s_%s_to_%s.xlsx", 
        currentUser.Pkwt.Tad.Nama, 
        start.Format("2006-01-02"), 
        end.Format("2006-01-02"))

    c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

    if err := f.Write(c.Writer); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat Excel"})
        return
    }
}


func EksporCustomTanggalPDF(c *gin.Context) {
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

    tanggalAwal := c.Query("tanggal_awal")
    tanggalAkhir := c.Query("tanggal_akhir")
    if tanggalAwal == "" || tanggalAkhir == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter tanggal_awal dan tanggal_akhir diperlukan"})
        return
    }

    start, err := time.Parse("2006-01-02", tanggalAwal)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid"})
        return
    }

    end, err := time.Parse("2006-01-02", tanggalAkhir)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal tidak valid"})
        return
    }

    if end.Before(start) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "tanggal_akhir harus setelah tanggal_awal"})
        return
    }

    var attendances []models.Absensi
    err = models.DB.Where("penempatan_id = ? AND tgl_absen BETWEEN ? AND ?",
        currentUser.Id, tanggalAwal, tanggalAkhir).
        Order("tgl_absen ASC").
        Find(&attendances).Error
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
        return
    }

    summary := HitungKehadiranCustomTanggal(attendances, start, end)
    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.AddPage()
    pdf.SetFont("Arial", "B", 16)
    pdf.Cell(40, 10, "Laporan Kehadiran")
    pdf.Ln(12)


    pdf.SetFont("Arial", "", 12)
    pdf.Cell(40, 10, fmt.Sprintf("Nama: %s", currentUser.Pkwt.Tad.Nama))
    pdf.Ln(8)
    pdf.Cell(40, 10, fmt.Sprintf("Periode: %s", summary.Bulan))
    pdf.Ln(12)

    pdf.SetFont("Arial", "B", 12)
    pdf.Cell(90, 10, "Metrik")
    pdf.Cell(90, 10, "Nilai")
    pdf.Ln(10)

    pdf.SetFont("Arial", "", 11)
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


    pdf.Ln(10)
    pdf.SetFont("Arial", "I", 9)
    pdf.Cell(0, 10, fmt.Sprintf("Dibuat pada: %s", time.Now().Format("02 January 2006 15:04:05")))
    filename := fmt.Sprintf("Laporan_Kehadiran_%s_%s_to_%s.pdf", 
        currentUser.Pkwt.Tad.Nama, 
        start.Format("2006-01-02"), 
        end.Format("2006-01-02"))

    c.Header("Content-Type", "application/pdf")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
    
    err = pdf.Output(c.Writer)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat PDF"})
        return
    }
}
