package recettescabas

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

func testPage() Page {
	return Page{
		Week: "19/10/2026", Deadline: "Jeudi 15 Octobre 2026 à 23h00", People: "4 pers.",
		Items: []Item{
			{N: 1, Name: "Gratin", Label: "top chrono", Ingredients: "champignons de Paris", Image: "https://x/1.webp"},
			{N: 2, Name: "Filet", Label: "viande 1", Selected: true, Ingredients: "porc", Image: "https://x/2.webp"},
			{N: 3, Name: "Poisson", Label: "poisson", Ingredients: "cabillaud"},
			{N: 10, Name: "Fruits", Label: "Fruits (9.10€)", Option: true, Selection: "Poires", Image: "https://x/10.webp"},
		},
	}
}

// fakes records what the service asked of the site.
type fakes struct {
	fetches     int
	applyWeek   string
	applyWanted map[int]bool
	applyErr    error
}

// newService returns a service on a fake site: Apply selects item 3.
func newService(t *testing.T) (Service, *fakes, *bytes.Buffer) {
	t.Helper()
	f := &fakes{}
	var log bytes.Buffer
	s := Service{
		Config: testConfig(), Log: &log, site: &sync.Mutex{},
		Fetch: func(context.Context) (Page, error) { f.fetches++; return testPage(), nil },
		Apply: func(_ context.Context, week string, desired map[int]bool) (Page, error) {
			f.applyWeek, f.applyWanted = week, desired
			if f.applyErr != nil {
				return Page{}, f.applyErr
			}
			p := testPage()
			p.Items[2].Selected = true
			return p, nil
		},
	}
	return s, f, &log
}
