package recettescabas

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const pollTimeout = 50 // seconds, Telegram long polling

// Telegram is a minimal Bot API client.
type Telegram struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func NewTelegram(token string) Telegram {
	return Telegram{
		Token:   token,
		BaseURL: "https://api.telegram.org",
		HTTP:    &http.Client{Timeout: (pollTimeout + 20) * time.Second},
	}
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID   int     `json:"message_id"`
	Chat        Chat    `json:"chat"`
	Text        string  `json:"text"`
	ReplyMarkup *Markup `json:"reply_markup"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Markup struct {
	InlineKeyboard [][]Button `json:"inline_keyboard"`
}

func markup(kb [][]Button) *Markup {
	if len(kb) == 0 {
		return nil
	}

	return &Markup{InlineKeyboard: kb}
}

func (t Telegram) GetUpdates(ctx context.Context, offset int) ([]Update, error) {
	var updates []Update
	err := t.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         pollTimeout,
		"allowed_updates": []string{"message", "callback_query"},
	}, &updates)

	return updates, err
}

func (t Telegram) Send(ctx context.Context, chatID int64, text string, kb [][]Button) error {
	return t.call(ctx, "sendMessage", struct {
		ChatID      int64   `json:"chat_id"`
		Text        string  `json:"text"`
		ReplyMarkup *Markup `json:"reply_markup,omitzero"`
	}{chatID, text, markup(kb)}, nil)
}

// Edit replaces a message's text and keyboard; a nil kb removes the keyboard.
func (t Telegram) Edit(ctx context.Context, chatID int64, messageID int, text string, kb [][]Button) error {
	return t.call(ctx, "editMessageText", struct {
		ChatID      int64   `json:"chat_id"`
		MessageID   int     `json:"message_id"`
		Text        string  `json:"text"`
		ReplyMarkup *Markup `json:"reply_markup,omitzero"`
	}{chatID, messageID, text, markup(kb)}, nil)
}

func (t Telegram) Answer(ctx context.Context, callbackID string) error {
	return t.call(ctx, "answerCallbackQuery", map[string]string{"callback_query_id": callbackID}, nil)
}

func (t Telegram) call(ctx context.Context, method string, params, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("%s: marshaling params: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.BaseURL+"/bot"+t.Token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: creating request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.HTTP.Do(req)
	if err != nil {
		// *url.Error's message contains the URL, which contains the token.
		if uerr, ok := errors.AsType[*url.Error](err); ok {
			err = uerr.Err
		}

		return fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var env struct {
		OK          bool           `json:"ok"`
		Description string         `json:"description"`
		Result      jsontext.Value `json:"result"`
	}
	if err := json.UnmarshalRead(resp.Body, &env); err != nil {
		return fmt.Errorf("%s: decoding response: %w", method, err)
	}

	if !env.OK {
		if strings.Contains(env.Description, "message is not modified") {
			return nil
		}

		return fmt.Errorf("%s: %s", method, env.Description)
	}

	if result != nil {
		if err := json.Unmarshal(env.Result, result); err != nil {
			return fmt.Errorf("%s: decoding result: %w", method, err)
		}
	}

	return nil
}
