package cs2dialect_test

import (
	"strings"
	"testing"

	"magicstrike/pkg/cs2dialect"
)

func TestCS2Lexicon(t *testing.T) {
	lexicon := cs2dialect.NewCS2Lexicon()
	if lexicon == nil {
		t.Fatal("expected lexicon to be initialized, got nil")
	}

	t.Run("Lookup valid term", func(t *testing.T) {
		term, found := lexicon.Lookup("ace")
		if !found {
			t.Error("expected to find 'ace'")
		}
		if term.Term != "ace" {
			t.Errorf("expected term 'ace', got %q", term.Term)
		}
	})

	t.Run("Lookup case insensitivity and normalization", func(t *testing.T) {
		term, found := lexicon.Lookup("AcE")
		if !found {
			t.Error("expected to find normalized 'ace'")
		}
		if term.Category != "giria" {
			t.Errorf("expected category 'giria', got %q", term.Category)
		}
	})

	t.Run("Lookup invalid term", func(t *testing.T) {
		_, found := lexicon.Lookup("nonexistentterm")
		if found {
			t.Error("expected not to find 'nonexistentterm'")
		}
	})

	t.Run("LookupFuzzy by alias", func(t *testing.T) {
		results := lexicon.LookupFuzzy("neid")
		if len(results) == 0 {
			t.Fatal("expected fuzzy results for 'neid'")
		}
		found := false
		for _, r := range results {
			if r.Term == "ace" {
				found = true
			}
		}
		if !found {
			t.Error("expected 'ace' in fuzzy results")
		}
	})

	t.Run("LookupFuzzy no matches", func(t *testing.T) {
		results := lexicon.LookupFuzzy("invalidfuzzy")
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("AllByCategory", func(t *testing.T) {
		terms := lexicon.AllByCategory("giria")
		if len(terms) == 0 {
			t.Error("expected some terms for category 'giria'")
		}
		for _, term := range terms {
			if term.Category != "giria" {
				t.Errorf("expected term category to be 'giria', got %q", term.Category)
			}
		}
	})

	t.Run("GetSystemPromptContext", func(t *testing.T) {
		prompt := lexicon.GetSystemPromptContext()
		if !strings.Contains(prompt, "Você entende o dialeto") {
			t.Error("expected system prompt to contain basic prompt context")
		}
		if !strings.Contains(prompt, "ace/neide") {
			t.Error("expected system prompt to contain terms")
		}
	})
}
