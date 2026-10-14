package recettescabas

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	haTimeout = 10 * time.Second
	// haSlots is the number of item entities in the Home Assistant package: items above
	// are pushed but not shown.
	haSlots = 25
)

// The field names are read by the Home Assistant package: don't rename them.
type haPage struct {
	Week     string    `json:"week"`
	Deadline string    `json:"deadline"`
	People   string    `json:"people"`
	Price    string    `json:"price"`
	Alerts   []haAlert `json:"alerts"`
	Items    []haItem  `json:"items"`
}

type haAlert struct {
	N      int    `json:"n"`
	Emojis string `json:"emojis"`
}

type haItem struct {
	N           int      `json:"n"`
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Option      bool     `json:"option"`
	Selected    bool     `json:"selected"`
	Flags       []haFlag `json:"flags"`
	Selection   string   `json:"selection"`
	Ingredients string   `json:"ingredients"`
	Pantry      string   `json:"pantry"`
	Image       string   `json:"image"`
}

type haFlag struct {
	Emoji string `json:"emoji"`
	Word  string `json:"word"`
}

// haJSON renders an annotated page as the webhook payload.
func haJSON(p Page) ([]byte, error) {
	out := haPage{Week: p.Week, Deadline: p.Deadline, People: p.People, Price: p.Price}

	for _, it := range alerts(p) {
		out.Alerts = append(out.Alerts, haAlert{N: it.N, Emojis: emojis(it)})
	}

	for _, it := range p.Items {
		flags := make([]haFlag, 0, len(it.Flags))
		for _, h := range it.Flags {
			flags = append(flags, haFlag(h))
		}

		out.Items = append(out.Items, haItem{
			N: it.N, Name: it.Name, Label: labelSuffix(it), Option: it.Option, Selected: it.Selected, Flags: flags,
			Selection: it.Selection, Ingredients: it.Ingredients, Pantry: it.Pantry, Image: it.Image,
		})
	}

	return json.Marshal(out)
}

// pushHA sends the page to the Home Assistant webhook, if one is set. A failure is only
// logged: Home Assistant must never break the action that read the site.
func (s Service) pushHA(ctx context.Context, p Page) {
	if s.HAWebhook == "" {
		return
	}

	p = Annotate(p, s.Config)
	for _, it := range p.Items {
		if it.N > haSlots {
			s.report(fmt.Errorf("home assistant: item %d has no entity (%d slots)", it.N, haSlots))
		}
	}

	s.report(s.postHA(ctx, p))
}

func (s Service) postHA(ctx context.Context, p Page) error {
	body, err := haJSON(p)
	if err != nil {
		return fmt.Errorf("home assistant: marshaling: %w", err)
	}

	// Pushed even if the caller gave up waiting: the site was read, Home Assistant should know.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), haTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.HAWebhook, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("home assistant: creating request: %w", redact(err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("home assistant: %w", redact(err))
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("home assistant: webhook answered %s", resp.Status)
	}

	return nil
}

// redact drops the URL of a *url.Error: it holds the webhook id, a secret.
func redact(err error) error {
	if uerr, ok := errors.AsType[*url.Error](err); ok {
		return uerr.Err
	}

	return err
}
