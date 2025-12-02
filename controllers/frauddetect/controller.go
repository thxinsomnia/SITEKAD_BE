package fraudcontrollers

import (
    "SITEKAD/models"
    "bytes"
    "encoding/json"
    "fmt"
    "math"
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
)

const ML_SERVICE_URL = "http://localhost:5000"

// Feature vector for anomaly detection
type AttendanceFeatures struct {
    EmployeeID        int64    `json:"employee_id"`
    EmployeeName      string  `json:"employee_name"`
    Date              string  `gorm:"type:date" json:"date"`
    CheckInHour       float64 `json:"checkin_hour"`
    CheckOutHour      float64 `json:"checkout_hour"`
    TotalWorkHours    float64 `json:"total_work_hours"`
    IsWeekend         int     `json:"is_weekend"`
    DayOfWeek         int     `json:"day_of_week"`
    IsMonday          int     `json:"is_monday"`
    AvgCheckInHour    float64 `json:"avg_checkin_hour"`
    StdCheckInHour    float64 `json:"std_checkin_hour"`
    AvgWorkHours      float64 `json:"avg_work_hours"`
    DaysWorked        int     `json:"days_worked_30d"`
    CheckInDeviation  float64 `json:"checkin_deviation"`
    WorkHourDeviation float64 `json:"workhour_deviation"`
    DistanceFromOffice float64 `json:"distance_km"`
}

type AnomalyDetectionResult struct {
    EmployeeID   int64     `json:"employee_id"`
    EmployeeName string   `json:"employee_name"`
    Date         string   `json:"date"`
    IsAnomaly    bool     `json:"is_anomaly"`
    AnomalyScore float64  `json:"anomaly_score"`
    Confidence   float64  `json:"confidence"`
    Severity     string   `json:"severity"`
    Reasons      []string `json:"reasons"`
}

// Helper: Convert time string to hours (8:30:00 -> 8.5)
func timeToHours(timeStr string) float64 {
    if timeStr == "" {
        return 0
    }
    t, err := time.Parse("15:04:05", timeStr)
    if err != nil {
        return 0
    }
    return float64(t.Hour()) + float64(t.Minute())/60.0
}

// Helper: Bool to int
func boolToInt(b bool) int {
    if b {
        return 1
    }
    return 0
}

// Calculate check-in statistics (mean and std deviation)
func calculateCheckInStats(attendances []models.Absensi) (float64, float64) {
    if len(attendances) == 0 {
        return 8.0, 0.5
    }

    var sum, sumSquares float64
    for _, a := range attendances {
        hour := timeToHours(a.Jam_masuk)
        sum += hour
        sumSquares += hour * hour
    }

    n := float64(len(attendances))
    mean := sum / n
    variance := (sumSquares / n) - (mean * mean)
    if variance < 0 {
        variance = 0
    }
    std := math.Sqrt(variance)

    return mean, std
}

// Calculate average work hours
func calculateAvgWorkHours(attendances []models.Absensi) float64 {
    if len(attendances) == 0 {
        return 8.0
    }

    var totalHours float64
    count := 0

    for _, a := range attendances {
        if a.Jam_keluar != nil && *a.Jam_keluar != "" {
            checkIn := timeToHours(a.Jam_masuk)
            checkOut := timeToHours(*a.Jam_keluar)
            hours := checkOut - checkIn
            if hours < 0 {
                hours += 24
            }
            totalHours += hours
            count++
        }
    }

    if count == 0 {
        return 8.0
    }
    return totalHours / float64(count)
}

// Extract features from attendance data using GORM
func ExtractAttendanceFeatures(penempatanID int64, date string) AttendanceFeatures {
    var absen models.Absensi
    result := models.DB.Where("penempatan_id = ? AND tgl_absen = ?", penempatanID, date).
        First(&absen)

    if result.Error != nil {
        return AttendanceFeatures{
            EmployeeID: penempatanID,
            Date:       date,
        }
    }

    checkInHour := timeToHours(absen.Jam_masuk)
    checkOutHour := 0.0
    totalWorkHours := 0.0

    if absen.Jam_keluar != nil && *absen.Jam_keluar != "" {
        checkOutHour = timeToHours(*absen.Jam_keluar)
        totalWorkHours = checkOutHour - checkInHour
        if totalWorkHours < 0 {
            totalWorkHours += 24
        }
    }

    // FIX: Normalize date format first
    var dateTime time.Time
    var dateStr string
    
    // Try to parse the date in multiple formats
    formats := []string{
        "2006-01-02",
        "2006-01-02T15:04:05Z07:00",
        "2006-01-02T15:04:05Z",
        "2006-01-02 15:04:05",
    }
    
    parsed := false
    for _, format := range formats {
        if dt, err := time.Parse(format, date); err == nil {
            dateTime = dt
            dateStr = dt.Format("2006-01-02") // Normalize to date-only format
            parsed = true
            break
        }
    }
    
    // If parsing fails, try extracting just the date part
    if !parsed && len(date) >= 10 {
        if dt, err := time.Parse("2006-01-02", date[:10]); err == nil {
            dateTime = dt
            dateStr = date[:10]
            parsed = true
        }
    }
    
    // Last resort
    if !parsed {
        dateTime = time.Now()
        dateStr = dateTime.Format("2006-01-02")
    }
    
    // Calculate 30 days ago in same format
    thirtyDaysAgo := dateTime.AddDate(0, 0, -30).Format("2006-01-02")

    // FIX: Use DATE() function for proper comparison with consistent format
    var historicalData []models.Absensi
    models.DB.Where("penempatan_id = ? AND tgl_absen >= ? AND tgl_absen < ?",
        penempatanID, thirtyDaysAgo, dateStr).
        Order("tgl_absen DESC").
        Limit(30). // Add limit to prevent huge result sets
        Find(&historicalData)

    avgCheckIn, stdCheckIn := calculateCheckInStats(historicalData)
    avgWorkHours := calculateAvgWorkHours(historicalData)

    // parsedDate, _ := time.Parse("2006-01-02", date)

    features := AttendanceFeatures{
        EmployeeID:        penempatanID,
        Date:              date,
        CheckInHour:       checkInHour,
        CheckOutHour:      checkOutHour,
        TotalWorkHours:    totalWorkHours,
        IsWeekend:         boolToInt(dateTime.Weekday() == time.Saturday || dateTime.Weekday() == time.Sunday),
        DayOfWeek:         int(dateTime.Weekday()),
        IsMonday:          boolToInt(dateTime.Weekday() == time.Monday),
        AvgCheckInHour:    avgCheckIn,
        StdCheckInHour:    stdCheckIn,
        AvgWorkHours:      avgWorkHours,
        DaysWorked:        len(historicalData),
        CheckInDeviation:  checkInHour - avgCheckIn,
        WorkHourDeviation: totalWorkHours - avgWorkHours,
        DistanceFromOffice: 0,
    }

    return features
}

// Train model with historical data
func TrainAnomalyModel(c *gin.Context) {
    startDate := c.DefaultQuery("start_date", time.Now().AddDate(0, -6, 0).Format("2006-01-02"))
    endDate := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

    var attendances []models.Absensi
    result := models.DB.Where("tgl_absen BETWEEN ? AND ?", startDate, endDate).
        Find(&attendances)

    if result.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Gagal Melakukan Fetch Data Absensi!",
        })
        return
    }

    if len(attendances) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Tidak ada data absensi yang ditemukan dalam rentang tanggal tersebut.",
        })
        return
    }

    var trainingData []AttendanceFeatures
    processedDates := make(map[string]map[int64]bool)

    for _, absen := range attendances {  
        if processedDates[absen.Tgl_absen] == nil {
            processedDates[absen.Tgl_absen] = make(map[int64]bool)
        }

        if !processedDates[absen.Tgl_absen][absen.Penempatan_id] {
            features := ExtractAttendanceFeatures(absen.Penempatan_id, absen.Tgl_absen)
            trainingData = append(trainingData, features)
            processedDates[absen.Tgl_absen][absen.Penempatan_id] = true
        }
    }

    payload := map[string]interface{}{
        "training_data": trainingData,
    }

    jsonData, _ := json.Marshal(payload)
    resp, err := http.Post(ML_SERVICE_URL+"/train", "application/json", bytes.NewBuffer(jsonData))

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Layanan Servis Sedang tidak tersedia",
            "details": err.Error(),
        })
        return
    }
    defer resp.Body.Close()

    var mlResult map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&mlResult)

    c.JSON(http.StatusOK, gin.H{
        "message":          "Model Berhasil Dilatih!",
        "training_samples": len(trainingData),
        "date_range": gin.H{
            "start": startDate,
            "end":   endDate,
        },
        "ml_response": mlResult,
    })
}

// Detect anomalies for a specific date
func DetectAttendanceAnomalies(c *gin.Context) {
    date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
    cabangID := c.Query("cabang_id")
    lokasiID := c.Query("lokasi_id")

    query := models.DB.Preload("Penempatan.Pkwt.Tad").
        Preload("Penempatan.Pkwt.Jabatan").
        Where("tgl_absen = ?", date)

    if cabangID != "" {
        query = query.Joins("JOIN penempatan ON penempatan.id = absensi.penempatan_id").
            Where("penempatan.cabang_id = ?", cabangID)
    }

    if lokasiID != "" {
        query = query.Joins("JOIN penempatan ON penempatan.id = absensi.penempatan_id").
            Where("penempatan.lokasi_kerja_id = ?", lokasiID)
    }

    var attendances []models.Absensi
    result := query.Find(&attendances)

    if result.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Gagal mengambil data absensi!",
        })
        return
    }

    if len(attendances) == 0 {
        c.JSON(http.StatusOK, gin.H{
            "message":   "Tidak ada data absensi yang ditemukan untuk tanggal: " + date,
            "anomalies": []AnomalyDetectionResult{},
        })
        return
    }

    var features []AttendanceFeatures
    employeeMap := make(map[int64]string)

    for _, absen := range attendances {
        feature := ExtractAttendanceFeatures(absen.Penempatan_id, date)

        if absen.Penempatan.Pkwt.Tad.Nama != "" {
            feature.EmployeeName = absen.Penempatan.Pkwt.Tad.Nama
            employeeMap[absen.Penempatan_id] = absen.Penempatan.Pkwt.Tad.Nama
        }

        features = append(features, feature)
    }

    jsonData, _ := json.Marshal(features)
    resp, err := http.Post(ML_SERVICE_URL+"/predict", "application/json", bytes.NewBuffer(jsonData))

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Layanan Servis Sedang tidak tersedia!",
            "details": err.Error(),
        })
        return
    }
    defer resp.Body.Close()

    var predictions []AnomalyDetectionResult
    if err := json.NewDecoder(resp.Body).Decode(&predictions); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Gagal memproses respons dari Servis!",
        })
        return
    }

    for i := range predictions {
        if name, ok := employeeMap[predictions[i].EmployeeID]; ok {
            predictions[i].EmployeeName = name
        }
    }

    anomalies := []AnomalyDetectionResult{}
    for _, pred := range predictions {
        if pred.IsAnomaly {
            anomalies = append(anomalies, pred)
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "date":            date,
        "total_checked":   len(predictions),
        "total_anomalies": len(anomalies),
        "anomaly_rate":    fmt.Sprintf("%.2f%%", float64(len(anomalies))/float64(len(predictions))*100),
        "anomalies":       anomalies,
    })
}

// Get anomaly history for an employee
func GetEmployeeAnomalyHistory(c *gin.Context) {
    employeeIDStr := c.Param("id")
    employeeID, err := strconv.ParseUint(employeeIDStr, 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "NITAD tidak valid",
        })
        return
    }

    days := c.DefaultQuery("days", "30")
    daysInt, _ := strconv.Atoi(days)

    endDate := time.Now().Format("2006-01-02")
    startDate := time.Now().AddDate(0, 0, -daysInt).Format("2006-01-02")

    var penempatan models.Penempatan
    if err := models.DB.Preload("Pkwt.Tad").
        Preload("Pkwt.Jabatan").
        First(&penempatan, int64(employeeID)).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Pegawai tidak ditemukan!",
        })
        return
    }

    var attendances []models.Absensi
    result := models.DB.Where("penempatan_id = ? AND tgl_absen BETWEEN ? AND ?",
        employeeID, startDate, endDate).
        Order("tgl_absen DESC").
        Find(&attendances)

    if result.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Gagal mengambil data absensi!",
        })
        return
    }

    if len(attendances) == 0 {
        c.JSON(http.StatusOK, gin.H{
            "employee": gin.H{
                "id":      penempatan.Id,
                "name":    penempatan.Pkwt.Tad.Nama,
                "jabatan": penempatan.Pkwt.Jabatan.Nama,
            },
            "message": "Tidak ada data absensi ditemukan untuk pegawai ini dalam rentang tanggal tersebut.",
            "history": []AnomalyDetectionResult{},
        })
        return
    }

    var features []AttendanceFeatures
    for _, absen := range attendances {
        feature := ExtractAttendanceFeatures(int64(employeeID), absen.Tgl_absen)
        feature.EmployeeName = penempatan.Pkwt.Tad.Nama
        features = append(features, feature)
    }

    jsonData, _ := json.Marshal(features)
    resp, err := http.Post(ML_SERVICE_URL+"/predict", "application/json", bytes.NewBuffer(jsonData))

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Layanan Servis Sedang tidak tersedia!",
        })
        return
    }
    defer resp.Body.Close()

    var predictions []AnomalyDetectionResult
    json.NewDecoder(resp.Body).Decode(&predictions)

    anomalyCount := 0
    for _, pred := range predictions {
        if pred.IsAnomaly {
            anomalyCount++
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "employee": gin.H{
            "id":      penempatan.Id,
            "name":    penempatan.Pkwt.Tad.Nama,
            "jabatan": penempatan.Pkwt.Jabatan.Nama,
        },
        "period": gin.H{
            "start": startDate,
            "end":   endDate,
            "days":  len(predictions),
        },
        "summary": gin.H{
            "total_days":   len(predictions),
            "anomalies":    anomalyCount,
            "anomaly_rate": fmt.Sprintf("%.1f%%", float64(anomalyCount)/float64(len(predictions))*100),
        },
        "history": predictions,
    })
}

// Get anomaly dashboard statistics
func GetAnomalyDashboard(c *gin.Context) {
    days := c.DefaultQuery("days", "7")
    daysInt, _ := strconv.Atoi(days)

    endDate := time.Now().Format("2006-01-02")
    startDate := time.Now().AddDate(0, 0, -daysInt).Format("2006-01-02")

    var totalRecords int64
    models.DB.Model(&models.Absensi{}).
        Where("tgl_absen BETWEEN ? AND ?", startDate, endDate).
        Count(&totalRecords)

    var uniqueEmployees int64
    models.DB.Model(&models.Absensi{}).
        Where("tgl_absen BETWEEN ? AND ?", startDate, endDate).
        Distinct("penempatan_id").
        Count(&uniqueEmployees)

    c.JSON(http.StatusOK, gin.H{
        "period": gin.H{
            "start": startDate,
            "end":   endDate,
            "days":  daysInt,
        },
        "statistics": gin.H{
            "total_records":    totalRecords,
            "unique_employees": uniqueEmployees,
        },
        "message": "Use /detect endpoint to analyze specific dates",
    })
}