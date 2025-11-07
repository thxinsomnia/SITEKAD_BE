package models


type LaporanAbsensi struct {
    Bulan            string  `json:"bulan"`
    TotalHariKerja   int     `json:"total_hari_kerja"`
    TotalHadir       int     `json:"total_hadir"`
    TotalAbsen       int     `json:"total_absen"`
    HariTelat       int     `json:"hari_telat"`
    CheckoutLebihAwal   int     `json:"checkout_lebih_awal"`
    TotalJamKerja   float64 `json:"total_jam_kerja"`
    RataRataJamKerja float64 `json:"rata_rata_jam_kerja"`
    WaktuLembur    float64 `json:"waktu_lembur"`
    PersentaseKehadiran   float64 `json:"persentase_kehadiran"` 
}

