package ethol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var (
	ErrNoActivePresence = errors.New("tidak ada presensi aktif")
	ErrAlreadyAttended  = errors.New("sudah melakukan presensi di sesi kuliah ini")
)

// Attend is kept for callers that still have a notification payload.
func (c *Client) Attend(ctx context.Context, n Notification, student Student) (string, error) {
	parts := strings.Split(n.Data, "-")
	if len(parts) != 2 {
		return "", fmt.Errorf("dataTerkait notifikasi %q tidak valid", n.Data)
	}
	kuliah, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("nomor kuliah %q tidak valid: %w", parts[0], err)
	}
	schema, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("jenis schema %q tidak valid: %w", parts[1], err)
	}
	var details []Course
	if err := c.getJSON(ctx, "/api/kuliah/by-kuliah-js", url.Values{"kuliah": {parts[0]}, "jenisSchema": {parts[1]}}, &details); err != nil {
		return "", err
	}
	if len(details) == 0 {
		return "", fmt.Errorf("detail kuliah %d kosong", kuliah)
	}
	return c.attendCourse(ctx, kuliah, schema, details[0].Origin, student)
}

func (c *Client) AttendCourse(ctx context.Context, course Course, student Student) (string, error) {
	return c.attendCourse(ctx, course.Number, course.Schema, course.Origin, student)
}

func (c *Client) attendCourse(ctx context.Context, kuliah, schema, origin int, student Student) (string, error) {
	var active []activePresence
	if err := c.getJSON(ctx, "/api/presensi/aktif-kuliah", url.Values{"kuliah": {strconv.Itoa(kuliah)}, "jenis_schema": {strconv.Itoa(schema)}}, &active); err != nil {
		return "", err
	}
	var key string
	for _, p := range active {
		if p.Open == 1 {
			key = p.Key
			break
		}
	}
	if key == "" {
		return "", ErrNoActivePresence
	}
	var historyBeforeSubmit []struct {
		Key string `json:"key"`
	}
	if err := c.getJSON(ctx, "/api/presensi/riwayat", url.Values{"kuliah": {strconv.Itoa(kuliah)}, "jenis_schema": {strconv.Itoa(schema)}, "nomor": {strconv.Itoa(student.Number)}}, &historyBeforeSubmit); err != nil {
		return "", err
	}
	for _, item := range historyBeforeSubmit {
		if item.Key == key {
			return "", ErrAlreadyAttended
		}
	}
	payload := map[string]any{"kuliah": kuliah, "jenis_schema": schema, "mahasiswa": student.Number, "key": key, "kuliah_asal": origin}
	var result struct {
		Success bool   `json:"sukses"`
		Message string `json:"pesan"`
	}
	if err := c.postJSON(ctx, "/api/presensi/mahasiswa", payload, &result); err != nil {
		return "", err
	}
	if !result.Success && result.Message == "Setujui kontrak perkuliahan terlebih dahulu sebelum melakukan presensi." {
		var contract struct {
			Success bool   `json:"sukses"`
			Message string `json:"pesan"`
		}
		if err := c.postJSON(ctx, "/api/kontrak/setuju", map[string]any{"kuliah": kuliah, "jenis_schema": schema}, &contract); err != nil {
			return "", fmt.Errorf("setuju kontrak: %w", err)
		}
		if !contract.Success {
			if contract.Message != "" {
				return "", fmt.Errorf("setuju kontrak ditolak: %s", contract.Message)
			}
			return "", errors.New("setuju kontrak ditolak")
		}
		if err := c.postJSON(ctx, "/api/presensi/mahasiswa", payload, &result); err != nil {
			return "", err
		}
	}
	if !result.Success {
		if result.Message == "Anda sudah melakukan di sesi kuliah ini" {
			return "", ErrAlreadyAttended
		}
		if result.Message != "" {
			return "", fmt.Errorf("server menolak presensi: %s", result.Message)
		}
		return "", errors.New("server menolak presensi")
	}
	var history json.RawMessage
	if err := c.getJSON(ctx, "/api/presensi/riwayat", url.Values{"kuliah": {strconv.Itoa(kuliah)}, "jenis_schema": {strconv.Itoa(schema)}, "nomor": {strconv.Itoa(student.Number)}}, &history); err != nil {
		return "", err
	}
	return key, nil
}
