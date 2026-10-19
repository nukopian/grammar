package grammar

import (
	"testing"

	"github.com/nukopian/grammar/internal/docref"
)

// referenceGroups are the example files, in the order and under the
// headings they appear in the README's Reference.
var referenceGroups = []docref.Group{
	{File: "example_test.go", Title: "A worked example"},
	{File: "example_grammar_test.go", Title: "Writing grammars"},
	{File: "example_parse_test.go", Title: "Parsing and matches"},
	{File: "example_node_test.go", Title: "Building rules in Go"},
	{File: "example_words_test.go", Title: "Word lists"},
}

// Every exported function and method has an example.
func TestEveryFeatureHasExample(t *testing.T) { docref.CheckExamples(t, referenceGroups) }

// README.md's Reference section is what the examples generate. To update it:
//
//	go test -run TestReference -update
func TestReference(t *testing.T) { docref.CheckReadme(t, referenceGroups) }
