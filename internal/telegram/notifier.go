package telegram

import (
	"context"
	"fmt"
	"html"
	"time"
)

type Notifier struct {
	client *Client
	loc    *time.Location
}

func NewNotifier(client *Client, loc *time.Location) *Notifier {
	if loc == nil {
		loc = time.Local
	}
	return &Notifier{
		client: client,
		loc:    loc,
	}
}

func (n *Notifier) NotifyPresenceSuccess(ctx context.Context, courseName, key string) {
	if n.client == nil {
		return
	}
	timeStr := time.Now().In(n.loc).Format("15:04:05 MST")
	text := fmt.Sprintf("✅ <b>Presensi Berhasil Tercatat!</b>\n\n"+
		"📚 <b>Mata Kuliah:</b> %s\n"+
		"🔑 <b>Key:</b> <code>%s</code>\n"+
		"🕒 <b>Waktu:</b> %s",
		html.EscapeString(courseName),
		html.EscapeString(key),
		timeStr,
	)
	_ = n.client.SendMessage(ctx, text, nil)
}

func (n *Notifier) NotifyContractApproved(ctx context.Context, courseName string) {
	if n.client == nil {
		return
	}
	text := fmt.Sprintf("📝 <b>Kontrak Kuliah Disetujui Otomatis!</b>\n\n"+
		"📚 <b>Mata Kuliah:</b> %s\n"+
		"Sistem secara otomatis menyetujui kontrak perkuliahan sebelum melanjutkan presensi.",
		html.EscapeString(courseName),
	)
	_ = n.client.SendMessage(ctx, text, nil)
}

func (n *Notifier) NotifyPresenceFailed(ctx context.Context, courseName string, err error) {
	if n.client == nil || err == nil {
		return
	}
	text := fmt.Sprintf("⚠️ <b>Presensi Gagal!</b>\n\n"+
		"📚 <b>Mata Kuliah:</b> %s\n"+
		"❌ <b>Error:</b> <code>%s</code>",
		html.EscapeString(courseName),
		html.EscapeString(err.Error()),
	)
	_ = n.client.SendMessage(ctx, text, nil)
}
