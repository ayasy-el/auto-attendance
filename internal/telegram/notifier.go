package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"auto-attendance/internal/ethol"
)

type Notifier struct {
	client     *Client
	loc        *time.Location
	portalHost string
}

func NewNotifier(client *Client, loc *time.Location, portalHost ...string) *Notifier {
	if loc == nil {
		loc = time.Local
	}
	host := ""
	if len(portalHost) > 0 {
		host = portalHost[0]
	}
	return &Notifier{
		client:     client,
		loc:        loc,
		portalHost: host,
	}
}

func (n *Notifier) NotifyPresenceSuccess(ctx context.Context, courseName, dosen, key string) {
	if n.client == nil {
		return
	}
	timeStr := time.Now().In(n.loc).Format("15:04:05 MST")
	dosenLine := ""
	if dosen != "" {
		dosenLine = fmt.Sprintf("👨‍🏫 <b>Dosen:</b> %s\n", html.EscapeString(dosen))
	}
	text := fmt.Sprintf("✅ <b>Presensi Berhasil Tercatat!</b>\n\n"+
		"📚 <b>Mata Kuliah:</b> %s\n"+
		"%s"+
		"🔑 <b>Key:</b> <code>%s</code>\n"+
		"🕒 <b>Waktu:</b> %s",
		html.EscapeString(courseName),
		dosenLine,
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

func (n *Notifier) NotifyTaskReminder(ctx context.Context, task ethol.TaskItem, stage string, remaining time.Duration) {
	if n.client == nil {
		return
	}
	remStr := formatRemainingDuration(remaining)

	linkLine := ""
	if task.KuliahID > 0 && n.portalHost != "" {
		baseURL := strings.TrimRight(n.portalHost, "/")
		link := fmt.Sprintf("%s/mahasiswa/matakuliah/%d/tugas", baseURL, task.KuliahID)
		linkLine = fmt.Sprintf("\n🔗 <a href=%q>Buka Tugas di Portal</a>", link)
	}

	text := fmt.Sprintf("🚨 <b>PENGINGAT DEADLINE TUGAS (%s)!</b>\n\n"+
		"📝 <b>Tugas:</b> %s\n"+
		"📚 <b>Mata Kuliah:</b> %s\n"+
		"⏰ <b>Deadline:</b> %s\n"+
		"⏳ <b>Sisa Waktu:</b> %s%s",
		html.EscapeString(stage),
		html.EscapeString(task.TaskTitle()),
		html.EscapeString(task.CourseName),
		html.EscapeString(task.Deadline),
		html.EscapeString(remStr),
		linkLine,
	)
	_ = n.client.SendMessage(ctx, text, nil)
}

func formatRemainingDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%d jam %d menit", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%d jam", h)
	}
	return fmt.Sprintf("%d menit", m)
}
