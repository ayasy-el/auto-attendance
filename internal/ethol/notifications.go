package ethol

import (
	"context"
	"errors"
	"net/url"
)

func (c *Client) Notifications(ctx context.Context) ([]Notification, error) {
	var v []Notification
	err := c.getJSON(ctx, "/api/notifikasi/mahasiswa", url.Values{"filterNotif": {"PRESENSI"}}, &v)
	return v, err
}
func (c *Client) CloseNotification(ctx context.Context, id string) error {
	var result struct {
		Success bool `json:"success"`
	}
	if err := c.putJSON(ctx, "/api/notifikasi/mahasiswa-baca-notif", map[string]string{"idNotifikasi": id}, &result); err != nil {
		return err
	}
	if !result.Success {
		return errors.New("server gagal menutup notifikasi")
	}
	return nil
}
