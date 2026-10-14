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

const maxAPIBody = 64 << 10

const haHeader = "✏️ Sélection modifiée depuis Home Assistant"

var exactWeekRe = regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)

// CheckBridge tells whether the Home Assistant bridge is enabled: all of its settings are
// set, or none. A partial setup is an error, so that a forgotten setting doesn't go unnoticed.
func CheckBridge(listen, token, webhook string) (bool, error) {
	var missing []string
	for _, setting := range []struct{ name, value string }{
		{"--api-listen", listen}, {"RC_API_TOKEN", token}, {"RC_HA_WEBHOOK_URL", webhook},
	} {
		if setting.value == "" {
			missing = append(missing, setting.name)
		}
	}

	switch len(missing) {
	case 0:
		return true, nil
	case 3:
		return false, nil
	default:
		return false, fmt.Errorf("home assistant bridge partially configured, missing %s", strings.Join(missing, ", "))
	}
}

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

// StartAPI listens on addr and serves the API in the background until ctx is done.
// It returns once listening, so that a bad address fails the start.
func (s Service) StartAPI(ctx context.Context, addr, token string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	srv := &http.Server{Handler: s.API(token), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			s.report(err)
		}
	}()

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

	// Once applied, the change is announced even if Home Assistant stopped waiting.
	ctx := context.WithoutCancel(r.Context())

	p, err := s.apply(ctx, req.Week, req.Desired)
	if errors.Is(err, ErrWeekChanged) {
		http.Error(w, "la semaine a changé : rien n'a été appliqué", http.StatusConflict)
		return
	}
	if err != nil {
		s.apiFail(w, err)
		return
	}

	s.report(s.TG.Send(ctx, s.ChatID, haHeader, nil))
	s.report(s.sendRecap(ctx, Annotate(p, s.Config)))

	w.WriteHeader(http.StatusOK)
}

// apiFail logs err and answers with its first line, like the Telegram "❌ échec" message.
func (s Service) apiFail(w http.ResponseWriter, err error) {
	s.report(err)
	http.Error(w, "échec : "+firstLine(err), http.StatusBadGateway)
}
