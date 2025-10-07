package models

type CekTugas struct {
	Ctid	int64 `gorm:"primaryKey" json:"ctid"`
	Ptid	int64 `gorm:"type bigint" json:"ptid"`
	Cid	int64 `gorm:"type bigint" json:"cid"`
	WaktuScan	string `gorm:"type timestamp" json:"waktu_scan"`
}

func (CekTugas) TableName() string {
	return "cek_tugas"
}