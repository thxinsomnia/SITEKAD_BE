package helper

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"SITEKAD/models"

	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/linear_models"
)

func timeToMinutes(timeStr string) float64 {
	parts := strings.Split(timeStr, ":")
	if len(parts) < 2 {
		return 0
	}
	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	return float64(hours*60 + minutes)
}

func minutesToTime(minutes float64) string {
	hours := int(minutes / 60)
	mins := int(minutes) % 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}

func PredictCheckoutTime(history [][2]string, newCheckInTime string) (string, error) {
	if len(history) == 0 {
		return "", fmt.Errorf("no training data available")
	}

	// Build CSV string from history data
	var csvBuffer bytes.Buffer
	csvBuffer.WriteString("jam_keluar,jam_masuk\n")

	for _, record := range history {
		checkInMinutes := timeToMinutes(record[0])
		checkOutMinutes := timeToMinutes(record[1])
		csvBuffer.WriteString(fmt.Sprintf("%.2f,%.2f\n", checkOutMinutes, checkInMinutes))
	}

	// Parse the CSV data
	instances, err := base.ParseCSVToInstances(csvBuffer.String(), true)
	if err != nil {
		return "", fmt.Errorf("failed to parse training data: %w", err)
	}

	// Train the model
	model := linear_models.NewLinearRegression()
	if err := model.Fit(instances); err != nil {
		return "", fmt.Errorf("failed to train model: %w", err)
	}

	// Prepare prediction data
	newCheckInMinutes := timeToMinutes(newCheckInTime)
	predCSV := fmt.Sprintf("jam_keluar,jam_masuk\n0.0,%.2f\n", newCheckInMinutes)

	predInstances, err := base.ParseCSVToInstances(predCSV, true)
	if err != nil {
		return "", fmt.Errorf("failed to parse prediction data: %w", err)
	}

	// Predict
	predictions, err := model.Predict(predInstances)
	if err != nil {
		return "", fmt.Errorf("prediction failed: %w", err)
	}

	// Extract prediction from the first row's class attribute
	classAttrs := predictions.AllClassAttributes()
	if len(classAttrs) == 0 {
		return "", fmt.Errorf("no class attribute in predictions")
	}

	classSpec := base.ResolveAttributes(predictions, classAttrs)[0]
	predictedBytes := predictions.Get(classSpec, 0)
	predictedMinutes := base.UnpackBytesToFloat(predictedBytes)
	predictedTime := minutesToTime(predictedMinutes)

	return predictedTime, nil
}

func GetTrainingDataForUser(userID int64) ([][2]string, error) {
	var recentAbsensi []models.Absensi

	err := models.DB.Where(
		"penempatan_id = ? AND jam_keluar IS NOT NULL",
		userID,
	).Order("id desc").Limit(10).Find(&recentAbsensi).Error

	if err != nil {
		return nil, err
	}

	var historyData [][2]string
	for _, absen := range recentAbsensi {
		if absen.Jam_keluar != nil {
			historyData = append(historyData, [2]string{absen.Jam_masuk, *absen.Jam_keluar})
		}
	}

	return historyData, nil
}
