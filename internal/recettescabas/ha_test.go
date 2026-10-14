package recettescabas

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeHA records the bodies pushed to the webhook.
func fakeHA(t *testing.T, status int) (url string, pushes func() []string) {
	t.Helper()
	var (
		mu     sync.Mutex
		bodies []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		bodies = append(bodies, string(body))
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/api/webhook/SECRETID", func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

func TestHAJSON(t *testing.T) {
	p := botPage()
	p.Items[0].Selected = true
	got, err := haJSON(Annotate(p, testConfig()))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"week":"19/10/2026","deadline":"Jeudi 15 Octobre 2026 à 23h00","people":"4 pers.","price":"",` +
		`"alerts":[{"n":1,"emojis":"🍄"}],"items":[` +
		`{"n":1,"name":"Gratin","label":"top chrono","option":false,"selected":true,"flags":[{"emoji":"🍄","word":"champignons"}],"selection":"","ingredients":"champignons de Paris","pantry":"","image":"https://x/1.webp"},` +
		`{"n":2,"name":"Filet","label":"viande 1","option":false,"selected":true,"flags":[],"selection":"","ingredients":"porc","pantry":"","image":"https://x/2.webp"},` +
		`{"n":3,"name":"Poisson","label":"","option":false,"selected":false,"flags":[],"selection":"","ingredients":"cabillaud","pantry":"","image":""},` +
		`{"n":10,"name":"Fruits","label":"9.10€","option":true,"selected":false,"flags":[],"selection":"Poires","ingredients":"","pantry":"","image":"https://x/10.webp"}]}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestHAJSONWithoutAlerts(t *testing.T) {
	got, err := haJSON(Page{Week: "19/10/2026"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `"alerts":[],"items":[]`; !strings.Contains(string(got), want) {
		t.Errorf("got %s, want empty lists, not null", got)
	}
}

func TestEverySiteReadIsPushed(t *testing.T) {
	url, pushes := fakeHA(t, http.StatusOK)
	s, _, _, _ := newService(t)
	s.HAWebhook = url

	if err := s.SendRecap(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.handle(context.Background(), Update{Message: &Message{Chat: Chat{ID: testChat}, Text: "/detail 1"}})
	s.handle(context.Background(), callback(cbValidate, recapMessage(Toggle(NewKeyboard(botPage().Items, false), 3))))

	got := pushes()
	if len(got) != 3 {
		t.Fatalf("%d pushes, want 3 (recap, detail, validate)", len(got))
	}
	// Pushed annotated: flags are set, as in the recap.
	if !strings.Contains(got[0], `"word":"champignons"`) {
		t.Errorf("push = %s", got[0])
	}
	if !strings.Contains(got[2], `"n":3,"name":"Poisson","label":"","option":false,"selected":true`) {
		t.Errorf("validate push = %s, want the applied state", got[2])
	}
}

func TestPushFailureIsOnlyLogged(t *testing.T) {
	url, _ := fakeHA(t, http.StatusInternalServerError)
	s, _, calls, log := newService(t)
	s.HAWebhook = url

	if err := s.SendRecap(context.Background()); err != nil {
		t.Fatalf("SendRecap = %v, want the recap sent anyway", err)
	}
	if last(calls).Method != "sendMessage" {
		t.Errorf("recap not sent: %+v", *calls)
	}
	if !strings.Contains(log.String(), "500") {
		t.Errorf("log = %q", log.String())
	}
}

func TestPushErrorHidesWebhookID(t *testing.T) {
	url, _ := fakeHA(t, http.StatusOK)
	s, _, _, log := newService(t)
	s.HAWebhook = strings.Replace(url, "http://", "http://unreachable.invalid.", 1)

	if err := s.SendRecap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if log.Len() == 0 || strings.Contains(log.String(), "SECRETID") {
		t.Errorf("log = %q, want an error without the webhook id", log.String())
	}
}

func TestPushWarnsAboutItemsWithoutSlot(t *testing.T) {
	url, pushes := fakeHA(t, http.StatusOK)
	s, _, _, log := newService(t)
	s.HAWebhook = url
	s.Fetch = func(context.Context) (Page, error) {
		p := botPage()
		p.Items = append(p.Items, Item{N: 26, Name: "Extra", Option: true})
		return p, nil
	}

	if err := s.SendRecap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "item 26") {
		t.Errorf("log = %q", log.String())
	}
	if got := pushes(); len(got) != 1 || !strings.Contains(got[0], `"n":26`) {
		t.Errorf("pushes = %v, want item 26 still pushed", got)
	}
}
