package grammar_test

import (
	"fmt"

	"github.com/nukopian/scalar"

	"github.com/nukopian/grammar"
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

// AddAs gives phrases a value that isn't itself a phrase, such as a code.
func ExampleWordList_AddAs() {
	occupations := grammar.NewWordList("occupations", grammar.FoldCase()).
		AddAs("2134", "Software engineer", "Software developer").
		AddAs("8211", "HGV driver")

	fmt.Println(occupations.Canonical("software developer"), occupations.Canonical("HGV driver"))
	fmt.Println(occupations.Canonical("2134"), occupations.Phrases())
	// Output:
	// [2134] [8211]
	// [] [hgv driver software developer software engineer]
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

// Canonicals gives the canonical values in the order they were first
// added, so the order of a rules file can be used as data: here, which
// section wins when a heading has phrases of two, such as "Desirable
// skills".
func ExampleWordList_Canonicals() {
	sections := grammar.NewWordList("sections", grammar.FoldCase())
	sections.AddRules(`Desirable, nice to have, ideally
Requirements, skills, essential
Benefits, perks, what we offer
Requirements, experience`)

	rank := map[string]int{}
	for i, s := range sections.Canonicals() {
		rank[s] = i
	}
	fmt.Println(sections.Canonicals())
	fmt.Println(rank["Desirable"] < rank["Requirements"])
	// Output:
	// [Desirable Requirements Benefits]
	// true
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
// unless the list has JoinedSpacing too.
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

// Plurals lets each phrase match its plural, with the same canonical
// value. Only the last word changes, by the regular English rules; the
// plurals don't show in Phrases, which lists what was added.
func ExamplePlurals() {
	roles := grammar.NewWordList("roles", grammar.FoldCase(), grammar.WholeWords(), grammar.Plurals()).
		Add("Care Assistant", "carer").Add("Secretary").Add("Coach")
	g := grammar.MustCompile(`grammar R { token TOP { @roles } }`, roles)

	for _, text := range []string{"Care Assistants", "carers", "secretaries", "coaches", "Coachs"} {
		fmt.Printf("%-18q %v\n", text, g.Parse(text).At("made").Or("-"))
	}
	fmt.Println(roles.Phrases())
	// Output:
	// "Care Assistants"  Care Assistant
	// "carers"           Care Assistant
	// "secretaries"      Secretary
	// "coaches"          Coach
	// "Coachs"           -
	// [care assistant carer coach secretary]
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

// A pattern list holds patterns as data, each a regex in Raku syntax, for
// <@name> to match, as Raku's <@array> does: the longest match wins. Here,
// the ways a period is written, matched with their spacing and case.
func ExampleNewPatternList() {
	periods, err := grammar.NewPatternList("periods",
		`:i per \s* annum`, `:i 'p.a.'`, `:i pa >>`, `:i per \s* hour`)
	if err != nil {
		fmt.Println(err)
		return
	}
	g := grammar.MustCompile(`grammar Pay { token TOP { '£' $<amount>=[ \d+ ] \s* $<period>=<@periods> } }`, periods)
	for _, text := range []string{"£45000 per annum", "£45000 PA", "£18 per hour", "£45000 p.a."} {
		fmt.Println(g.Parse(text).At("period.~"))
	}

	_, err = grammar.NewPatternList("bad", `\d+ k`, `per [ annum`)
	fmt.Println(err)
	// Output:
	// per annum
	// PA
	// per hour
	// p.a.
	// pattern list bad: pattern "per [ annum": grammar: line 1, column 12: expected ]
}

// MustPatternList is NewPatternList, panicking if a pattern doesn't
// compile, for lists written into the program.
func ExampleMustPatternList() {
	amounts := grammar.MustPatternList("amounts", `\d+ k`, `\d ** 1..3 [ ',' \d ** 3 ]+`, `\d+`)
	g := grammar.MustCompile(`grammar A { token TOP { '£' <@amounts> } }`, amounts)
	fmt.Println(g.FindAll("£45k, £45,000 or £45").Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
	// Output:
	// £45k | £45,000 | £45
}

// Name is how Raku syntax refers to the list: <@periods>.
func ExamplePatternList_Name() {
	fmt.Println(grammar.MustPatternList("periods", `pa`).Name())
	// Output:
	// periods
}

// ExactAcronyms keeps a folded list's acronyms in capitals, so "IT", the
// field, isn't "it", the word.
func ExampleExactAcronyms() {
	l := grammar.NewWordList("fields", grammar.FoldCase(), grammar.WholeWords(), grammar.ExactAcronyms()).
		Add("IT").Add("computing")
	g := grammar.MustCompile(`grammar F { token TOP { @fields } }`, l)
	fmt.Println(g.FindAll("Is it IT or Computing?").Map(func(m *scalar.SV) any { return m.At("~") }).Join(", "))
	// Output:
	// IT, Computing
}

// MapOnly keeps the value of a phrases => value rule out of the phrases,
// so a section called legal isn't found in "Legal Secretary".
func ExampleMapOnly() {
	sections := grammar.NewWordList("sections", grammar.FoldCase(), grammar.WholeWords(), grammar.MapOnly())
	err := sections.AddRules(`
benefits, perks, what we offer
legal notice, the legal bit => legal
`)
	if err != nil {
		fmt.Println(err)
		return
	}
	g := grammar.MustCompile(`grammar Heading { token TOP { @sections } }`, sections)
	for _, heading := range []string{"Legal Secretary", "Benefits", "Legal Notice"} {
		fmt.Printf("%s: %v\n", heading, g.Find(heading).At("made").Or("no section"))
	}
	fmt.Println(sections.Canonicals())
	// Output:
	// Legal Secretary: no section
	// Benefits: benefits
	// Legal Notice: legal
	// [benefits legal]
}
