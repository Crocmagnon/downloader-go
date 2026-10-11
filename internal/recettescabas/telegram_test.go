package recettescabas

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type tgCall struct {
	Method string
	Body   string
}

// fakeTelegram records every call and answers each with the given result JSON.
func fakeTelegram(t *testing.T, result string) (Telegram, *[]tgCall) {
	t.Helper()
	var calls []tgCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if !strings.HasPrefix(r.URL.Path, "/botSECRET/") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		calls = append(calls, tgCall{Method: method, Body: string(body)})
		_, _ = io.WriteString(w, `{"ok":true,"result":`+result+`}`)
	}))
	t.Cleanup(srv.Close)
	tg := NewTelegram("SECRET")
	tg.BaseURL = srv.URL
	return tg, &calls
}

func TestSendWithKeyboard(t *testing.T) {
	tg, calls := fakeTelegram(t, `{}`)
	kb := [][]Button{{{Text: "✅2", CallbackData: "t:2"}}}
	if err := tg.Send(context.Background(), 42, "salut", kb); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	for _, want := range []string{`"chat_id":42`, `"text":"salut"`, `"inline_keyboard":[[{"text":"✅2","callback_data":"t:2"}]]`} {
		if c.Method != "sendMessage" || !strings.Contains(c.Body, want) {
			t.Errorf("call = %+v, missing %s", c, want)
		}
	}
}

func TestEditWithoutKeyboardOmitsMarkup(t *testing.T) {
	tg, calls := fakeTelegram(t, `{}`)
	if err := tg.Edit(context.Background(), 42, 7, "⏳", nil); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.Method != "editMessageText" || !strings.Contains(c.Body, `"message_id":7`) || strings.Contains(c.Body, "reply_markup") {
		t.Errorf("call = %+v", c)
	}
}

func TestGetUpdates(t *testing.T) {
	tg, calls := fakeTelegram(t, `[{"update_id":5,"callback_query":{"id":"cb","data":"t:3","message":{"message_id":7,"chat":{"id":42},"text":"x","reply_markup":{"inline_keyboard":[[{"text":"3","callback_data":"t:3"}]]}}}}]`)
	updates, err := tg.GetUpdates(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains((*calls)[0].Body, `"offset":5`) {
		t.Errorf("body = %s", (*calls)[0].Body)
	}
	q := updates[0].CallbackQuery
	if updates[0].UpdateID != 5 || q.Data != "t:3" || q.Message.Chat.ID != 42 || q.Message.ReplyMarkup.InlineKeyboard[0][0].Text != "3" {
		t.Errorf("updates = %+v", updates)
	}
}

func TestAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/editMessageText") {
			_, _ = io.WriteString(w, `{"ok":false,"description":"Bad Request: message is not modified"}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":false,"description":"Forbidden: bot was blocked by the user"}`)
	}))
	defer srv.Close()
	tg := NewTelegram("SECRET")
	tg.BaseURL = srv.URL

	if err := tg.Edit(context.Background(), 1, 1, "x", nil); err != nil {
		t.Errorf("'message is not modified' must not be an error, got %v", err)
	}
	if err := tg.Send(context.Background(), 1, "x", nil); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Errorf("err = %v", err)
	}
}

func TestTransportErrorDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // connection refused
	tg := NewTelegram("SECRET")
	tg.BaseURL = srv.URL

	err := tg.Send(context.Background(), 1, "x", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error leaks the bot token: %v", err)
	}
}
