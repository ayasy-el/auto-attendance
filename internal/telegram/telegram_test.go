package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"auto-attendance/internal/config"
	"auto-attendance/internal/ethol"
	"auto-attendance/internal/logging"
	"auto-attendance/internal/scheduler"
)

func TestTelegramClient_SendMessageAndGetUpdates(t *testing.T) {
	var sendCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/botTOKEN/sendMessage":
			sendCalled.Add(1)
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
		case "/botTOKEN/getUpdates":
			fmt.Fprint(w, `{"ok":true,"result":[{"update_id":100,"message":{"message_id":1,"chat":{"id":12345},"text":"/status"}}]}`)
		case "/botTOKEN/deleteWebhook":
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("TOKEN", "12345")
	client.baseURL = server.URL

	ctx := context.Background()

	// Test SendMessage
	if err := client.SendMessage(ctx, "Hello", nil); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if sendCalled.Load() != 1 {
		t.Fatalf("expected sendCalled = 1, got %d", sendCalled.Load())
	}

	// Test GetUpdates
	updates, err := client.GetUpdates(ctx, 0, 10)
	if err != nil {
		t.Fatalf("GetUpdates failed: %v", err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 100 {
		t.Fatalf("unexpected updates: %#v", updates)
	}

	// Test DeleteWebhook
	if err := client.DeleteWebhook(ctx); err != nil {
		t.Fatalf("DeleteWebhook failed: %v", err)
	}
}

func TestCommandHandler_BasicCommands(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "logs.db")
	store, err := logging.Configure("Asia/Jakarta", dbPath, 7)
	if err != nil {
		t.Fatalf("logging.Configure failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	loc, _ := time.LoadLocation("Asia/Jakarta")

	cfg := config.Config{
		Host:     "http://localhost",
		CASHost:  "http://localhost",
		Username: "student",
		Password: "password",
		Timezone: "Asia/Jakarta",
	}
	etholClient, _ := ethol.New(cfg)
	student := ethol.Student{Number: 12345, Name: "Budi Santoso"}
	sched := scheduler.New(etholClient, cfg, student)

	handler := NewCommandHandler(etholClient, sched, student, loc, "v2.0.0")

	// Test /help
	helpResp := handler.Handle(ctx, "/help")
	if !strings.Contains(helpResp, "BANTUAN AUTO ATTENDANCE") {
		t.Fatalf("expected help text, got %q", helpResp)
	}

	// Test /whoami
	whoamiResp := handler.Handle(ctx, "/whoami")
	if !strings.Contains(whoamiResp, "Budi Santoso") || !strings.Contains(whoamiResp, "12345") {
		t.Fatalf("expected student name and ID in /whoami, got %q", whoamiResp)
	}

	// Test /status
	statusResp := handler.Handle(ctx, "/status")
	if !strings.Contains(statusResp, "STATUS SISTEM") || !strings.Contains(statusResp, "Budi Santoso") {
		t.Fatalf("expected status details, got %q", statusResp)
	}

	// Test /pause and /resume
	pauseResp := handler.Handle(ctx, "/pause")
	if !sched.IsPaused() || !strings.Contains(pauseResp, "Dijeda") {
		t.Fatalf("expected scheduler paused, got %q", pauseResp)
	}
	resumeResp := handler.Handle(ctx, "/resume")
	if sched.IsPaused() || !strings.Contains(resumeResp, "Dilanjutkan") {
		t.Fatalf("expected scheduler resumed, got %q", resumeResp)
	}

	// Test /logs default (5 lines)
	logsResp := handler.Handle(ctx, "/logs")
	if !strings.Contains(logsResp, "LOG TERAKHIR") {
		t.Fatalf("expected logs response, got %q", logsResp)
	}

	// Test /logs 10
	logs10Resp := handler.Handle(ctx, "/logs 10")
	if !strings.Contains(logs10Resp, "LOG TERAKHIR") {
		t.Fatalf("expected logs 10 response, got %q", logs10Resp)
	}

	// Test /ping
	pingResp := handler.Handle(ctx, "/ping")
	if pingResp != "🏓 Pong!" {
		t.Fatalf("expected 🏓 Pong!, got %q", pingResp)
	}

	// Test unknown command
	unknownResp := handler.Handle(ctx, "/unknown_cmd")
	if !strings.Contains(unknownResp, "tidak dikenali") {
		t.Fatalf("expected unknown command message, got %q", unknownResp)
	}
}

func TestBot_ProcessUpdate_SecurityFilter(t *testing.T) {
	var sentMessage atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/botTOKEN/sendMessage":
			sentMessage.Add(1)
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
		case "/botTOKEN/answerCallbackQuery":
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		case "/botTOKEN/sendChatAction":
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		case "/botTOKEN/editMessageReplyMarkup":
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	loc, _ := time.LoadLocation("Asia/Jakarta")
	cfg := config.Config{Host: "http://localhost", CASHost: "http://localhost", Username: "u", Password: "p"}
	etholClient, _ := ethol.New(cfg)
	student := ethol.Student{Number: 12345, Name: "Tester"}
	sched := scheduler.New(etholClient, cfg, student)

	bot := NewBot("TOKEN", "99999", etholClient, sched, student, loc, "v2.0.0")
	bot.client.baseURL = server.URL

	// 1. Update from unauthorized chat_id (e.g. 11111 != 99999) -> must be IGNORED
	unauthUpdate := Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 1,
			Chat:      Chat{ID: 11111},
			Text:      "/status",
		},
	}
	bot.processUpdate(ctx, unauthUpdate)
	if sentMessage.Load() != 0 {
		t.Fatalf("expected unauthorized message to be dropped, sentMessage = %d", sentMessage.Load())
	}

	// 2. Update from authorized chat_id (99999) -> must be PROCESSED
	authUpdate := Update{
		UpdateID: 2,
		Message: &Message{
			MessageID: 2,
			Chat:      Chat{ID: 99999},
			Text:      "/help",
		},
	}
	bot.processUpdate(ctx, authUpdate)
	if sentMessage.Load() != 1 {
		t.Fatalf("expected authorized message to receive response, sentMessage = %d", sentMessage.Load())
	}
}

func TestNotifier_NotifyTaskReminder(t *testing.T) {
	var sentText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if txt, ok := body["text"].(string); ok {
			sentText = txt
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	}))
	defer server.Close()

	client := NewClient("TOKEN", "99999")
	client.baseURL = server.URL
	loc, _ := time.LoadLocation("Asia/Jakarta")

	notifier := NewNotifier(client, loc, "https://ethol.pens.ac.id")

	task := ethol.TaskItem{
		KuliahID:   221585,
		CourseName: "Workshop Jaringan",
		Title:      "Modul 5",
		Deadline:   "16-10-2026 23:59 WIB",
	}

	notifier.NotifyTaskReminder(context.Background(), task, "Kurang 3 Jam", 2*time.Hour+45*time.Minute)

	if !strings.Contains(sentText, "PENGINGAT DEADLINE TUGAS (Kurang 3 Jam)") {
		t.Fatalf("expected stage in text, got: %s", sentText)
	}
	if !strings.Contains(sentText, "Modul 5") || !strings.Contains(sentText, "Workshop Jaringan") {
		t.Fatalf("expected task and course title in text, got: %s", sentText)
	}
	if !strings.Contains(sentText, "2 jam 45 menit") {
		t.Fatalf("expected remaining time string, got: %s", sentText)
	}
	if !strings.Contains(sentText, "https://ethol.pens.ac.id/mahasiswa/matakuliah/221585/tugas") {
		t.Fatalf("expected portal link in text, got: %s", sentText)
	}
}

func TestCommandHandler_LogsTruncation(t *testing.T) {
	// Create 50 lines directly in memory to test formatLogsMessage (> 6000 chars)
	longMsg := strings.Repeat("A", 120)
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, fmt.Sprintf("[2026-10-15 12:00:00 WIB] line %d %s", i, longMsg))
	}

	formatted := formatLogsMessage(lines, 50)
	if len(formatted) > 3800 {
		t.Fatalf("expected message length <= 3800, got %d", len(formatted))
	}
	if !strings.Contains(formatted, "baris lama terpotong") {
		t.Fatalf("expected truncation notice in output")
	}
	if !strings.HasPrefix(formatted, "📜 <b>LOG TERAKHIR") {
		t.Fatalf("expected valid header in output")
	}
	if !strings.HasSuffix(formatted, "</pre>") {
		t.Fatalf("expected valid closing pre tag in output")
	}
}

func TestBot_ButtonsOnlyOnLatestMessage(t *testing.T) {
	var sentCount atomic.Int32
	var removeMarkupCount atomic.Int32
	var lastRemovedMsgID atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/botTOKEN/sendMessage":
			id := sentCount.Add(1)
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d}}`, id)
		case "/botTOKEN/editMessageReplyMarkup":
			removeMarkupCount.Add(1)
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if mid, ok := payload["message_id"].(float64); ok {
				lastRemovedMsgID.Store(int64(mid))
			}
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		case "/botTOKEN/deleteWebhook":
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		case "/botTOKEN/sendChatAction":
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	loc, _ := time.LoadLocation("Asia/Jakarta")
	cfg := config.Config{Host: "http://localhost", CASHost: "http://localhost", Username: "u", Password: "p"}
	etholClient, _ := ethol.New(cfg)
	student := ethol.Student{Number: 12345, Name: "Tester"}
	sched := scheduler.New(etholClient, cfg, student)

	bot := NewBot("TOKEN", "99999", etholClient, sched, student, loc, "v2.0.0")
	bot.client.baseURL = server.URL

	// First message: send welcome greeting
	keyboard := DefaultKeyboard()
	bot.sendReplyWithKeyboard(ctx, "Welcome 1", keyboard)

	if sentCount.Load() != 1 {
		t.Fatalf("expected 1 sent message, got %d", sentCount.Load())
	}
	if removeMarkupCount.Load() != 0 {
		t.Fatalf("first message should not remove markup from non-existent previous message")
	}
	if _, ok := bot.activeKeyboardIDs[1]; !ok {
		t.Fatalf("expected activeKeyboardIDs to contain 1, got %+v", bot.activeKeyboardIDs)
	}

	// Second message: should remove markup from message 1 and record message 2
	bot.sendReplyWithKeyboard(ctx, "Status reply 2", keyboard)

	if sentCount.Load() != 2 {
		t.Fatalf("expected 2 sent messages, got %d", sentCount.Load())
	}
	if removeMarkupCount.Load() != 1 {
		t.Fatalf("expected 1 removeMarkup call, got %d", removeMarkupCount.Load())
	}
	if lastRemovedMsgID.Load() != 1 {
		t.Fatalf("expected removeMarkup called on message_id 1, got %d", lastRemovedMsgID.Load())
	}
	if _, ok := bot.activeKeyboardIDs[2]; !ok {
		t.Fatalf("expected activeKeyboardIDs to contain 2, got %+v", bot.activeKeyboardIDs)
	}

	// Third message via button click on message 2: should immediately remove markup on 2 and record message 3
	cbUpdate := Update{
		UpdateID: 10,
		CallbackQuery: &CallbackQuery{
			ID: "cb10",
			Message: &Message{
				MessageID: 2,
				Chat:      Chat{ID: 99999},
			},
			Data: "/status",
		},
	}
	bot.processUpdate(ctx, cbUpdate)

	if sentCount.Load() != 3 {
		t.Fatalf("expected 3 sent messages, got %d", sentCount.Load())
	}
	if removeMarkupCount.Load() < 2 {
		t.Fatalf("expected at least 2 removeMarkup calls, got %d", removeMarkupCount.Load())
	}
	if lastRemovedMsgID.Load() != 2 {
		t.Fatalf("expected removeMarkup called on message_id 2, got %d", lastRemovedMsgID.Load())
	}
	if _, ok := bot.activeKeyboardIDs[3]; !ok {
		t.Fatalf("expected activeKeyboardIDs to contain 3, got %+v", bot.activeKeyboardIDs)
	}
}
