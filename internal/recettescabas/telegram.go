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

const (
	pollTimeout = 50     // seconds, Telegram long polling
	parseMode   = "HTML" // message texts are HTML: escape what comes from the site
)

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
		ParseMode   string  `json:"parse_mode"`
		ReplyMarkup *Markup `json:"reply_markup,omitzero"`
	}{chatID, text, parseMode, markup(kb)}, nil)
}

// Photo is one picture of an album, sent by URL.
type Photo struct {
	URL     string
	Caption string
}

// SendAlbum sends 2 to 10 photos as one album.
func (t Telegram) SendAlbum(ctx context.Context, chatID int64, photos []Photo) error {
	type media struct {
		Type    string `json:"type"`
		Media   string `json:"media"`
		Caption string `json:"caption,omitzero"`
	}

	items := make([]media, 0, len(photos))
	for _, p := range photos {
		items = append(items, media{Type: "photo", Media: p.URL, Caption: p.Caption})
	}

	return t.call(ctx, "sendMediaGroup", struct {
		ChatID int64   `json:"chat_id"`
		Media  []media `json:"media"`
	}{chatID, items}, nil)
}

// Command is an entry of the bot's "/" menu.
type Command struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func (t Telegram) SetCommands(ctx context.Context, commands []Command) error {
	return t.call(ctx, "setMyCommands", map[string]any{"commands": commands}, nil)
}

// Edit replaces a message's text and keyboard; a nil kb removes the keyboard.
func (t Telegram) Edit(ctx context.Context, chatID int64, messageID int, text string, kb [][]Button) error {
	return t.call(ctx, "editMessageText", struct {
		ChatID      int64   `json:"chat_id"`
		MessageID   int     `json:"message_id"`
		Text        string  `json:"text"`
		ParseMode   string  `json:"parse_mode"`
		ReplyMarkup *Markup `json:"reply_markup,omitzero"`
	}{chatID, messageID, text, parseMode, markup(kb)}, nil)
}

// EditMarkup replaces only a message's keyboard, keeping its formatted text: a callback
// only gives back the plain text. A nil kb removes the keyboard.
func (t Telegram) EditMarkup(ctx context.Context, chatID int64, messageID int, kb [][]Button) error {
	return t.call(ctx, "editMessageReplyMarkup", struct {
		ChatID      int64   `json:"chat_id"`
		MessageID   int     `json:"message_id"`
		ReplyMarkup *Markup `json:"reply_markup,omitzero"`
	}{chatID, messageID, markup(kb)}, nil)
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
		return fmt.Errorf("%s: creating request: %w", method, redact(err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", method, redact(err))
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

// redact drops the URL of a *url.Error: it contains the bot token.
func redact(err error) error {
	if uerr, ok := errors.AsType[*url.Error](err); ok {
		return uerr.Err
	}

	return err
}
