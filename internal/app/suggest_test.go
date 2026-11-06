package app

import (
	"slices"
	"testing"
)

// Clearing and retyping a query must not let the dismissed request replace
// its successor's suggestions or clear its loading state.
func TestSuggestionsIgnoreDismissedRequest(t *testing.T) {
	for _, olderFirst := range []bool{true, false} {
		name := "newer-finishes-first"
		if olderFirst {
			name = "older-finishes-first"
		}
		t.Run(name, func(t *testing.T) {
			a := newTestApp()
			posted := posts(a)
			asked := make(chan chan []string, 2)
			fake(a).suggest = func(string) ([]string, error) {
				reply := make(chan []string, 1)
				asked <- reply
				return <-reply, nil
			}
			a.suggest("same query")
			older := wait(t, asked, "the first suggestion request")
			defer close(older)
			a.dismissSuggestions()
			a.suggest("same query")
			newer := wait(t, asked, "the replacement suggestion request")
			defer close(newer)
			if olderFirst {
				older <- []string{"stale"}
				runPost(t, posted)
				if !a.suggestions.loading || len(a.suggestions.items) != 0 {
					t.Errorf("dismissed request changed the replacement: loading=%v, items=%v", a.suggestions.loading, a.suggestions.items)
				}
			}
			newer <- []string{"current"}
			runPost(t, posted)
			if !olderFirst {
				older <- []string{"stale"}
				runPost(t, posted)
			}
			if !slices.Equal(a.suggestions.items, []string{"current"}) {
				t.Fatalf("suggestions=%v, want only the replacement request's results", a.suggestions.items)
			}
		})
	}
}
