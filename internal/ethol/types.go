package ethol

type Student struct {
	Number int    `json:"nomor"`
	Name   string `json:"nama"`
}
type Course struct {
	Number  int `json:"nomor"`
	Origin  int `json:"kuliah_asal"`
	Schema  int `json:"jenisSchema"`
	Subject struct {
		Name string `json:"nama"`
	} `json:"matakuliah"`
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
