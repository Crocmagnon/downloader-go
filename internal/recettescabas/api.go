package recettescabas

import (
	"context"
	"crypto/subtle"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	maxAPIBody = 64 << 10
	// maxFailRunes shortens error answers: Playwright errors carry long call logs.
	maxFailRunes = 300
)

var exactWeekRe = regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)

// API serves the endpoints Home Assistant calls. Every request needs "Authorization: Bearer <token>".
func (s Service) API(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /refresh", s.handleRefresh)
	mux.HandleFunc("POST /apply", s.handleApply)

	want := []byte("Bearer " + token)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAPIBody)
		mux.ServeHTTP(w, r)
	})
}

// Serve listens on addr and serves the API until ctx is done.
func (s Service) Serve(ctx context.Context, addr, token string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	srv := &http.Server{Handler: s.API(token), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s Service) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if _, err := s.fetch(r.Context()); err != nil {
		s.apiFail(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s Service) handleApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Week    string       `json:"week"`
		Desired map[int]bool `json:"desired"`
	}
	if err := json.UnmarshalRead(r.Body, &req, json.RejectUnknownMembers(true)); err != nil ||
		!exactWeekRe.MatchString(req.Week) || len(req.Desired) == 0 {
		http.Error(w, "requête invalide : week (JJ/MM/AAAA) et desired (numéro → booléen) attendus", http.StatusBadRequest)
		return
	}

	// Once started, the change is applied and pushed even if Home Assistant stopped waiting.
	_, err := s.apply(context.WithoutCancel(r.Context()), req.Week, req.Desired)
	if errors.Is(err, ErrWeekChanged) {
		http.Error(w, "la semaine a changé : rien n'a été appliqué", http.StatusConflict)
		return
	}
	if err != nil {
		s.apiFail(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// apiFail logs err and answers with its first line, shortened.
func (s Service) apiFail(w http.ResponseWriter, err error) {
	s.report(err)

	msg, _, _ := strings.Cut(err.Error(), "\n")
	if r := []rune(msg); len(r) > maxFailRunes {
		msg = string(r[:maxFailRunes]) + "…"
	}

	http.Error(w, "échec : "+msg, http.StatusBadGateway)
}
