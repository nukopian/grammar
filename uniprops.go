package grammar

import (
	"strings"
	"sync"
	"unicode"
)

// Unicode properties, for <:Lu>, <:Letter>, <:Latin>, <:Script<Latin>> and
// the like: general categories by short or long name, scripts, and binary
// properties, from Go's unicode tables. Names match loosely, as Unicode's
// do: case, spaces, hyphens and underscores don't matter, so <:lu> and
// <:uppercase-letter> are <:Lu>.

// categoryNames are the long names of the general categories, as Unicode
// gives them; Go's tables have only the short ones.
var categoryNames = map[string]string{
	"Letter": "L", "Uppercase_Letter": "Lu", "Lowercase_Letter": "Ll", "Titlecase_Letter": "Lt",
	"Modifier_Letter": "Lm", "Other_Letter": "Lo",
	"Mark": "M", "Nonspacing_Mark": "Mn", "Spacing_Mark": "Mc", "Enclosing_Mark": "Me",
	"Number": "N", "Decimal_Number": "Nd", "Letter_Number": "Nl", "Other_Number": "No",
	"Punctuation": "P", "Connector_Punctuation": "Pc", "Dash_Punctuation": "Pd", "Open_Punctuation": "Ps",
	"Close_Punctuation": "Pe", "Initial_Punctuation": "Pi", "Final_Punctuation": "Pf", "Other_Punctuation": "Po",
	"Symbol": "S", "Math_Symbol": "Sm", "Currency_Symbol": "Sc", "Modifier_Symbol": "Sk", "Other_Symbol": "So",
	"Separator": "Z", "Space_Separator": "Zs", "Line_Separator": "Zl", "Paragraph_Separator": "Zp",
	"Other": "C", "Control": "Cc", "Format": "Cf", "Surrogate": "Cs", "Private_Use": "Co",
}

// derived are the binary properties Unicode derives from others, which
// Go's tables don't have, as Unicode's DerivedCoreProperties defines them.
var derived = map[string]func(rune) bool{
	"Alphabetic": func(r rune) bool {
		return unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_Alphabetic, r)
	},
	"Uppercase": func(r rune) bool { return unicode.IsUpper(r) || unicode.Is(unicode.Other_Uppercase, r) },
	"Lowercase": func(r rune) bool { return unicode.IsLower(r) || unicode.Is(unicode.Other_Lowercase, r) },
}

// propertyAliases are Unicode's short names for the binary properties
// above, and for White_Space.
var propertyAliases = map[string]string{
	"Alpha": "Alphabetic", "Upper": "Uppercase", "Lower": "Lowercase", "WSpace": "White_Space", "Space": "White_Space",
}

var (
	propertiesOnce sync.Once
	properties     map[string]func(rune) bool // by loose name
	scripts        map[string]func(rune) bool // by loose name
)

// looseName is name as Unicode compares property names: ignoring case,
// spaces, hyphens and underscores.
func looseName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == ' ' {
			return -1
		}
		return unicode.ToLower(r)
	}, name)
}

func loadProperties() {
	table := func(t *unicode.RangeTable) func(rune) bool { return func(r rune) bool { return unicode.Is(t, r) } }
	properties, scripts = map[string]func(rune) bool{}, map[string]func(rune) bool{}
	for name, t := range unicode.Scripts {
		scripts[looseName(name)] = table(t)
		properties[looseName(name)] = table(t)
	}
	for name, t := range unicode.Properties {
		properties[looseName(name)] = table(t)
	}
	for name, f := range derived {
		properties[looseName(name)] = f
	}
	for alias, name := range propertyAliases {
		properties[looseName(alias)] = properties[looseName(name)]
	}
	// Categories last: a one- or two-letter name is a category first.
	for name, t := range unicode.Categories {
		properties[looseName(name)] = table(t)
	}
	for long, short := range categoryNames {
		if t, ok := unicode.Categories[short]; ok {
			properties[looseName(long)] = table(t)
		}
	}
}

// unicodeProperty returns the test for a property, or false if there's no
// such property.
func unicodeProperty(name string) (func(rune) bool, bool) {
	propertiesOnce.Do(loadProperties)
	f, ok := properties[looseName(name)]
	return f, ok
}

// unicodeScript returns the test for a script, as <:Script<Latin>> names
// it, or false if there's no such script.
func unicodeScript(name string) (func(rune) bool, bool) {
	propertiesOnce.Do(loadProperties)
	f, ok := scripts[looseName(name)]
	return f, ok
}
