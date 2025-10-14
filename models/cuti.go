package models

import "time"

type Cuti struct {
	Id	int64	`gorm:"primaryKey" json:"id"`
	Nitad	string	`gorm:"type varchar(200)" json:"nitad"`
	TanggalPengajuan	string	`gorm:"type timestamp" json:"tanggal_pengajuan"`
	TanggalAwal	string	`gorm:"type date" json:"tanggal_awal"`
	TanggalAkhir	string	`gorm:"type date" json:"tanggal_akhir"`
	Alasan	string	`gorm:"type text" json:"alasan"`
	Keterangan	string	`gorm:"type text" json:"keterangan"`
	Gambar	string	`gorm:"type varchar(200)" json:"gambar"`
	CreatedAt	time.Time	`gorm:"type timestamp" json:"created_at"`
	IsDeleted	int8	`gorm:"type tinyint" json:"is_deleted"`
}

func (Cuti) TableName() string {
	return "cuti"
}