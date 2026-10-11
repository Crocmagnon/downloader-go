package recettescabas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ErrWeekChanged is returned by Apply when the site shows another week than the recap.
var ErrWeekChanged = errors.New("week changed")

const maxFailRunes = 300

const helpText = "/recap — renvoie le récap de la semaine\n/detail N — ingrédients de l'élément N"

type Service struct {
	TG     Telegram
	ChatID int64
	Config Config
	// Fetch logs in and returns the current preferences page.
	Fetch func(ctx context.Context) (Page, error)
	// Apply sets the selection state of each item in desired and returns the page
	// as the site saved it. It returns ErrWeekChanged if the site's week isn't week.
	Apply func(ctx context.Context, week string, desired map[int]bool) (Page, error)
	Log   io.Writer
}

func (s Service) SendRecap(ctx context.Context) error {
	p, err := s.Fetch(ctx)
	if err != nil {
		return err
	}

	p = Annotate(p, s.Config)

	return s.TG.Send(ctx, s.ChatID, RecapText(p), NewKeyboard(p.Items))
}

// Run handles updates until ctx is done.
func (s Service) Run(ctx context.Context) error {
	offset := 0

	for {
		updates, err := s.TG.GetUpdates(ctx, offset)
		if ctx.Err() != nil {
			return nil
		}

		if err != nil {
			s.report(err)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}

			continue
		}

		for _, u := range updates {
			offset = u.UpdateID + 1
			s.handle(ctx, u)
		}
	}
}

func (s Service) handle(ctx context.Context, u Update) {
	switch {
	case u.CallbackQuery != nil && u.CallbackQuery.Message != nil:
		if id := u.CallbackQuery.Message.Chat.ID; id != s.ChatID {
			s.ignore(id)
			return
		}

		s.callback(ctx, u.CallbackQuery)
	case u.Message != nil:
		if id := u.Message.Chat.ID; id != s.ChatID {
			s.ignore(id)
			return
		}

		s.message(ctx, u.Message)
	}
}

func (s Service) message(ctx context.Context, m *Message) {
	cmd, arg, _ := strings.Cut(strings.TrimSpace(m.Text), " ")

	switch cmd {
	case "/recap":
		if err := s.SendRecap(ctx); err != nil {
			s.fail(ctx, err)
		}
	case "/detail":
		s.detail(ctx, arg)
	default:
		s.report(s.TG.Send(ctx, s.ChatID, helpText, nil))
	}
}

func (s Service) detail(ctx context.Context, arg string) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil {
		s.report(s.TG.Send(ctx, s.ChatID, "Usage : /detail N", nil))
		return
	}

	p, err := s.Fetch(ctx)
	if err != nil {
		s.fail(ctx, err)
		return
	}

	for _, it := range Annotate(p, s.Config).Items {
		if it.N == n {
			s.report(s.TG.Send(ctx, s.ChatID, DetailText(it), nil))
			return
		}
	}

	s.report(s.TG.Send(ctx, s.ChatID, fmt.Sprintf("Élément %d inconnu.", n), nil))
}

func (s Service) callback(ctx context.Context, q *CallbackQuery) {
	s.report(s.TG.Answer(ctx, q.ID))

	msg := q.Message

	var kb [][]Button
	if msg.ReplyMarkup != nil {
		kb = msg.ReplyMarkup.InlineKeyboard
	}

	switch {
	case strings.HasPrefix(q.Data, cbToggle):
		n, err := strconv.Atoi(strings.TrimPrefix(q.Data, cbToggle))
		if err != nil {
			return
		}

		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, msg.Text, Toggle(kb, n)))
	case q.Data == cbCancel:
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, msg.Text, Reset(kb)))
	case q.Data == cbValidate:
		s.validate(ctx, msg, kb)
	}
}

func (s Service) validate(ctx context.Context, msg *Message, kb [][]Button) {
	desired := Pending(kb)
	if len(desired) == 0 {
		return
	}

	s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, "⏳ application…", nil))

	// An empty week (unexpected message text) never matches the site's: nothing is applied.
	p, err := s.Apply(ctx, weekRe.FindString(msg.Text), desired)

	switch {
	case errors.Is(err, ErrWeekChanged):
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID,
			msg.Text+"\n\n⚠️ La semaine a changé : rien n'a été appliqué. Utilise /recap.", nil))
	case err != nil:
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, msg.Text, kb))
		s.fail(ctx, err)
	default:
		p = Annotate(p, s.Config)
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, RecapText(p), NewKeyboard(p.Items)))
	}
}

// fail logs err and sends its first line to Telegram, shortened: Playwright errors
// carry long call logs, and a message over Telegram's limit would not be delivered.
func (s Service) fail(ctx context.Context, err error) {
	s.report(err)

	msg, _, _ := strings.Cut(err.Error(), "\n")
	if r := []rune(msg); len(r) > maxFailRunes {
		msg = string(r[:maxFailRunes]) + "…"
	}

	s.report(s.TG.Send(ctx, s.ChatID, "❌ échec : "+msg, nil))
}

func (s Service) report(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(s.Log, "recettes-cabas: %v\n", err)
	}
}

func (s Service) ignore(chatID int64) {
	_, _ = fmt.Fprintf(s.Log, "recettes-cabas: ignoring update from chat %d\n", chatID)
}
