package ethol

import "strings"

type Student struct {
	Number int    `json:"nomor"`
	Name   string `json:"nama"`
}

type Course struct {
	Number    int    `json:"nomor"`
	Origin    int    `json:"kuliah_asal"`
	Schema    int    `json:"jenisSchema"`
	Dosen     string `json:"dosen"`
	GelarDpn  string `json:"gelar_dpn"`
	GelarBlk  string `json:"gelar_blk"`
	KodeKelas string `json:"kode_kelas"`
	Pararel   string `json:"pararel"`
	Subject   struct {
		Name string `json:"nama"`
	} `json:"matakuliah"`
}

func (c Course) LecturerName() string {
	name := strings.TrimSpace(c.Dosen)
	if name == "" {
		return ""
	}
	if c.GelarDpn != "" {
		name = c.GelarDpn + ". " + name
	}
	if c.GelarBlk != "" {
		name = name + ", " + c.GelarBlk
	}
	return name
}

type ClassTime struct {
	Course int    `json:"kuliah"`
	Day    int    `json:"nomor_hari"`
	Start  string `json:"jam_awal"`
	End    string `json:"jam_akhir"`
}

type Notification struct {
	ID     string `json:"idNotifikasi"`
	Status string `json:"status"`
	Data   string `json:"dataTerkait"`
}

type activePresence struct {
	Key  string `json:"key"`
	Open int    `json:"open"`
}
