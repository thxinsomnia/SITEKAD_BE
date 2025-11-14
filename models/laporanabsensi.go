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

type LaporanAbsensiHarian struct {
    Tanggal     string  `json:"date"`
    NamaHari    string  `json:"day_name"`
    JamMasuk    string  `json:"check_in"`
    JamKeluar   *string `json:"check_out"`
    JamKerja    float64 `json:"work_hours"`
    Status      string  `json:"status"` 
    IsLembur   bool    `json:"is_overtime"`
    JamLembur float64 `json:"overtime_hours"`
}

type RekapPegawai struct {
    Nama                string  `json:"nama"`
    Jabatan             string  `json:"jabatan"`
    TotalHariKerja      int     `json:"total_hari_kerja"`
    TotalHadir          int     `json:"total_hadir"`
    TotalAbsen          int     `json:"total_absen"`
    HariTelat           int     `json:"hari_telat"`
    TotalJamKerja       float64 `json:"total_jam_kerja"`
    RataRataJamKerja    float64 `json:"rata_rata_jam_kerja"`
    PersentaseKehadiran float64 `json:"persentase_kehadiran"`
}