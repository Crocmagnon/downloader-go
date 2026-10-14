package recettescabas

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testToken = "tok"

func post(t *testing.T, srv *httptest.Server, path, auth, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

type apiFixture struct {
	srv    *httptest.Server
	fakes  *fakes
	calls  *[]tgCall
	pushes func() []string
	svc    *Service
}

func newAPI(t *testing.T) apiFixture {
	t.Helper()
	s, f, calls, _ := newService(t)
	url, pushes := fakeHA(t, http.StatusOK)
	s.HAWebhook = url
	fx := apiFixture{fakes: f, calls: calls, pushes: pushes, svc: &s}
	// Built per request so that tests may change the service after newAPI.
	fx.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fx.svc.API(testToken).ServeHTTP(w, r)
	}))
	t.Cleanup(fx.srv.Close)
	return fx
}

func TestAPIRequiresToken(t *testing.T) {
	fx := newAPI(t)
	for _, auth := range []string{"", "Bearer nope", "tok", "bearer tok"} {
		if code, body := post(t, fx.srv, "/refresh", auth, ""); code != http.StatusUnauthorized || body != "" {
			t.Errorf("auth %q: %d %q", auth, code, body)
		}
	}
	if fx.fakes.fetches != 0 {
		t.Error("unauthenticated request read the site")
	}
}

func TestAPIEmptyTokenNeverAuthenticates(t *testing.T) {
	s, _, _, _ := newService(t)
	srv := httptest.NewServer(s.API(""))
	t.Cleanup(srv.Close)
	if code, _ := post(t, srv, "/refresh", "Bearer ", ""); code != http.StatusUnauthorized {
		t.Errorf("code = %d", code)
	}
}

func TestAPIOnlyPost(t *testing.T) {
	fx := newAPI(t)
	req, _ := http.NewRequest(http.MethodGet, fx.srv.URL+"/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := fx.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("code = %d", resp.StatusCode)
	}
}

func TestAPIRefresh(t *testing.T) {
	fx := newAPI(t)
	if code, _ := post(t, fx.srv, "/refresh", "Bearer "+testToken, ""); code != http.StatusNoContent {
		t.Fatalf("code = %d", code)
	}
	if fx.fakes.fetches != 1 || len(fx.pushes()) != 1 {
		t.Errorf("fetches = %d, pushes = %d", fx.fakes.fetches, len(fx.pushes()))
	}
	if len(*fx.calls) != 0 {
		t.Errorf("refresh talked to Telegram: %+v", *fx.calls)
	}
}

func TestAPISiteFailure(t *testing.T) {
	fx := newAPI(t)
	fx.svc.Fetch = func(context.Context) (Page, error) {
		return Page{}, errors.New("waiting for preferences form: timeout\nCall log:\n  - navigating to …")
	}
	code, body := post(t, fx.srv, "/refresh", "Bearer "+testToken, "")
	if code != http.StatusBadGateway || body != "échec : waiting for preferences form: timeout\n" {
		t.Errorf("%d %q", code, body)
	}
	if len(fx.pushes()) != 0 {
		t.Error("failed read was pushed")
	}
}

func TestAPIApplyRejectsBadInput(t *testing.T) {
	for _, body := range []string{
		``,
		`not json`,
		`{"week":"19/10/2026","desired":{}}`,
		`{"week":"19/10/2026"}`,
		`{"week":"19/10/2026","desired":{"x":true}}`,
		`{"week":"19/10/2026","desired":{"3":"yes"}}`,
		`{"desired":{"3":true}}`,
		`{"week":"semaine du 19/10/2026","desired":{"3":true}}`,
		`{"week":"19/10/2026","desired":{"3":true},"extra":1}`,
		`{"week":"19/10/2026","desired":{"3":true},"pad":"` + strings.Repeat("x", maxAPIBody) + `"}`,
	} {
		fx := newAPI(t)
		if code, _ := post(t, fx.srv, "/apply", "Bearer "+testToken, body); code != http.StatusBadRequest {
			t.Errorf("%.60s: code = %d", body, code)
		}
		if fx.fakes.applyWanted != nil {
			t.Errorf("%.60s: applied", body)
		}
	}
}

func TestAPIApply(t *testing.T) {
	fx := newAPI(t)
	code, _ := post(t, fx.srv, "/apply", "Bearer "+testToken, `{"week":"19/10/2026","desired":{"3":true,"2":false}}`)
	if code != http.StatusOK {
		t.Fatalf("code = %d", code)
	}
	if fx.fakes.applyWeek != "19/10/2026" || !maps.Equal(fx.fakes.applyWanted, map[int]bool{3: true, 2: false}) {
		t.Errorf("apply(%q, %v)", fx.fakes.applyWeek, fx.fakes.applyWanted)
	}
	if got := fx.pushes(); len(got) != 1 || !strings.Contains(got[0], `"name":"Poisson","label":"","option":false,"selected":true`) {
		t.Errorf("pushes = %v", got)
	}

	var methods []string
	for _, c := range *fx.calls {
		methods = append(methods, c.Method)
	}
	if strings.Join(methods, ",") != "sendMessage,sendMediaGroup,sendMessage" {
		t.Fatalf("telegram calls = %v", methods)
	}
	if c := (*fx.calls)[0]; !strings.Contains(c.Body, "✏️ Sélection modifiée depuis Home Assistant") {
		t.Errorf("header = %s", c.Body)
	}
	if c := last(fx.calls); !strings.Contains(c.Body, "✅ 3. Poisson") {
		t.Errorf("recap = %s", c.Body)
	}
}

func TestAPIApplyWeekChanged(t *testing.T) {
	fx := newAPI(t)
	fx.fakes.applyErr = ErrWeekChanged
	code, body := post(t, fx.srv, "/apply", "Bearer "+testToken, `{"week":"12/10/2026","desired":{"3":true}}`)
	if code != http.StatusConflict || !strings.Contains(body, "semaine a changé") {
		t.Errorf("%d %q", code, body)
	}
	if len(*fx.calls) != 0 || len(fx.pushes()) != 0 {
		t.Errorf("calls = %+v, pushes = %v", *fx.calls, fx.pushes())
	}
}

func TestAPIApplySiteFailure(t *testing.T) {
	fx := newAPI(t)
	fx.fakes.applyErr = errors.New("item 3: the site did not save the change")
	code, body := post(t, fx.srv, "/apply", "Bearer "+testToken, `{"week":"19/10/2026","desired":{"3":true}}`)
	if code != http.StatusBadGateway || body != "échec : item 3: the site did not save the change\n" {
		t.Errorf("%d %q", code, body)
	}
	if len(*fx.calls) != 0 {
		t.Errorf("calls = %+v", *fx.calls)
	}
}

func TestSiteAccessIsSerialized(t *testing.T) {
	s, _, _, _ := newService(t)
	var running, overlaps atomic.Int32
	slow := func() {
		if running.Add(1) > 1 {
			overlaps.Add(1)
		}
		time.Sleep(20 * time.Millisecond)
		running.Add(-1)
	}
	s.Fetch = func(context.Context) (Page, error) { slow(); return botPage(), nil }
	s.Apply = func(context.Context, string, map[int]bool) (Page, error) { slow(); return botPage(), nil }

	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() { _, _ = s.fetch(context.Background()) })
		wg.Go(func() { _, _ = s.apply(context.Background(), "19/10/2026", map[int]bool{3: true}) })
	}
	wg.Wait()
	if n := overlaps.Load(); n != 0 {
		t.Errorf("%d overlapping site accesses", n)
	}
}

func TestCheckBridge(t *testing.T) {
	if on, err := CheckBridge("", "", ""); on || err != nil {
		t.Errorf("unset: %v, %v", on, err)
	}
	if on, err := CheckBridge("127.0.0.1:8789", "t", "https://ha/api/webhook/x"); !on || err != nil {
		t.Errorf("set: %v, %v", on, err)
	}
	_, err := CheckBridge("127.0.0.1:8789", "", "https://ha/api/webhook/x")
	if err == nil || !strings.Contains(err.Error(), "RC_API_TOKEN") || strings.Contains(err.Error(), "--api-listen") {
		t.Errorf("partial: %v", err)
	}
}
