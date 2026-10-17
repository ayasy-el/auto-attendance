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
	lastKeyboardMsgID int64
}

func NewBot(token, chatID string, etholClient *ethol.Client, sched *scheduler.Scheduler, student ethol.Student, loc *time.Location, version string) *Bot {
	if token == "" || chatID == "" {
		return nil
	}

	tgClient := NewClient(token, chatID)
	cmdHandler := NewCommandHandler(etholClient, sched, student, loc, version)

	return &Bot{
		client:  tgClient,
		handler: cmdHandler,
		chatID:  strings.TrimSpace(chatID),
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

func (b *Bot) sendReplyWithKeyboard(ctx context.Context, text string, keyboard any) {
	b.keyboardMu.Lock()
	defer b.keyboardMu.Unlock()

	// Remove keyboard from previous message so only the latest reply retains the buttons
	if b.lastKeyboardMsgID > 0 {
		_ = b.client.RemoveReplyMarkup(ctx, b.chatID, b.lastKeyboardMsgID)
	}

	msgID, err := b.client.SendMessageID(ctx, text, keyboard)
	if err != nil {
		log.Printf("telegram: gagal mengirim pesan: %v", err)
		return
	}
	if msgID > 0 {
		b.lastKeyboardMsgID = msgID
	}
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

		// Instantly clear button loading wheel and show typing indicator in chat
		_ = b.client.AnswerCallbackQuery(ctx, u.CallbackQuery.ID)
		_ = b.client.SendChatAction(ctx, "typing")

		if u.CallbackQuery.Data != "" {
			resp := b.handler.Handle(ctx, u.CallbackQuery.Data)
			if resp != "" {
				b.sendReplyWithKeyboard(ctx, resp, keyboard)
			}
		}
	}
}
