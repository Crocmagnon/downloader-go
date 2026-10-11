package recettescabas

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"strings"
	"testing"
)

const testChat = 42

func botPage() Page {
	return Page{
		Week: "19/10/2026", Deadline: "Jeudi 15 Octobre 2026 à 23h00", People: "4 pers.",
		Items: []Item{
			{N: 1, Name: "Gratin", Label: "top chrono", Ingredients: "champignons de Paris"},
			{N: 2, Name: "Filet", Label: "viande 1", Selected: true, Ingredients: "porc"},
			{N: 3, Name: "Poisson", Label: "poisson", Ingredients: "cabillaud"},
		},
	}
}

type fakes struct {
	fetches     int
	applyWeek   string
	applyWanted map[int]bool
	applyErr    error
}

func newService(t *testing.T) (Service, *fakes, *[]tgCall, *bytes.Buffer) {
	tg, calls := fakeTelegram(t, `{}`)
	f := &fakes{}
	var log bytes.Buffer
	s := Service{
		TG: tg, ChatID: testChat, Config: testConfig(), Log: &log,
		Fetch: func(context.Context) (Page, error) { f.fetches++; return botPage(), nil },
		Apply: func(_ context.Context, week string, desired map[int]bool) (Page, error) {
			f.applyWeek, f.applyWanted = week, desired
			if f.applyErr != nil {
				return Page{}, f.applyErr
			}
			p := botPage()
			p.Items[2].Selected = true
			return p, nil
		},
	}
	return s, f, calls, &log
}

func recapMessage(kb [][]Button) *Message {
	p := Annotate(botPage(), testConfig())
	return &Message{MessageID: 7, Chat: Chat{ID: testChat}, Text: RecapText(p), ReplyMarkup: &Markup{InlineKeyboard: kb}}
}

func callback(data string, msg *Message) Update {
	return Update{CallbackQuery: &CallbackQuery{ID: "cb", Data: data, Message: msg}}
}

func last(calls *[]tgCall) tgCall { return (*calls)[len(*calls)-1] }

func TestSendRecap(t *testing.T) {
	s, _, calls, _ := newService(t)
	if err := s.SendRecap(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := last(calls)
	for _, want := range []string{`"chat_id":42`, "Semaine du 19/10/2026", "🍄 Gratin", `"callback_data":"t:3"`} {
		if c.Method != "sendMessage" || !strings.Contains(c.Body, want) {
			t.Errorf("call = %+v, missing %s", c, want)
		}
	}
}

func TestIgnoresOtherChats(t *testing.T) {
	s, f, calls, log := newService(t)
	s.handle(context.Background(), Update{Message: &Message{Chat: Chat{ID: 999}, Text: "/recap"}})
	other := recapMessage(NewKeyboard(botPage().Items))
	other.Chat.ID = 999
	s.handle(context.Background(), callback(cbValidate, other))

	if len(*calls) != 0 || f.fetches != 0 || f.applyWanted != nil {
		t.Errorf("calls = %+v, fetches = %d, apply = %v", *calls, f.fetches, f.applyWanted)
	}
	if !strings.Contains(log.String(), "999") {
		t.Errorf("log = %q, want the ignored chat id", log.String())
	}
}

func TestToggleCallbackEditsKeyboardOnly(t *testing.T) {
	s, f, calls, _ := newService(t)
	s.handle(context.Background(), callback("t:3", recapMessage(NewKeyboard(botPage().Items))))

	if (*calls)[0].Method != "answerCallbackQuery" {
		t.Errorf("first call = %s, want answerCallbackQuery", (*calls)[0].Method)
	}
	c := last(calls)
	if c.Method != "editMessageText" || !strings.Contains(c.Body, "➕3") || !strings.Contains(c.Body, "Valider (1)") {
		t.Errorf("call = %+v", c)
	}
	if f.fetches != 0 || f.applyWanted != nil {
		t.Error("toggling must not touch the site")
	}
}

func TestGarbageCallbackIsNoop(t *testing.T) {
	s, _, calls, _ := newService(t)
	s.handle(context.Background(), callback("t:x", recapMessage(NewKeyboard(botPage().Items))))
	s.handle(context.Background(), callback("zzz", recapMessage(nil)))
	for _, c := range *calls {
		if c.Method != "answerCallbackQuery" {
			t.Errorf("unexpected call %+v", c)
		}
	}
}

func TestValidateAppliesAndShowsNewRecap(t *testing.T) {
	s, f, calls, _ := newService(t)
	kb := Toggle(NewKeyboard(botPage().Items), 3)
	s.handle(context.Background(), callback(cbValidate, recapMessage(kb)))

	if f.applyWeek != "19/10/2026" || !maps.Equal(f.applyWanted, map[int]bool{3: true}) {
		t.Errorf("apply(%q, %v)", f.applyWeek, f.applyWanted)
	}
	c := last(calls)
	if c.Method != "editMessageText" || !strings.Contains(c.Body, "✅3") || strings.Contains(c.Body, "Valider") {
		t.Errorf("final edit = %+v", c)
	}
}

func TestValidateWeekChanged(t *testing.T) {
	s, f, calls, _ := newService(t)
	f.applyErr = ErrWeekChanged
	s.handle(context.Background(), callback(cbValidate, recapMessage(Toggle(NewKeyboard(botPage().Items), 3))))

	c := last(calls)
	if c.Method != "editMessageText" || !strings.Contains(c.Body, "La semaine a changé") || strings.Contains(c.Body, "inline_keyboard") {
		t.Errorf("final edit = %+v", c)
	}
}

func TestValidateFailureRestoresKeyboard(t *testing.T) {
	s, f, calls, _ := newService(t)
	f.applyErr = errors.New("boom")
	s.handle(context.Background(), callback(cbValidate, recapMessage(Toggle(NewKeyboard(botPage().Items), 3))))

	var restored, reported bool
	for _, c := range *calls {
		restored = restored || (c.Method == "editMessageText" && strings.Contains(c.Body, "➕3"))
		reported = reported || (c.Method == "sendMessage" && strings.Contains(c.Body, "❌ échec : boom"))
	}
	if !restored || !reported {
		t.Errorf("restored = %v, reported = %v, calls = %+v", restored, reported, *calls)
	}
}

func TestDetail(t *testing.T) {
	tests := []struct {
		text    string
		want    string
		fetches int
	}{
		{"/detail 1", "champignons de Paris", 1},
		{"/detail 42", "Élément 42 inconnu", 1},
		{"/detail abc", "Usage : /detail N", 0},
		{"/detail", "Usage : /detail N", 0},
		{"bonjour", "/recap", 0},
	}
	for _, tt := range tests {
		s, f, calls, _ := newService(t)
		s.handle(context.Background(), Update{Message: &Message{Chat: Chat{ID: testChat}, Text: tt.text}})
		if c := last(calls); c.Method != "sendMessage" || !strings.Contains(c.Body, tt.want) || f.fetches != tt.fetches {
			t.Errorf("%q: call = %+v, fetches = %d", tt.text, c, f.fetches)
		}
	}
}

func TestFailMessageIsShort(t *testing.T) {
	s, _, calls, _ := newService(t)
	s.fail(context.Background(), errors.New("boom: "+strings.Repeat("x", 5000)+"\nCall log:\n  - waiting for locator"))
	c := last(calls)
	if c.Method != "sendMessage" || !strings.Contains(c.Body, "❌ échec : boom") || strings.Contains(c.Body, "Call log") || len(c.Body) > 1000 {
		t.Errorf("call = %s %d bytes", c.Method, len(c.Body))
	}
}
