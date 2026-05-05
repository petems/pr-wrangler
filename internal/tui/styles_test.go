package tui

import "testing"

func TestThemeNamesMatchColorSchemes(t *testing.T) {
	if len(ThemeNames) != len(colorSchemes) {
		t.Fatalf("ThemeNames has %d entries, colorSchemes has %d; lists are out of sync",
			len(ThemeNames), len(colorSchemes))
	}
	for _, name := range ThemeNames {
		if _, ok := colorSchemes[name]; !ok {
			t.Errorf("ThemeNames lists %q but colorSchemes has no entry for it", name)
		}
	}
	for name := range colorSchemes {
		found := false
		for _, n := range ThemeNames {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("colorSchemes has %q but ThemeNames does not include it", name)
		}
	}
}
