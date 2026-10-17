package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"auto-attendance/internal/ethol"
	"auto-attendance/internal/scheduler"
)

type Bot struct {
	client            *Client
	handler           *CommandHandler
	chatID            string
	keyboardMu        sync.Mutex
	activeKeyboardIDs map[int64]struct{}
}

func NewBot(token, chatID string, etholClient *ethol.Client, sched *scheduler.Scheduler, student ethol.Student, loc *time.Location, version string) *Bot {
	if token == "" || chatID == "" {
		return nil
	}

	tgClient := NewClient(token, chatID)
	cmdHandler := NewCommandHandler(etholClient, sched, student, loc, version)

	return &Bot{
		client:            tgClient,
		handler:           cmdHandler,
		chatID:            strings.TrimSpace(chatID),
		activeKeyboardIDs: make(map[int64]struct{}),
	}
}

func (b *Bot) Client() *Client {
	return b.client
}

func (b *Bot) Start(ctx context.Context) {
	if b == nil || b.client == nil {
		return
	}

	log.Println("telegram bot: menginisialisasi listener polling...")
	_ = b.client.DeleteWebhook(ctx)

	// Send welcome greeting on startup
	keyboard := DefaultKeyboard()
	welcome := fmt.Sprintf("🤖 <b>Auto Attendance Aktif!</b>\n\n"+
		"Sistem presensi otomatis sedang berjalan di latar belakang.\n"+
		"Gunakan tombol di bawah atau ketik /help untuk melihat menu perintah.",
	)
	b.sendReplyWithKeyboard(ctx, welcome, keyboard)

	go b.pollLoop(ctx)
}

func (b *Bot) sendReplyWithKeyboard(ctx context.Context, text string, keyboard any, additionalMsgIDs ...int64) {
	b.keyboardMu.Lock()
	defer b.keyboardMu.Unlock()

	// Remove keyboard from all previous active messages
	for mid := range b.activeKeyboardIDs {
		_ = b.client.RemoveReplyMarkup(ctx, b.chatID, mid)
		delete(b.activeKeyboardIDs, mid)
	}
	for _, mid := range additionalMsgIDs {
		if mid > 0 {
			_ = b.client.RemoveReplyMarkup(ctx, b.chatID, mid)
			delete(b.activeKeyboardIDs, mid)
		}
	}

	chunks := splitTelegramMessage(text, 3800)
	if len(chunks) == 0 {
		return
	}

	for i := 0; i < len(chunks)-1; i++ {
		_ = b.client.SendMessageToChat(ctx, b.chatID, chunks[i], nil)
	}

	lastMsgID, err := b.client.SendMessageID(ctx, chunks[len(chunks)-1], keyboard)
	if err != nil {
		log.Printf("telegram: gagal mengirim pesan: %v", err)
		return
	}
	if lastMsgID > 0 {
		b.activeKeyboardIDs[lastMsgID] = struct{}{}
	}
}

func splitTelegramMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	remaining := text
	for len(remaining) > maxLen {
		splitIdx := strings.LastIndex(remaining[:maxLen], "\n")
		if splitIdx <= 0 {
			splitIdx = maxLen
		}
		chunks = append(chunks, remaining[:splitIdx])
		remaining = strings.TrimPrefix(remaining[splitIdx:], "\n")
	}
	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}
	return chunks
}

func (b *Bot) pollLoop(ctx context.Context) {
	var offset int64 = 0

	for {
		select {
		case <-ctx.Done():
			log.Println("telegram bot: polling dihentikan")
			return
		default:
		}

		updates, err := b.client.GetUpdates(ctx, offset, 20)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(3 * time.Second)
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}

			go b.processUpdate(ctx, update)
		}
	}
}

func (b *Bot) processUpdate(ctx context.Context, u Update) {
	keyboard := DefaultKeyboard()

	// 1. Text message
	if u.Message != nil && u.Message.Text != "" {
		senderChatID := strconv.FormatInt(u.Message.Chat.ID, 10)
		if senderChatID != b.chatID {
			// Unauthorized chat: silently drop
			return
		}

		// Show loading feedback (typing indicator in header)
		_ = b.client.SendChatAction(ctx, "typing")

		resp := b.handler.Handle(ctx, u.Message.Text)
		if resp != "" {
			b.sendReplyWithKeyboard(ctx, resp, keyboard)
		}
		return
	}

	// 2. Inline button click (CallbackQuery)
	if u.CallbackQuery != nil && u.CallbackQuery.Message != nil {
		senderChatID := strconv.FormatInt(u.CallbackQuery.Message.Chat.ID, 10)
		if senderChatID != b.chatID {
			// Unauthorized: silently drop
			return
		}

		clickedMsgID := u.CallbackQuery.Message.MessageID

		// Instantly clear button loading wheel and show typing indicator in chat
		_ = b.client.AnswerCallbackQuery(ctx, u.CallbackQuery.ID)
		_ = b.client.SendChatAction(ctx, "typing")

		// Remove markup from the clicked message immediately so it cannot be double-clicked
		if clickedMsgID > 0 {
			_ = b.client.RemoveReplyMarkup(ctx, b.chatID, clickedMsgID)
		}

		if u.CallbackQuery.Data != "" {
			resp := b.handler.Handle(ctx, u.CallbackQuery.Data)
			if resp != "" {
				b.sendReplyWithKeyboard(ctx, resp, keyboard, clickedMsgID)
			}
		}
	}
}
