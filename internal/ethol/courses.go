package ethol

import (
	"context"
	"net/url"
	"strconv"
)

func (c *Client) Courses(ctx context.Context) ([]Course, error) {
	var v []Course
	err := c.getJSON(ctx, "/api/kuliah", url.Values{"tahun": {strconv.Itoa(c.cfg.Tahun)}, "semester": {strconv.Itoa(c.cfg.Semester)}}, &v)
	return v, err
}
func (c *Client) Schedules(ctx context.Context, courses []Course) ([]ClassTime, error) {
	body := struct {
		Courses []struct {
			Number int `json:"nomor"`
			Schema int `json:"jenisSchema"`
		} `json:"kuliahs"`
		Tahun    int `json:"tahun"`
		Semester int `json:"semester"`
	}{Tahun: c.cfg.Tahun, Semester: c.cfg.Semester}
	for _, course := range courses {
		body.Courses = append(body.Courses, struct {
			Number int `json:"nomor"`
			Schema int `json:"jenisSchema"`
		}{course.Number, course.Schema})
	}
	var v []ClassTime
	err := c.postJSON(ctx, "/api/kuliah/hari-kuliah-in", body, &v)
	return v, err
}
