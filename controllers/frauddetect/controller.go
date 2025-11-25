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
    Date              string  `json:"date"`
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

