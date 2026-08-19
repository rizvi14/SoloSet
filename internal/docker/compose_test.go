package docker

import (
	"reflect"
	"testing"
)

func TestLineWriter_SplitsAcrossWrites(t *testing.T) {
	var got []string
	lw := &lineWriter{emit: func(s string) { got = append(got, s) }}

	// A line delivered in two writes should emit once, when the newline lands.
	lw.Write([]byte("hello wo"))
	lw.Write([]byte("rld\nsecond line\npartial"))
	lw.flush()

	want := []string{"hello world", "second line", "partial"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLineWriter_TrimsCarriageReturns(t *testing.T) {
	var got []string
	lw := &lineWriter{emit: func(s string) { got = append(got, s) }}
	lw.Write([]byte("windows line\r\n"))
	lw.flush()

	if len(got) != 1 || got[0] != "windows line" {
		t.Fatalf("got %q, want [\"windows line\"]", got)
	}
}

func TestLineWriter_FlushWithoutTrailingNewlineEmitsRemainder(t *testing.T) {
	var got []string
	lw := &lineWriter{emit: func(s string) { got = append(got, s) }}
	lw.Write([]byte("no newline"))
	if len(got) != 0 {
		t.Fatalf("expected nothing emitted before flush, got %q", got)
	}
	lw.flush()
	if len(got) != 1 || got[0] != "no newline" {
		t.Fatalf("got %q, want [\"no newline\"]", got)
	}
}

func TestFirstLine(t *testing.T) {
	cases := map[string]string{
		"single":              "single",
		"first\nsecond\nthird": "first",
		"":                    "",
	}
	for in, want := range cases {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
