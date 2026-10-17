package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	token   string
	chatID  string
	baseURL string
	http    *http.Client
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
	Text      string `json:"text"`
	Date      int64  `json:"date"`
}

type updatesResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

func NewClient(token, chatID string) *Client {
	return &Client{
		token:   token,
		chatID:  chatID,
		baseURL: "https://api.telegram.org",
		http: &http.Client{
			Timeout: 35 * time.Second,
		},
	}
}

type sendResponse struct {
	OK     bool `json:"ok"`
	Result *struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
}

func (c *Client) SendMessage(ctx context.Context, text string, markup any) error {
	_, err := c.SendMessageID(ctx, text, markup)
	return err
}

func (c *Client) SendMessageID(ctx context.Context, text string, markup any) (int64, error) {
	return c.SendMessageToChatID(ctx, c.chatID, text, markup)
}

func (c *Client) SendMessageToChat(ctx context.Context, chatID, text string, markup any) error {
	_, err := c.SendMessageToChatID(ctx, chatID, text, markup)
	return err
}

func (c *Client) SendMessageToChatID(ctx context.Context, chatID, text string, markup any) (int64, error) {
	if c.token == "" || chatID == "" {
		return 0, nil
	}

	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("telegram sendMessage HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var sr sendResponse
	if err := json.Unmarshal(body, &sr); err == nil && sr.Result != nil {
		return sr.Result.MessageID, nil
	}
	return 0, nil
}

func (c *Client) RemoveReplyMarkup(ctx context.Context, chatID string, messageID int64) error {
	if c.token == "" || chatID == "" || messageID <= 0 {
		return nil
	}

	// In Telegram Bot API, omitting reply_markup completely clears the inline keyboard.
	// Passing an empty array or empty inline_keyboard can trigger 400 Bad Request.
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/bot%s/editMessageReplyMarkup", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		errStr := strings.TrimSpace(string(body))
		// Ignore harmless errors if message was already modified, deleted, or missing text
		if strings.Contains(errStr, "message is not modified") ||
			strings.Contains(errStr, "message to edit not found") ||
			strings.Contains(errStr, "there is no text in the message to edit") {
			return nil
		}
		return fmt.Errorf("editMessageReplyMarkup HTTP %d: %s", resp.StatusCode, errStr)
	}
	return nil
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, queryID string) error {
	if c.token == "" || queryID == "" {
		return nil
	}
	payload := map[string]string{"callback_query_id": queryID}
	data, _ := json.Marshal(payload)
	endpoint := fmt.Sprintf("%s/bot%s/answerCallbackQuery", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) SendChatAction(ctx context.Context, action string) error {
	return c.SendChatActionToChat(ctx, c.chatID, action)
}

func (c *Client) SendChatActionToChat(ctx context.Context, chatID, action string) error {
	if c.token == "" || chatID == "" {
		return nil
	}
	payload := map[string]string{
		"chat_id": chatID,
		"action":  action,
	}
	data, _ := json.Marshal(payload)
	endpoint := fmt.Sprintf("%s/bot%s/sendChatAction", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	if c.token == "" {
		return nil, nil
	}

	endpoint := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&timeout=%d", c.baseURL, c.token, offset, timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("telegram getUpdates HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var res updatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Result, nil
}

func (c *Client) DeleteWebhook(ctx context.Context) error {
	if c.token == "" {
		return nil
	}
	endpoint := fmt.Sprintf("%s/bot%s/deleteWebhook", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
