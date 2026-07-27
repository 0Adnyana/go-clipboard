package slug_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/0adnyana/go-clipboard/internal/slug"
)

func TestValidate_validSlugs(t *testing.T) {
	cases := []string{
		"abc",
		"a_b-c",
		"Notes",
		"notes",
		strings.Repeat("a", slug.MaxLength),
	}
	for _, s := range cases {
		if err := slug.Validate(s); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", s, err)
		}
	}
}

func TestValidate_rejectsTooShort(t *testing.T) {
	for _, s := range []string{"", "a", "ab"} {
		err := slug.Validate(s)
		if err == nil {
			t.Fatalf("Validate(%q) = nil, want error", s)
		}
		var ve *slug.ValidationError
		if !errors.As(err, &ve) || ve.Code != "invalid_slug" {
			t.Fatalf("Validate(%q) = %v, want invalid_slug", s, err)
		}
	}
}

func TestValidate_rejectsTooLong(t *testing.T) {
	err := slug.Validate(strings.Repeat("a", slug.MaxLength+1))
	if err == nil {
		t.Fatal("Validate(too long) = nil, want error")
	}
	var ve *slug.ValidationError
	if !errors.As(err, &ve) || ve.Code != "invalid_slug" {
		t.Fatalf("Validate(too long) = %v, want invalid_slug", err)
	}
}

func TestValidate_rejectsIllegalCharset(t *testing.T) {
	cases := []string{
		"has space",
		"slash/name",
		"dot.name",
		"unicode-☃",
		"tab\tchar",
	}
	for _, s := range cases {
		err := slug.Validate(s)
		if err == nil {
			t.Fatalf("Validate(%q) = nil, want error", s)
		}
		var ve *slug.ValidationError
		if !errors.As(err, &ve) || ve.Code != "invalid_slug" {
			t.Fatalf("Validate(%q) = %v, want invalid_slug", s, err)
		}
	}
}

func TestValidate_preservesCase(t *testing.T) {
	const mixed = "MyClip-01"
	if err := slug.Validate(mixed); err != nil {
		t.Fatalf("Validate(%q) = %v, want nil", mixed, err)
	}
}

func TestValidate_rejectsReservedExactMatch(t *testing.T) {
	for _, name := range slug.Reserved {
		err := slug.Validate(name)
		if err == nil {
			t.Fatalf("Validate(%q) = nil, want reserved error", name)
		}
		var ve *slug.ValidationError
		if !errors.As(err, &ve) || ve.Code != "reserved_slug" {
			t.Fatalf("Validate(%q) = %v, want reserved_slug", name, err)
		}
	}
}

func TestValidate_reservedIsCaseSensitive(t *testing.T) {
	if err := slug.Validate("Status"); err != nil {
		t.Fatalf("Validate(Status) = %v, want nil (case-sensitive reservation only)", err)
	}
	if err := slug.Validate("STATUS"); err != nil {
		t.Fatalf("Validate(STATUS) = %v, want nil (case-sensitive reservation only)", err)
	}
}

func TestIsReserved_exactMatchOnly(t *testing.T) {
	if !slug.IsReserved("status") {
		t.Fatal("IsReserved(status) = false, want true")
	}
	if slug.IsReserved("Status") {
		t.Fatal("IsReserved(Status) = true, want false")
	}
}
