package grammar_test

import (
	"fmt"

	"github.com/nukopian/job-meta/internal/scalar/grammar"
)

// A word list holds words and phrases, each with a canonical value, so that
// synonyms match as one thing. Pass it to Compile and match it with @name;
// a rule that's nothing but the list gets the canonical value of what it
// matched as its made value.
func ExampleNewWordList() {
	titles := grammar.NewWordList("titles")
	titles.Add("Software Engineer", "Software Developer", "SWE")
	titles.Add("Data Engineer", "Data Platform Engineer")

	g := grammar.MustCompile(`
grammar Title {
    token TOP   { <title> .* }      # a title, then anything
    token title { @titles }
}`, titles)

	for _, text := range []string{"SWE (Payments)", "Data Platform Engineer, Remote"} {
		fmt.Println(g.Parse(text).At("title.made"))
	}
	// Output:
	// Software Engineer
	// Data Engineer
}

// Words is @name for grammars built in Go.
func ExampleWords() {
	titles := grammar.NewWordList("titles").Add("Software Engineer", "SWE")
	g := grammar.New("Title").
		Token("TOP", grammar.Seq(grammar.Call("title"), grammar.Star(grammar.AnyChar))).
		Token("title", grammar.Words(titles))

	fmt.Println(g.Parse("SWE II").At("title.made"))
	// Output:
	// Software Engineer
}

// A list matches the longest phrase it can, whatever order they were added
// in, and like Raku's, never gives back part of it. It doesn't look at word
// boundaries: "Java" matches the start of "JavaScript", unless the rule
// says the word must end there, with >>.
func ExampleWords_longestAndBoundaries() {
	langs := grammar.NewWordList("langs").Add("Java").Add("JavaScript")
	longest := grammar.MustCompile(`grammar L { token TOP { @langs } }`, langs)
	fmt.Println(longest.Subparse("JavaScript developer").At("~"))

	java := grammar.NewWordList("java").Add("Java")
	loose := grammar.MustCompile(`grammar L2 { token TOP { @java } }`, java)
	whole := grammar.MustCompile(`grammar L3 { token TOP { @java >> } }`, java)
	fmt.Println(loose.Subparse("JavaScript").Defined(), whole.Subparse("JavaScript").Defined())
	// Output:
	// JavaScript
	// true false
}

// Add adds a canonical value and its synonyms. The canonical value is a
// phrase in the list too. Add returns the list, so calls can be chained.
func ExampleWordList_Add() {
	units := grammar.NewWordList("units").
		Add("Year", "annum", "pa").
		Add("Week", "wk")

	fmt.Println(units.Canonical("annum"), units.Canonical("Year"), units.Canonical("wk"))
	// Output:
	// [Year] [Year] [Week]
}

// AddRule takes one synonym rule in the format Elasticsearch and Solr use.
// A list of phrases means its first; phrases => phrases means the right
// side; a backslash escapes a comma or =>.
func ExampleWordList_AddRule() {
	l := grammar.NewWordList("units")
	for _, rule := range []string{
		"Year, annum, yr",       // annum and yr mean Year
		"hr, ph => Hour",        // hr and ph mean Hour, as does Hour
		`£\, per annum => Year`, // the phrase "£, per annum"
	} {
		if err := l.AddRule(rule); err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println(l.Canonical("yr"), l.Canonical("ph"), l.Canonical("£, per annum"))

	fmt.Println(l.AddRule("a => b => c"))
	// Output:
	// [Year] [Hour] [Year]
	// grammar: synonym rule "a => b => c" has more than one =>
}

// AddRules takes many rules, one per line, as Elasticsearch's synonyms API
// returns them. Blank lines and lines starting with # are skipped.
func ExampleWordList_AddRules() {
	units := grammar.NewWordList("units", grammar.FoldCase())
	err := units.AddRules(`
# periods
Year, annum, yr, pa, p.a.
Week, wk, pw
hour, hr, ph => Hour
`)
	if err != nil {
		fmt.Println(err)
		return
	}
	g := grammar.MustCompile(`
grammar Rate {
    token TOP  { '£' $<amount>=[ \d+ ] \s* [ per | a | '/' ]? \s* <unit> }
    token unit { @units }
}`, units)

	for _, text := range []string{"£450 per WK", "£18/hr", "£40000 p.a."} {
		m := g.Parse(text)
		fmt.Println(m.At("amount.~"), m.At("unit.made"))
	}
	// Output:
	// 450 Week
	// 18 Hour
	// 40000 Year
}

// Canonical says what a phrase means: everything it means, in the order
// added, or nothing if it isn't in the list. It reads the phrase as the list
// matches text, so with FoldCase and LooseSpacing it ignores case and
// spacing too.
func ExampleWordList_Canonical() {
	titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing()).
		Add("Front-end Developer", "Frontend Developer")

	fmt.Println(titles.Canonical("FRONT END developer"))
	fmt.Println(titles.Canonical("Back-end Developer") == nil)
	// Output:
	// [Front-end Developer]
	// true
}

// Ambiguous lists the phrases that mean more than one thing. A match of one
// has the first meaning as its made value; Canonical gives the rest, for the
// caller to choose from context, or to pass on to something that can.
func ExampleWordList_Ambiguous() {
	titles := grammar.NewWordList("titles", grammar.FoldCase())
	titles.Add("Product Manager", "PM")
	titles.Add("Project Manager", "PM")

	fmt.Println(titles.Ambiguous())

	g := grammar.MustCompile(`grammar T { token TOP { <title> }; token title { @titles } }`, titles)
	fmt.Println(g.Parse("PM").At("title.made"))
	// Output:
	// map[pm:[Product Manager Project Manager]]
	// Product Manager
}

// Phrases lists every phrase, as the list stores it: case folded with
// FoldCase, spacing collapsed with LooseSpacing.
func ExampleWordList_Phrases() {
	l := grammar.NewWordList("units", grammar.FoldCase()).Add("Year", "Annum", "p.a.")
	fmt.Println(l.Phrases())
	// Output:
	// [annum p.a. year]
}

// Len counts distinct phrases: a synonym added twice counts once.
func ExampleWordList_Len() {
	l := grammar.NewWordList("units").Add("Year", "annum").Add("Year", "annum", "yr")
	fmt.Println(l.Len())
	// Output:
	// 3
}

// Name is how Raku syntax refers to the list: @units.
func ExampleWordList_Name() {
	fmt.Println(grammar.NewWordList("units").Name())
	// Output:
	// units
}

// FoldCase makes a list ignore case wherever it's used. Without it, a list
// ignores case only where the rule says :i, as in Raku.
func ExampleFoldCase() {
	exact := grammar.NewWordList("exact").Add("Senior")
	folded := grammar.NewWordList("folded", grammar.FoldCase()).Add("Senior")
	g := grammar.MustCompile(`
grammar Seniority {
    token TOP    { @exact }
    token folded { @folded }
    token ci     { :i @exact }
}`, exact, folded)

	fmt.Println(g.Parse("SENIOR").Defined())
	fmt.Println(g.Parse("SENIOR", grammar.Rule("folded")).Defined())
	fmt.Println(g.Parse("SENIOR", grammar.Rule("ci")).Defined())
	// Output:
	// false
	// true
	// true
}

// LooseSpacing makes any run of spaces, hyphens, dashes and underscores
// match any other, as job titles are written every which way. There must
// still be some spacing: "frontend" is a different phrase from "front-end",
// so add both if both are used.
func ExampleLooseSpacing() {
	l := grammar.NewWordList("titles", grammar.LooseSpacing()).Add("front-end developer")
	g := grammar.MustCompile(`grammar T { token TOP { @titles } }`, l)

	for _, text := range []string{"front end developer", "front – end developer", "front_end  developer", "frontend developer"} {
		fmt.Println(g.Parse(text).Defined())
	}
	// Output:
	// true
	// true
	// true
	// false
}

// JoinedSpacing lets the text leave out a phrase's spacing, as postings
// write "on site", "on-site" and "onsite" for the same thing. Write the
// spaced form: there's no telling where a space would go in "onsite". With
// LooseSpacing too, one phrase covers every spelling, and the canonical
// value is the same for each.
func ExampleJoinedSpacing() {
	places := grammar.NewWordList("places", grammar.FoldCase(), grammar.LooseSpacing(),
		grammar.JoinedSpacing(), grammar.WholeWords()).
		Add("On-site", "on site", "office based")
	g := grammar.MustCompile(`grammar P { token TOP { @places } }`, places)

	for _, text := range []string{"Onsite", "on-site", "On Site", "office-based", "Officebased", "onsiteness"} {
		fmt.Printf("%-14q %v\n", text, g.Parse(text).At("made").Or("-"))
	}
	fmt.Println(places.Canonical("officebased"))
	// Output:
	// "Onsite"       On-site
	// "on-site"      On-site
	// "On Site"      On-site
	// "office-based" On-site
	// "Officebased"  On-site
	// "onsiteness"   -
	// [On-site]
}

// WholeWords matches a phrase only where it isn't part of a longer word,
// which << @list >> would do, without having to write the boundaries.
func ExampleWholeWords() {
	levels := grammar.NewWordList("levels", grammar.FoldCase(), grammar.WholeWords()).
		Add("Lead", "team lead", "team leader").Add("Senior", "snr")
	g := grammar.MustCompile(`grammar L { token TOP { @levels } }`, levels)

	for _, title := range []string{"Team Leader (Care)", "Snr Engineer", "Leadership Trainer", "SNR-level role"} {
		fmt.Printf("%-20q %v\n", title, g.Find(title).At("made").Or("-"))
	}
	// Output:
	// "Team Leader (Care)" Lead
	// "Snr Engineer"       Senior
	// "Leadership Trainer" -
	// "SNR-level role"     Senior
}

// WordLists makes a list for each set of synonym rules, ready for Compile:
// one call instead of a loop per list. A rule ending with a comma carries on
// onto the next line.
func ExampleWordLists() {
	uses, err := grammar.WordLists(map[string]string{
		"units": `Year, annum, yr, pa,
		          p.a., per annum
		          Week, wk`,
		"levels": "Senior, snr, sr\nJunior, jnr, graduate",
	}, grammar.FoldCase())
	if err != nil {
		fmt.Println(err)
		return
	}
	g := grammar.MustCompile(`
grammar Rate {
    token TOP   { <level> \s+ <unit> }
    token level { @levels }
    token unit  { @units }
}`, uses...)
	m := g.Parse("Snr per annum")
	fmt.Println(m.At("level.made"), m.At("unit.made"))
	// Output:
	// Senior Year
}

// A named capture of a word list, $<name>=@list, has the canonical value as
// its made value, so a list inside a bigger rule needs no rule of its own,
// and grammar.To can read it straight into a struct.
func ExampleNamed_wordList() {
	levels := grammar.NewWordList("levels", grammar.FoldCase()).Add("Senior", "snr", "sr")
	g := grammar.MustCompile(`grammar T { token TOP { $<level>=@levels? \s* $<role>=[ \N+ ] } }`, levels)

	for _, title := range []string{"Snr Software Engineer", "Software Engineer"} {
		t, _ := grammar.To[struct{ Level, Role string }](g.Parse(title))
		fmt.Printf("%+v\n", t)
	}
	// Output:
	// {Level:Senior Role:Software Engineer}
	// {Level: Role:Software Engineer}
}
