package ethol

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type ScheduleItem struct {
	Hari       string `json:"hari"`
	JamAwal    string `json:"jam_awal"`
	JamAkhir   string `json:"jam_akhir"`
	Matakuliah any    `json:"matakuliah"`
	Dosen      string `json:"dosen"`
	Ruang      string `json:"ruang"`
	Nomor      int    `json:"nomor"`
	Kuliah     int    `json:"kuliah"`
}

func (s ScheduleItem) CourseName() string {
	if name := parseCourseName(s.Matakuliah); name != "" {
		return name
	}
	if s.Kuliah > 0 {
		return fmt.Sprintf("Kuliah #%d", s.Kuliah)
	}
	return ""
}

func (s ScheduleItem) DayValue() int {
	h := strings.ToLower(strings.TrimSpace(s.Hari))
	switch {
	case strings.HasPrefix(h, "senin"):
		return 1
	case strings.HasPrefix(h, "selasa"):
		return 2
	case strings.HasPrefix(h, "rabu"):
		return 3
	case strings.HasPrefix(h, "kamis"):
		return 4
	case strings.HasPrefix(h, "jum"):
		return 5
	case strings.HasPrefix(h, "sabtu"):
		return 6
	case strings.HasPrefix(h, "minggu") || strings.HasPrefix(h, "ahad"):
		return 7
	default:
		return 8
	}
}

type TaskItem struct {
	Title      string `json:"title"`
	Judul      string `json:"judul"`
	Deadline   string `json:"deadline"`
	DeadlineID string `json:"deadline_indonesia"`
	KuliahID   int    `json:"kuliah_id"`
	CourseName string `json:"matkul"`
}

func (t TaskItem) TaskTitle() string {
	if t.Judul != "" {
		return t.Judul
	}
	if t.Title != "" {
		return t.Title
	}
	return "Tugas Perkuliahan"
}

type ExamInfo struct {
	Nomor   int    `json:"nomor"`
	Mulai   string `json:"mulai"`
	Selesai string `json:"selesai"`
}

type ExamItem struct {
	Nomor      int      `json:"nomor"`
	KodeKelas  string   `json:"kode_kelas"`
	Matakuliah any      `json:"matakuliah"`
	Dosen      any      `json:"dosen"`
	Ruang      string   `json:"ruang"`
	Jenis      string   `json:"jenis"`
	Ujian      ExamInfo `json:"ujian"`
}

func (e ExamItem) CourseName() string {
	return parseCourseName(e.Matakuliah)
}

func (e ExamItem) DosenName() string {
	if s, ok := e.Dosen.(string); ok && s != "" {
		return s
	}
	if m, ok := e.Dosen.(map[string]any); ok {
		if name, ok := m["nama"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}

type MaterialItem struct {
	Judul   string `json:"judul"`
	Desk    string `json:"deskripsi"`
	URL     string `json:"url"`
	Tanggal string `json:"tgl"`
	File    string `json:"file"`
}

type AttendanceStats struct {
	TotalSesi int     `json:"totalSesi"`
	RataHadir float64 `json:"rataHadir"`
}

type RosterStudent struct {
	NRP  string `json:"nrp"`
	Nama string `json:"nama"`
}

type RosterInfo struct {
	Course    Course
	Key       string
	Attendees []RosterStudent
	Absentees []RosterStudent
}

func parseCourseName(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	if m, ok := v.(map[string]any); ok {
		if name, ok := m["nama"].(string); ok && name != "" {
			return name
		}
		if name, ok := m["matakuliah"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}

// GetOnlineSchedule fetches the rich weekly timetable from /api/jadwal/jadwal-online.
func (c *Client) GetOnlineSchedule(ctx context.Context) ([]ScheduleItem, error) {
	tahun, semester, err := c.ActivePeriod(ctx)
	if err != nil {
		return nil, err
	}

	var items []ScheduleItem
	endpoint := fmt.Sprintf("/api/jadwal/jadwal-online?tahun=%d&semester=%d", tahun, semester)
	if err := c.getJSON(ctx, endpoint, nil, &items); err != nil {
		return nil, err
	}

	sort.SliceStable(items, func(i, j int) bool {
		di, dj := items[i].DayValue(), items[j].DayValue()
		if di != dj {
			return di < dj
		}
		return items[i].JamAwal < items[j].JamAwal
	})

	return items, nil
}

// GetTasks fetches pending assignments across all currently enrolled courses.
func (c *Client) GetTasks(ctx context.Context) ([]TaskItem, error) {
	courses, err := c.Courses(ctx)
	if err != nil {
		return nil, err
	}

	var allTasks []TaskItem
	for _, course := range courses {
		var tasks []TaskItem
		endpoint := fmt.Sprintf("/api/tugas?kuliah=%d&jenisSchema=%d", course.Number, course.Schema)
		if err := c.getJSON(ctx, endpoint, nil, &tasks); err != nil {
			continue
		}
		cName := course.Subject.Name
		if cName == "" {
			cName = fmt.Sprintf("Kuliah #%d", course.Number)
		}
		for i := range tasks {
			if tasks[i].CourseName == "" {
				tasks[i].CourseName = cName
			}
			tasks[i].KuliahID = course.Number
			allTasks = append(allTasks, tasks[i])
		}
	}

	sort.SliceStable(allTasks, func(i, j int) bool {
		return allTasks[i].Deadline < allTasks[j].Deadline
	})

	return allTasks, nil
}

// GetExams fetches midterm (UTS) and final (UAS) timetables.
func (c *Client) GetExams(ctx context.Context) ([]ExamItem, error) {
	tahun, semester, err := c.ActivePeriod(ctx)
	if err != nil {
		return nil, err
	}

	var allExams []ExamItem
	for _, jenis := range []struct {
		id   int
		name string
	}{{1, "UTS"}, {2, "UAS"}} {
		var exams []ExamItem
		endpoint := fmt.Sprintf("/api/ujian/daftar-ujian?tahun=%d&semester=%d&jenis=%d", tahun, semester, jenis.id)
		if err := c.getJSON(ctx, endpoint, nil, &exams); err != nil {
			continue
		}
		for i := range exams {
			exams[i].Jenis = jenis.name
			allExams = append(allExams, exams[i])
		}
	}

	sort.SliceStable(allExams, func(i, j int) bool {
		return allExams[i].Ujian.Mulai < allExams[j].Ujian.Mulai
	})

	return allExams, nil
}

// GetMaterials fetches uploaded lecture files and videos for a course.
func (c *Client) GetMaterials(ctx context.Context, course Course) ([]MaterialItem, []MaterialItem, error) {
	var materials []MaterialItem
	materiPath := fmt.Sprintf("/api/materi?matakuliah=%d&jenis_schema=%d", course.Number, course.Schema)
	_ = c.getJSON(ctx, materiPath, nil, &materials)

	var videos []MaterialItem
	videoPath := fmt.Sprintf("/api/video?kuliah=%d&jenis_schema=%d", course.Number, course.Schema)
	_ = c.getJSON(ctx, videoPath, nil, &videos)

	return materials, videos, nil
}

// GetAttendanceStats fetches official student attendance percentage and session counts.
func (c *Client) GetAttendanceStats(ctx context.Context) (*AttendanceStats, error) {
	tahun, semester, err := c.ActivePeriod(ctx)
	if err != nil {
		return nil, err
	}

	var res struct {
		Sukses bool `json:"sukses"`
		Data   struct {
			TotalSesi any `json:"totalSesi"`
			RataHadir any `json:"rataHadir"`
		} `json:"data"`
	}

	endpoint := fmt.Sprintf("/api/presensi/stat-beranda-mahasiswa?tahun=%d&semester=%d", tahun, semester)
	if err := c.getJSON(ctx, endpoint, nil, &res); err != nil {
		return nil, err
	}

	stats := &AttendanceStats{}
	switch v := res.Data.TotalSesi.(type) {
	case float64:
		stats.TotalSesi = int(v)
	case int:
		stats.TotalSesi = v
	case string:
		stats.TotalSesi, _ = strconv.Atoi(strings.TrimSpace(v))
	}

	switch v := res.Data.RataHadir.(type) {
	case float64:
		stats.RataHadir = v
	case int:
		stats.RataHadir = float64(v)
	case string:
		stats.RataHadir, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
	}

	return stats, nil
}

// GetActiveRoster finds any currently open presence session and queries attendees and absentees.
func (c *Client) GetActiveRoster(ctx context.Context) (*RosterInfo, error) {
	courses, err := c.Courses(ctx)
	if err != nil {
		return nil, err
	}

	for _, course := range courses {
		var active []activePresence
		if err := c.getJSON(ctx, "/api/presensi/aktif-kuliah", url.Values{
			"kuliah":       {strconv.Itoa(course.Number)},
			"jenis_schema": {strconv.Itoa(course.Schema)},
		}, &active); err != nil {
			continue
		}

		var key string
		for _, p := range active {
			if p.Open == 1 && p.Key != "" {
				key = p.Key
				break
			}
		}

		if key == "" {
			continue
		}

		info := &RosterInfo{
			Course: course,
			Key:    key,
		}

		// Query attendees
		attendeesURL := fmt.Sprintf("/api/presensi/daftar-mahasiswa-hadir-kuliah?key=%s", url.QueryEscape(key))
		_ = c.getJSON(ctx, attendeesURL, nil, &info.Attendees)

		// Query absentees
		absentURL := fmt.Sprintf("/api/presensi/daftar-mahasiswa-tidak-hadir-kuliah?key=%s&kuliah=%d&jenis_schema=%d",
			url.QueryEscape(key), course.Number, course.Schema)
		_ = c.getJSON(ctx, absentURL, nil, &info.Absentees)

		return info, nil
	}

	return nil, nil
}
