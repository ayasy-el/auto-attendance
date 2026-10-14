package ethol

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
)

type authConfigResponse struct {
	TahunAktifSnake    any `json:"tahun_aktif"`
	TahunAktifCamel    any `json:"tahunAktif"`
	SemesterAktifSnake any `json:"semester_aktif"`
	SemesterAktifCamel any `json:"semesterAktif"`
}

func parseYearSem(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(val))
		return n
	default:
		return 0
	}
}

func (c *Client) ActivePeriod(ctx context.Context) (int, int, error) {
	c.periodMu.RLock()
	if c.activeYear > 0 && c.activeSemester > 0 {
		y, s := c.activeYear, c.activeSemester
		c.periodMu.RUnlock()
		return y, s, nil
	}
	c.periodMu.RUnlock()

	var conf authConfigResponse
	if err := c.getJSON(ctx, "/api/auth/config", nil, &conf); err != nil {
		return 0, 0, fmt.Errorf("ambil config periode aktif: %w", err)
	}

	tahun := parseYearSem(conf.TahunAktifSnake)
	if tahun == 0 {
		tahun = parseYearSem(conf.TahunAktifCamel)
	}
	semester := parseYearSem(conf.SemesterAktifSnake)
	if semester == 0 {
		semester = parseYearSem(conf.SemesterAktifCamel)
	}

	if tahun <= 0 || semester <= 0 {
		return 0, 0, fmt.Errorf("periode akademik tidak valid dari server: tahun=%d semester=%d", tahun, semester)
	}

	log.Printf("periode akademik terdeteksi: tahun=%d semester=%d", tahun, semester)

	c.periodMu.Lock()
	c.activeYear = tahun
	c.activeSemester = semester
	c.periodMu.Unlock()

	return tahun, semester, nil
}

func (c *Client) Courses(ctx context.Context) ([]Course, error) {
	tahun, semester, err := c.ActivePeriod(ctx)
	if err != nil {
		return nil, err
	}
	var v []Course
	err = c.getJSON(ctx, "/api/kuliah", url.Values{"tahun": {strconv.Itoa(tahun)}, "semester": {strconv.Itoa(semester)}}, &v)
	return v, err
}

func (c *Client) Schedules(ctx context.Context, courses []Course) ([]ClassTime, error) {
	tahun, semester, err := c.ActivePeriod(ctx)
	if err != nil {
		return nil, err
	}
	body := struct {
		Courses []struct {
			Number int `json:"nomor"`
			Schema int `json:"jenisSchema"`
		} `json:"kuliahs"`
		Tahun    int `json:"tahun"`
		Semester int `json:"semester"`
	}{Tahun: tahun, Semester: semester}
	for _, course := range courses {
		body.Courses = append(body.Courses, struct {
			Number int `json:"nomor"`
			Schema int `json:"jenisSchema"`
		}{course.Number, course.Schema})
	}
	var v []ClassTime
	err = c.postJSON(ctx, "/api/kuliah/hari-kuliah-in", body, &v)
	return v, err
}
