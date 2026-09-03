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

func (c *Client) Attend(ctx context.Context, n Notification, student Student) (string, error) {
	parts := strings.Split(n.Data, "-")
	if len(parts) != 2 {
		return "", fmt.Errorf("dataTerkait notifikasi %q tidak valid", n.Data)
	}
	kuliah, _ := strconv.Atoi(parts[0])
	schema, _ := strconv.Atoi(parts[1])
	var details []Course
	if err := c.getJSON(ctx, "/api/kuliah/by-kuliah-js", url.Values{"kuliah": {parts[0]}, "jenisSchema": {parts[1]}}, &details); err != nil {
		return "", err
	}
	if len(details) == 0 {
		return "", fmt.Errorf("detail kuliah %d kosong", kuliah)
	}
	var active []activePresence
	if err := c.getJSON(ctx, "/api/presensi/aktif-kuliah", url.Values{"kuliah": {parts[0]}, "jenis_schema": {parts[1]}}, &active); err != nil {
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
		return "", errors.New("tidak ada presensi aktif")
	}
	payload := map[string]any{"kuliah": kuliah, "jenis_schema": schema, "mahasiswa": student.Number, "key": key, "kuliah_asal": details[0].Origin}
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
		if result.Message != "" {
			return "", fmt.Errorf("server menolak presensi: %s", result.Message)
		}
		return "", errors.New("server menolak presensi")
	}
	var history json.RawMessage
	if err := c.getJSON(ctx, "/api/presensi/riwayat", url.Values{"kuliah": {parts[0]}, "jenis_schema": {parts[1]}, "nomor": {strconv.Itoa(student.Number)}}, &history); err != nil {
		return "", err
	}
	return key, nil
}
