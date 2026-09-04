package acl

import "testing"

func TestTodayFormat(t *testing.T) {
	value := Today()
	if len(value) != len("2006-01-02") { t.Fatalf("unexpected date: %q", value) }
}
