package fraudcontrollers

import (
	"SITEKAD/models"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const urlmodel = "http://localhost:5001"
type FiturData struct {
	PenempatanID      int64   `json:"penempatan_id"`
	Tanggal           string  `json:"tanggal"`
	CheckInLatitude   float64 `json:"checkin_latitude"`
	CheckInLongitude  float64 `json:"checkin_longitude"`
	CheckOutLatitude  float64 `json:"checkout_latitude,omitempty"`
	CheckOutLongitude float64 `json:"checkout_longitude,omitempty"`
}

type Hasildeteksi struct {
	PenempatanID      int64                  `json:"penempatan_id"`
	Tanggal           string                 `json:"tanggal"`
	IsFakeGPS         bool                   `json:"is_fake_gps"`
	SkorAnomali      float64                `json:"skor_anomali"`
	Confidence        float64                `json:"confidence"`
	Severity          string                 `json:"severity"`
	Alasan           []string               `json:"alasan"`
	CheckInLocation   map[string]float64     `json:"checkin_location"`
	CheckOutLocation  map[string]float64     `json:"checkout_location,omitempty"`
	DetailAnomali 	map[string]interface{} `json:"detail_anomali,omitempty"`
}


func parseGpsCoordinates(coordStr string) (lat, long float64) {
	if coordStr == "" {
		return 0, 0
	}
	var parsedLat, parsedLong float64
	_, err := fmt.Sscanf(coordStr, "%f,%f", &parsedLat, &parsedLong)
	if err != nil {
		return 0, 0
	}
	return parsedLat, parsedLong
}

func LatihModelDeteksi(c *gin.Context) {
	startDate := c.DefaultQuery("start_date", time.Now().AddDate(0, -3, 0).Format("2006-01-02"))
	endDate := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))
	var attendances []models.Absensi
	result := models.DB.Where("tgl_absen BETWEEN ? AND ? AND is_deleted = 0", startDate, endDate).
		Preload("Penempatan").
		Order("tgl_absen ASC").
		Find(&attendances)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal mengambil data absensi",
			"details": result.Error.Error(),
		})
		return
	}

	if len(attendances) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Tidak ada data absensi",
			"message": "Tidak ada data untuk periode yang dipilih",
		})
		return
	}

	trainingData := make([]FiturData, 0)
	skippedCount := 0
	for _, absen := range attendances {
		checkInLat, checkInLong := parseGpsCoordinates(absen.Kordmasuk)
		if checkInLat == 0 && checkInLong == 0 {
			skippedCount++
			continue
		}

		gpsData := FiturData{
			PenempatanID:       absen.Penempatan_id,
			Tanggal:             absen.Tgl_absen,
			CheckInLatitude:  checkInLat,
			CheckInLongitude: checkInLong,
		}

		if absen.Kordkeluar != nil && *absen.Kordkeluar != "" {
			checkOutLat, checkOutLong := parseGpsCoordinates(*absen.Kordkeluar)
			if checkOutLat != 0 || checkOutLong != 0 {
				gpsData.CheckOutLatitude = checkOutLat
				gpsData.CheckOutLongitude = checkOutLong
			}
		}

		trainingData = append(trainingData, gpsData)
	}

	if len(trainingData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Tidak ada data GPS yang valid",
			"message": "Semua record tidak memiliki koordinat GPS",
			"skipped": skippedCount,
		})
		return
	}

	requestBody := map[string]interface{}{
		"training_data": trainingData,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal marshal JSON",
			"details": err.Error(),
		})
		return
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/train", urlmodel),
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal mengakses URL Python",
			"details": err.Error(),
			"url":     urlmodel,
		})
		return
	}
	defer resp.Body.Close()
	var pythonResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pythonResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal parsing response Python",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "sukses",
		"message":         "Model deteksi fakeGps berhasil dilatih!",
		"periode":         fmt.Sprintf("%s sampai %s", startDate, endDate),
		"total_data":   len(attendances),
		"data_gps_valid":  len(trainingData),
		"data_tidak_valid": skippedCount,
		"python_response": pythonResponse,
	})
}

func DeteksiFakeGps(c *gin.Context) {
	PenempatanIDStr := c.Query("penempatan_id")
	startDate := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	endDate := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))
	aoEnabled := c.Query("ao_enabled")
	var PenempatanID int64
	if PenempatanIDStr != "" {
		fmt.Sscanf(PenempatanIDStr, "%d", &PenempatanID)
	}

	query := models.DB.Where("tgl_absen BETWEEN ? AND ? AND is_deleted = 0", startDate, endDate)
	if PenempatanID > 0 {
		query = query.Where("penempatan_id = ?", PenempatanID)
	}

	var attendances []models.Absensi
	result := query.Preload("Penempatan").Order("tgl_absen ASC").Find(&attendances)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal mengambil data absensi",
			"details": result.Error.Error(),
		})
		return
	}

	if len(attendances) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Tidak ada data",
			"message": "Tidak ada data absensi untuk parameter yang diberikan",
		})
		return
	}

	gpsData := make([]FiturData, 0)
	for _, absen := range attendances {
		checkInLat, checkInLong := parseGpsCoordinates(absen.Kordmasuk)
		if checkInLat == 0 && checkInLong == 0 {
			continue
		}

		data := FiturData{
			PenempatanID:       absen.Penempatan_id,
			Tanggal:             absen.Tgl_absen,
			CheckInLatitude:  checkInLat,
			CheckInLongitude: checkInLong,
		}

		if absen.Kordkeluar != nil && *absen.Kordkeluar != "" {
			checkOutLat, checkOutLong := parseGpsCoordinates(*absen.Kordkeluar)
			if checkOutLat != 0 || checkOutLong != 0 {
				data.CheckOutLatitude = checkOutLat
				data.CheckOutLongitude = checkOutLong
			}
		}
		gpsData = append(gpsData, data)
	}

	if len(gpsData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Tidak ada data Gps yang valid",
			"message": "Semua record tidak memiliki koordinat Gps",
		})
		return
	}

	jsonData, err := json.Marshal(gpsData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal marshal JSON",
			"details": err.Error(),
		})
		return
	}

	
	resp, err := http.Post(
		fmt.Sprintf("%s/predict?ao_enabled=%s", urlmodel, aoEnabled),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal mengakses URL Python",
			"details": err.Error(),
			"url":     urlmodel,
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResponse)
		c.JSON(resp.StatusCode, gin.H{
			"error":           "Python service error",
			"python_response": errorResponse,
		})
		return
	}

	var detectionResults []Hasildeteksi
	if err := json.NewDecoder(resp.Body).Decode(&detectionResults); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal parsing response Python",
			"details": err.Error(),
		})
		return
	}

	fakeGPSCount := 0
	for _, result := range detectionResults {
		if result.IsFakeGPS {
			fakeGPSCount++
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":            "sukses",
		"total_checked":     len(detectionResults),
		"terdeteksi_menggunakan_fakeGps": fakeGPSCount,
		"clean_records":     len(detectionResults) - fakeGPSCount,
		"results":           detectionResults,
	})
}


func GetFakeGpsDashboard(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	var days int
	fmt.Sscanf(daysStr, "%d", &days)
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	var attendances []models.Absensi
	result := models.DB.Where("tgl_absen BETWEEN ? AND ? AND is_deleted = 0", startDate, endDate).
		Preload("Penempatan").
		Find(&attendances)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Gagal mengambil data",
			"details": result.Error.Error(),
		})
		return
	}

	gpsData := make([]FiturData, 0)
	for _, absen := range attendances {
		checkInLat, checkInLong := parseGpsCoordinates(absen.Kordmasuk)
		if checkInLat == 0 && checkInLong == 0 {
			continue
		}

		data := FiturData{
			PenempatanID:       absen.Penempatan_id,
			Tanggal:             absen.Tgl_absen,
			CheckInLatitude:  checkInLat,
			CheckInLongitude: checkInLong,
		}

		if absen.Kordkeluar != nil && *absen.Kordkeluar != "" {
			checkOutLat, checkOutLong := parseGpsCoordinates(*absen.Kordkeluar)
			data.CheckOutLatitude = checkOutLat
			data.CheckOutLongitude = checkOutLong
		}
		gpsData = append(gpsData, data)
	}

	if len(gpsData) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":            "sukses",
			"total_records":     0,
			"terdeteksi_menggunakan_fakeGps": 0,
			"message":           "Tidak ada data GPS untuk dianalisis",
		})
		return
	}

	jsonData, _ := json.Marshal(gpsData)
	resp, err := http.Post(
		fmt.Sprintf("%s/predict", urlmodel),
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Layanan Python tidak tersedia",
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResponse)
		c.JSON(http.StatusOK, gin.H{
			"status":            "error",
			"total_records":     0,
			"terdeteksi_menggunakan_fakeGps": 0,
			"python_error":      errorResponse,
		})
		return
	}

	var detectionResults []Hasildeteksi
	if err := json.NewDecoder(resp.Body).Decode(&detectionResults); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":            "error",
			"total_records":     0,
			"terdeteksi_menggunakan_fakeGps": 0,
			"message":           "Failed to parse Python response",
		})
		return
	}

	stats := map[string]interface{}{
		"total_records":     len(detectionResults),
		"terdeteksi_menggunakan_fakeGps": 0,
		"clean_records":     0,
		"by_severity": map[string]int{
			"critical": 0,
			"high":     0,
			"medium":   0,
			"low":      0,
		},
		"by_employee": make(map[int64]int),
	}

	for _, result := range detectionResults {
		if result.IsFakeGPS {
			stats["terdeteksi_menggunakan_fakeGps"] = stats["terdeteksi_menggunakan_fakeGps"].(int) + 1
			severityMap := stats["by_severity"].(map[string]int)
			severityMap[result.Severity]++
			employeeMap := stats["by_employee"].(map[int64]int)
			employeeMap[result.PenempatanID]++
		} else {
			stats["clean_records"] = stats["clean_records"].(int) + 1
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "sukses",
		"periode":   fmt.Sprintf("%s sampai %s", startDate, endDate),
		"dashboard": stats,
	})
}

func CekStatusModel(c *gin.Context) {
	resp, err := http.Get(fmt.Sprintf("%s/health", urlmodel))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "Layanan Python fake GPS tidak tersedia",
			"url":     urlmodel,
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	var healthResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&healthResponse)

	c.JSON(http.StatusOK, gin.H{
		"status":         "sehat",
		"python_service": healthResponse,
		"service_url":    urlmodel,
	})
}
