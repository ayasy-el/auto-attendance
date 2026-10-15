package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"auto-attendance/internal/ethol"
	"auto-attendance/internal/logging"
	"auto-attendance/internal/scheduler"
)

type CommandHandler struct {
	client    *ethol.Client
	scheduler *scheduler.Scheduler
	student   ethol.Student
	loc       *time.Location
	version   string
}

func NewCommandHandler(client *ethol.Client, sched *scheduler.Scheduler, student ethol.Student, loc *time.Location, version string) *CommandHandler {
	if loc == nil {
		loc = time.Local
	}
	return &CommandHandler{
		client:    client,
		scheduler: sched,
		student:   student,
		loc:       loc,
		version:   version,
	}
}

func DefaultKeyboard() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "📅 Jadwal", CallbackData: "/jadwal"},
				{Text: "📝 Tugas", CallbackData: "/tugas"},
			},
			{
				{Text: "⚡ Check", CallbackData: "/check"},
				{Text: "📜 Logs", CallbackData: "/logs"},
			},
			{
				{Text: "📊 Rekap", CallbackData: "/rekap"},
				{Text: "👥 Kelas", CallbackData: "/presensi_kelas"},
			},
			{
				{Text: "ℹ️ Status", CallbackData: "/status"},
				{Text: "❓ Bantuan", CallbackData: "/help"},
			},
		},
	}
}

func (h *CommandHandler) Handle(ctx context.Context, text string) string {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) == 0 {
		return ""
	}

	cmd := strings.ToLower(parts[0])
	// Strip bot mention if command is e.g. /status@mybot
	if idx := strings.Index(cmd, "@"); idx != -1 {
		cmd = cmd[:idx]
	}

	switch cmd {
	case "/start", "/help", "bantuan":
		return h.help()

	case "/status", "status":
		return h.status()

	case "/check", "check":
		return h.check(ctx)

	case "/logs", "logs":
		n := 5
		if len(parts) > 1 {
			if parsed, err := strconv.Atoi(parts[1]); err == nil && parsed > 0 {
				n = parsed
			}
		}
		return h.logs(ctx, n)

	case "/today", "today":
		return h.today()

	case "/whoami", "whoami":
		return h.whoami()

	case "/pause", "pause":
		h.scheduler.Pause()
		return "⏸️ <b>Scanning Presensi Dijeda!</b>\nOtomatisasi dijeda sementara. Gunakan /resume untuk mengaktifkannya kembali."

	case "/resume", "resume":
		h.scheduler.Resume()
		return "▶️ <b>Scanning Presensi Dilanjutkan!</b>\nOtomatisasi kembali aktif mengikuti jadwal kelas."

	case "/jadwal", "jadwal":
		return h.jadwal(ctx)

	case "/tugas", "tugas":
		return h.tugas(ctx)

	case "/ujian", "ujian":
		return h.ujian(ctx)

	case "/materi", "materi":
		return h.materi(ctx)

	case "/rekap", "rekap":
		return h.rekap(ctx)

	case "/presensi_kelas", "/kelas", "presensi_kelas", "kelas":
		return h.presensiKelas(ctx)

	default:
		return "Perintah tidak dikenali. Ketik /help untuk melihat daftar perintah yang tersedia."
	}
}

func (h *CommandHandler) help() string {
	return `🤖 <b>BANTUAN AUTO ATTENDANCE</b>

<b>Perintah Kontrol:</b>
• /status - Status operasional daemon
• /check - Trigger pengecekan presensi sekarang
• /logs [n] - Lihat n baris log terakhir (default 5, maks 50)
• /today - Daftar kehadiran tercatat hari ini
• /whoami - Profil mahasiswa tertaut
• /pause - Jeda scanning otomatis
• /resume - Lanjutkan scanning otomatis

<b>Perintah Akademik:</b>
• /jadwal - Jadwal perkuliahan mingguan
• /tugas - Daftar tugas kuliah & deadline
• /ujian - Jadwal ujian UTS & UAS
• /materi - Materi perkuliahan & video
• /rekap - Rekap persentase kehadiran
• /presensi_kelas - Roster kehadiran di kelas aktif`
}

func (h *CommandHandler) status() string {
	st := h.scheduler.Status()
	modeStr := st.Mode
	if st.IsPaused {
		modeStr = "PAUSED (Dijeda)"
	}
	if st.IsDryRun {
		modeStr += " [DRY-RUN]"
	}

	uptime := time.Since(st.StartTime).Round(time.Second)
	lastTickStr := "Belum ada"
	if !st.LastTick.IsZero() {
		lastTickStr = st.LastTick.Format("15:04:05 MST")
	}

	verStr := h.version
	if verStr == "" {
		verStr = "dev"
	}

	return fmt.Sprintf("ℹ️ <b>STATUS SISTEM</b>\n\n"+
		"👤 <b>Mahasiswa:</b> %s (%d)\n"+
		"🏷️ <b>Versi:</b> <code>%s</code>\n"+
		"⚙️ <b>Mode:</b> <code>%s</code>\n"+
		"📚 <b>Mata Kuliah:</b> %d dipantau\n"+
		"🕒 <b>Tick Terakhir:</b> %s\n"+
		"⏳ <b>Uptime:</b> %s",
		html.EscapeString(st.Student.Name), st.Student.Number,
		html.EscapeString(verStr),
		html.EscapeString(modeStr),
		st.Courses,
		lastTickStr,
		uptime.String(),
	)
}

func (h *CommandHandler) check(ctx context.Context) string {
	courses, err := h.scheduler.TriggerCheck(ctx)
	if err != nil {
		return fmt.Sprintf("❌ <b>Pengecekan Gagal:</b>\n<code>%s</code>", html.EscapeString(err.Error()))
	}
	return fmt.Sprintf("⚡ <b>Pengecekan Selesai!</b>\nMemindai %d mata kuliah. Jika ada sesi presensi yang baru dibuka, notifikasi akan segera masuk.", courses)
}

func (h *CommandHandler) logs(ctx context.Context, n int) string {
	store := logging.GetGlobalStore()
	if store == nil {
		return "⚠️ Log database SQLite belum aktif atau tidak dikonfigurasi."
	}

	if n > 50 {
		n = 50
	}

	lines, err := store.GetRecentLogs(ctx, n)
	if err != nil {
		return fmt.Sprintf("❌ <b>Gagal Membaca Log:</b> %s", html.EscapeString(err.Error()))
	}

	if len(lines) == 0 {
		return fmt.Sprintf("📜 <b>LOG TERAKHIR (%d baris)</b>\n\n(Belum ada catatan log)", n)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📜 <b>LOG TERAKHIR (%d baris):</b>\n<pre>", len(lines)))
	for _, l := range lines {
		sb.WriteString(html.EscapeString(l))
		sb.WriteString("\n")
	}
	sb.WriteString("</pre>")

	return sb.String()
}

func (h *CommandHandler) today() string {
	attended := h.scheduler.AttendedToday()
	if len(attended) == 0 {
		return "📌 <b>PRESENSI HARI INI</b>\n\nBelum ada presensi yang tercatat untuk hari ini."
	}

	var sb strings.Builder
	sb.WriteString("📌 <b>PRESENSI HARI INI:</b>\n\n")
	for i, k := range attended {
		sb.WriteString(fmt.Sprintf("%d. <code>%s</code>\n", i+1, html.EscapeString(k)))
	}
	return strings.TrimSpace(sb.String())
}

func (h *CommandHandler) whoami() string {
	return fmt.Sprintf("👤 <b>PROFIL MAHASISWA</b>\n\n"+
		"Nama: <b>%s</b>\n"+
		"Nomor ID: <code>%d</code>",
		html.EscapeString(h.student.Name),
		h.student.Number,
	)
}

func (h *CommandHandler) jadwal(ctx context.Context) string {
	items, err := h.client.GetOnlineSchedule(ctx)
	if err != nil {
		return fmt.Sprintf("❌ <b>Gagal Mengambil Jadwal:</b> %s", html.EscapeString(err.Error()))
	}

	if len(items) == 0 {
		return "📅 <b>JADWAL PERKULIAHAN</b>\n\nTidak ada jadwal perkuliahan ditemukan."
	}

	now := time.Now().In(h.loc)
	todayVal := int(now.Weekday())
	if todayVal == 0 {
		todayVal = 7
	}

	var sb strings.Builder
	sb.WriteString("📅 <b>JADWAL PERKULIAHAN</b>\n")

	currentDay := -1
	for _, item := range items {
		dayVal := item.DayValue()
		if dayVal != currentDay {
			currentDay = dayVal
			dayName := strings.ToUpper(strings.TrimSpace(item.Hari))
			if dayName == "" {
				dayName = "LAINNYA"
			}
			if dayVal == todayVal {
				sb.WriteString(fmt.Sprintf("\n📌 <b>%s (HARI INI)</b>\n", html.EscapeString(dayName)))
			} else {
				sb.WriteString(fmt.Sprintf("\n📌 <b>%s</b>\n", html.EscapeString(dayName)))
			}
		}

		sb.WriteString(fmt.Sprintf("• <b>%s</b>\n  🕒 %s - %s",
			html.EscapeString(item.CourseName()),
			html.EscapeString(item.JamAwal),
			html.EscapeString(item.JamAkhir),
		))
		if item.Ruang != "" {
			sb.WriteString(fmt.Sprintf(" | 📍 %s", html.EscapeString(item.Ruang)))
		}
		if item.Dosen != "" {
			sb.WriteString(fmt.Sprintf(" | 👨‍🏫 %s", html.EscapeString(item.Dosen)))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

func (h *CommandHandler) tugas(ctx context.Context) string {
	tasks, err := h.client.GetTasks(ctx)
	if err != nil {
		return fmt.Sprintf("❌ <b>Gagal Mengambil Tugas:</b> %s", html.EscapeString(err.Error()))
	}

	if len(tasks) == 0 {
		return "🎉 <b>TUGAS KULIAH</b>\n\nTidak ada tugas perkuliahan yang aktif."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📝 <b>TUGAS KULIAH (%d)</b>\n\n", len(tasks)))

	now := time.Now().In(h.loc)
	for i, task := range tasks {
		badge := "🟢"
		deadlineStr := task.Deadline
		if task.DeadlineID != "" {
			deadlineStr = task.DeadlineID
		}

		if dl, err := time.ParseInLocation("2006-01-02 15:04:05", task.Deadline, h.loc); err == nil {
			diff := dl.Sub(now)
			if diff < 0 {
				badge = "⚪ (Selesai/Tutup)"
			} else if diff < 24*time.Hour {
				badge = "🔴 (Mendesak! < 24 Jam)"
			} else if diff < 72*time.Hour {
				badge = "🟡 (H-3)"
			}
		}

		sb.WriteString(fmt.Sprintf("%d. %s <b>%s</b>\n   📚 %s\n   ⏰ Deadline: %s\n\n",
			i+1,
			badge,
			html.EscapeString(task.TaskTitle()),
			html.EscapeString(task.CourseName),
			html.EscapeString(deadlineStr),
		))
	}

	return strings.TrimSpace(sb.String())
}

func (h *CommandHandler) ujian(ctx context.Context) string {
	exams, err := h.client.GetExams(ctx)
	if err != nil {
		return fmt.Sprintf("❌ <b>Gagal Mengambil Jadwal Ujian:</b> %s", html.EscapeString(err.Error()))
	}

	if len(exams) == 0 {
		return "🎯 <b>JADWAL UJIAN</b>\n\nBelum ada jadwal ujian yang terdaftar."
	}

	var sb strings.Builder
	sb.WriteString("🎯 <b>JADWAL UJIAN (UTS / UAS)</b>\n\n")
	for i, ex := range exams {
		sb.WriteString(fmt.Sprintf("%d. [%s] <b>%s</b>\n   🕒 %s - %s\n",
			i+1,
			html.EscapeString(ex.Jenis),
			html.EscapeString(ex.CourseName()),
			html.EscapeString(ex.Ujian.Mulai),
			html.EscapeString(ex.Ujian.Selesai),
		))
		if ex.Ruang != "" {
			sb.WriteString(fmt.Sprintf("   📍 Ruang: %s\n", html.EscapeString(ex.Ruang)))
		}
		if d := ex.DosenName(); d != "" {
			sb.WriteString(fmt.Sprintf("   👨‍🏫 Dosen: %s\n", html.EscapeString(d)))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

func (h *CommandHandler) materi(ctx context.Context) string {
	courses, err := h.client.Courses(ctx)
	if err != nil {
		return fmt.Sprintf("❌ <b>Gagal Mengambil Daftar Kuliah:</b> %s", html.EscapeString(err.Error()))
	}

	var sb strings.Builder
	sb.WriteString("📑 <b>MATERI & VIDEO PERKULIAHAN</b>\n\n")

	foundCount := 0
	for _, c := range courses {
		materials, videos, err := h.client.GetMaterials(ctx, c)
		if err != nil || (len(materials) == 0 && len(videos) == 0) {
			continue
		}
		foundCount++
		sb.WriteString(fmt.Sprintf("📖 <b>%s</b>\n", html.EscapeString(c.Subject.Name)))
		for _, m := range materials {
			title := m.Judul
			if title == "" {
				title = "Dokumen Materi"
			}
			sb.WriteString(fmt.Sprintf("  • [📄 Dokumen] %s (%s)\n", html.EscapeString(title), html.EscapeString(m.Tanggal)))
		}
		for _, v := range videos {
			title := v.Judul
			if title == "" {
				title = "Rekaman Video"
			}
			sb.WriteString(fmt.Sprintf("  • [🎥 Video] %s (%s)\n", html.EscapeString(title), html.EscapeString(v.Tanggal)))
		}
		sb.WriteString("\n")
	}

	if foundCount == 0 {
		return "📑 <b>MATERI PERKULIAHAN</b>\n\nBelum ada materi atau video yang diunggah dosen."
	}

	return strings.TrimSpace(sb.String())
}

func (h *CommandHandler) rekap(ctx context.Context) string {
	stats, err := h.client.GetAttendanceStats(ctx)
	if err != nil {
		return fmt.Sprintf("❌ <b>Gagal Mengambil Rekap:</b> %s", html.EscapeString(err.Error()))
	}

	return fmt.Sprintf("📊 <b>REKAP KEHADIRAN RESMI</b>\n\n"+
		"📈 <b>Rata-rata Kehadiran:</b> %.1f%%\n"+
		"🔢 <b>Total Sesi Perkuliahan:</b> %d sesi",
		stats.RataHadir,
		stats.TotalSesi,
	)
}

func (h *CommandHandler) presensiKelas(ctx context.Context) string {
	roster, err := h.client.GetActiveRoster(ctx)
	if err != nil {
		return fmt.Sprintf("❌ <b>Gagal Mengambil Roster:</b> %s", html.EscapeString(err.Error()))
	}

	if roster == nil {
		return "👥 <b>PRESENSI KELAS AKTIF</b>\n\nSaat ini tidak ada sesi kuliah yang membuka presensi aktif."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("👥 <b>PRESENSI KELAS AKTIF</b>\n\n"+
		"📚 <b>Mata Kuliah:</b> %s\n"+
		"🔑 <b>Key:</b> <code>%s</code>\n"+
		"✅ <b>Hadir:</b> %d mahasiswa\n"+
		"❌ <b>Belum Hadir:</b> %d mahasiswa\n\n",
		html.EscapeString(roster.Course.Subject.Name),
		html.EscapeString(roster.Key),
		len(roster.Attendees),
		len(roster.Absentees),
	))

	if len(roster.Attendees) > 0 {
		sb.WriteString("<b>Daftar Mahasiswa Hadir:</b>\n")
		for i, a := range roster.Attendees {
			if i >= 15 {
				sb.WriteString(fmt.Sprintf("... dan %d lainnya\n", len(roster.Attendees)-15))
				break
			}
			sb.WriteString(fmt.Sprintf("• %s - %s\n", html.EscapeString(a.NRP), html.EscapeString(a.Nama)))
		}
	}

	return strings.TrimSpace(sb.String())
}
