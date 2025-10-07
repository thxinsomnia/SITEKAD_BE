package models

import "time"

type PengerjaanTugas struct {
	Ptid         int64  `gorm:"primaryKey" json:"ptid"`
	PenempatanId int64  `gorm:"type bigint" json:"penempatan_id"`
	WaktuMulai   time.Time `gorm:"type timestamp" json:"waktu_mulai"`
	WaktuSelesai time.Time `gorm:"type timestamp" json:"waktu_selesai"`
	Status       string    `gorm:"type varchar(50)" json:"status"`
}

func (PengerjaanTugas) TableName() string {
	return "pengerjaan_tugas"
}
