package tui

import (
	"reflect"
	"testing"
)

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

func TestColorSchemesPopulateEveryToken(t *testing.T) {
	schemeType := reflect.TypeOf(ColorScheme{})
	for name, scheme := range colorSchemes {
		value := reflect.ValueOf(scheme)
		for i := 0; i < schemeType.NumField(); i++ {
			field := schemeType.Field(i)
			if value.Field(i).IsNil() {
				t.Errorf("color scheme %q leaves %s unset", name, field.Name)
			}
		}
	}
}
