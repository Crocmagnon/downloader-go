package recettescabas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrWeekChanged is returned by Apply when the site shows another week than the one asked.
var ErrWeekChanged = errors.New("week changed")

type Service struct {
	Config Config
	// Fetch logs in and returns the current preferences page.
	Fetch func(ctx context.Context) (Page, error)
	// Apply sets the selection state of each item in desired and returns the page
	// as the site saved it. It returns ErrWeekChanged if the site's week isn't week.
	Apply func(ctx context.Context, week string, desired map[int]bool) (Page, error)
	Log   io.Writer
	// HAWebhook is the Home Assistant webhook every read of the site is pushed to.
	HAWebhook string
	// site serializes Fetch and Apply: two browsers would race on cookies.json.
	site *sync.Mutex
}

// fetch reads the site and pushes what it read to Home Assistant.
func (s Service) fetch(ctx context.Context) (Page, error) {
	s.site.Lock()
	defer s.site.Unlock()

	p, err := s.Fetch(ctx)
	if err == nil {
		s.pushHA(ctx, p)
	}

	return p, err
}

// apply changes the selection on the site and pushes the result to Home Assistant.
func (s Service) apply(ctx context.Context, week string, desired map[int]bool) (Page, error) {
	s.site.Lock()
	defer s.site.Unlock()

	p, err := s.Apply(ctx, week, desired)
	if err == nil {
		s.pushHA(ctx, p)
	}

	return p, err
}

func (s Service) report(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(s.Log, "recettes-cabas: %v\n", err)
	}
}
