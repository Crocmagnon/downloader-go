package tohmo

import "testing"

func TestLatestInvoiceID(t *testing.T) {
	for body, want := range map[string]string{
		`[{"id":"a"},{"id":"b"}]`:                 "a",
		`{"data":[{"id":12345678}],"nextPage":2}`: "12345678",
		`{"items":[{"id":"x"}]}`:                  "x",
	} {
		if got, err := latestInvoiceID([]byte(body)); err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", body, got, err, want)
		}
	}
	if _, err := latestInvoiceID([]byte(`{"data":[]}`)); err == nil {
		t.Error("empty list: want error")
	}
}
