package models

import "time"

type CleaningService struct {
    Ccid        int64  `gorm:"primaryKey" json:"ccid"`
    Ptid        int64  `gorm:"type:bigint" json:"ptid"`
    Cid         int64  `gorm:"type:bigint" json:"cid"`
    WaktuScan   string `gorm:"type:timestamp" json:"waktu_scan"`
    FotoSebelum string `gorm:"type:text" json:"foto_sebelum"`
    FotoSesudah string `gorm:"type:text" json:"foto_sesudah"`
    CreatedAt   time.Time `gorm:"type:timestamp" json:"created_at"`
    
    PengerjaanTugas PengerjaanTugas `gorm:"foreignKey:Ptid" json:"-"`
    Checkpoint      Checkpoint      `gorm:"foreignKey:Cid" json:"-"`
}

func (CleaningService) TableName() string {
    return "cc"
}