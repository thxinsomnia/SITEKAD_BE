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

