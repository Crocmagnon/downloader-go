package recettescabas

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"time"
)

// ErrWeekChanged is returned by Apply when the site shows another week than the recap.
var ErrWeekChanged = errors.New("week changed")

const maxFailRunes = 300

const typingEvery = 4 * time.Second

// Telegram albums hold 2 to 10 items.
const (
	minAlbum = 2
	maxAlbum = 10
)

const helpText = "/recap — renvoie le récap de la semaine\n/detail N — ingrédients de l'élément N\n/affichage — boutons avec titres ou numéros"

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
	// PrefsPath is the /affichage settings file, shared by the bot and the weekly recap.
	PrefsPath string
}

func (s Service) SendRecap(ctx context.Context) error {
	p, err := s.Fetch(ctx)
	if err != nil {
		return err
	}

	p = Annotate(p, s.Config)

	// Recipe photos first, as an album; the recap stays usable without them.
	var photos []Photo
	for _, it := range p.Items {
		if it.Image != "" {
			photos = append(photos, Photo{URL: it.Image, Caption: fmt.Sprintf("%d. %s", it.N, it.Name)})
		}
	}
	if len(photos) >= minAlbum {
		s.report(s.TG.SendAlbum(ctx, s.ChatID, photos[:min(len(photos), maxAlbum)]))
	}

	return s.TG.Send(ctx, s.ChatID, RecapText(p), NewKeyboard(p.Items, s.prefs().Compact))
}

// Run handles updates until ctx is done.
func (s Service) Run(ctx context.Context) error {
	// Fills the "/" menu in Telegram; harmless if it fails.
	s.report(s.TG.SetCommands(ctx, []Command{
		{Command: "recap", Description: "Récap de la semaine"},
		{Command: "detail", Description: "Ingrédients d'un élément : /detail 6"},
		{Command: "affichage", Description: "Boutons avec titres ou numéros"},
	}))

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
		stop := s.busy(ctx)
		defer stop()

		if err := s.SendRecap(ctx); err != nil {
			s.fail(ctx, err)
		}
	case "/detail":
		s.detail(ctx, arg)
	case "/affichage":
		s.report(s.TG.Send(ctx, s.ChatID, "Affichage des boutons :", [][]Button{{
			{Text: "🔢 Numéros", CallbackData: cbStyle + "n"},
			{Text: "🔤 Titres", CallbackData: cbStyle + "t"},
		}}))
	default:
		s.report(s.TG.Send(ctx, s.ChatID, helpText, nil))
	}
}

// detail sends the ingredients of item arg, or a picker when arg is empty (the "/" menu
// sends commands right away, without letting the user type a number).
func (s Service) detail(ctx context.Context, arg string) {
	arg = strings.TrimSpace(arg)

	n, err := strconv.Atoi(arg)
	if err != nil && arg != "" {
		s.report(s.TG.Send(ctx, s.ChatID, "Usage : /detail N, par exemple /detail 6", nil))
		return
	}

	stop := s.busy(ctx)
	defer stop()

	p, err := s.Fetch(ctx)
	if err != nil {
		s.fail(ctx, err)
		return
	}

	p = Annotate(p, s.Config)
	if arg == "" {
		s.report(s.TG.Send(ctx, s.ChatID, "Quel élément ?", DetailKeyboard(p.Items, s.prefs().Compact)))
		return
	}

	for _, it := range p.Items {
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

		s.report(s.TG.EditMarkup(ctx, s.ChatID, msg.MessageID, Toggle(kb, n)))
	case strings.HasPrefix(q.Data, cbDetail):
		s.detail(ctx, strings.TrimPrefix(q.Data, cbDetail))
	case strings.HasPrefix(q.Data, cbStyle):
		compact := q.Data == cbStyle+"n"
		if err := SavePrefs(s.PrefsPath, Prefs{Compact: compact}); err != nil {
			s.fail(ctx, err)
			return
		}

		text := "Affichage : titres. Prochain /recap avec ce style."
		if compact {
			text = "Affichage : numéros. Prochain /recap avec ce style."
		}
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, text, nil))
	case q.Data == cbCancel:
		s.report(s.TG.EditMarkup(ctx, s.ChatID, msg.MessageID, Reset(kb)))
	case q.Data == cbValidate:
		s.validate(ctx, msg, kb)
	}
}

func (s Service) validate(ctx context.Context, msg *Message, kb [][]Button) {
	desired := Pending(kb)
	if len(desired) == 0 {
		return
	}

	s.report(s.TG.EditMarkup(ctx, s.ChatID, msg.MessageID, [][]Button{{{Text: "⏳ application…", CallbackData: cbNoop}}}))

	stop := s.busy(ctx)
	defer stop()

	// An empty week (unexpected message text) never matches the site's: nothing is applied.
	p, err := s.Apply(ctx, weekRe.FindString(msg.Text), desired)

	switch {
	case errors.Is(err, ErrWeekChanged):
		s.report(s.TG.EditMarkup(ctx, s.ChatID, msg.MessageID, nil))
		s.report(s.TG.Send(ctx, s.ChatID, "⚠️ La semaine a changé : rien n'a été appliqué. Utilise /recap.", nil))
	case err != nil:
		s.report(s.TG.EditMarkup(ctx, s.ChatID, msg.MessageID, kb))
		s.fail(ctx, err)
	default:
		p = Annotate(p, s.Config)
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, RecapText(p), NewKeyboard(p.Items, s.prefs().Compact)))
	}
}

// fail logs err and sends its first line to Telegram, shortened: Playwright errors
// carry long call logs, and a message over Telegram's limit would not be delivered.
// busy shows "typing…" in the chat until stop is called: reading or changing the site takes
// several seconds, and Telegram drops the indicator after 5.
func (s Service) busy(ctx context.Context) (stop func()) {
	s.report(s.TG.Typing(ctx, s.ChatID))

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(typingEvery)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.TG.Typing(ctx, s.ChatID); ctx.Err() == nil {
					s.report(err)
				}
			}
		}
	}()

	return cancel
}

// prefs falls back to the defaults if the file can't be read.
func (s Service) prefs() Prefs {
	p, err := LoadPrefs(s.PrefsPath)
	s.report(err)

	return p
}

func (s Service) fail(ctx context.Context, err error) {
	s.report(err)

	msg, _, _ := strings.Cut(err.Error(), "\n")
	if r := []rune(msg); len(r) > maxFailRunes {
		msg = string(r[:maxFailRunes]) + "…"
	}

	s.report(s.TG.Send(ctx, s.ChatID, "❌ échec : "+html.EscapeString(msg), nil))
}

func (s Service) report(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(s.Log, "recettes-cabas: %v\n", err)
	}
}

func (s Service) ignore(chatID int64) {
	_, _ = fmt.Fprintf(s.Log, "recettes-cabas: ignoring update from chat %d\n", chatID)
}
