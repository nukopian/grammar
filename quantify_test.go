package grammar

import (
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// As in Raku, a quantifier straight after an anchor, <?>, <!> or a code
// assertion is an error, as each matches no text: Rakudo 2025.11 says
// "Can only quantify a construct that produces a match". Lookarounds and
// <?name> may be quantified, as may any of them in brackets, or with a :
// after, or as a separator, as Raku allows.
func TestQuantifyNothing(t *testing.T) {
	yes := Assertion("t", func(*scalar.SV) bool { return true })
	for _, body := range []string{`^* a`, `$? a`, `^^* a`, `$$+ a`, `<<* a`, `>>? a`, `^ ** 2 a`, `^*? a`,
		`<?>? a`, `<!>* a`, `<?{ t($/) }>? a`, `<!{ t($/) }>+ a`} {
		_, err := Compile(`grammar G { token TOP { `+body+` } }`, yes)
		if err == nil || !strings.Contains(err.Error(), "matches no text, so it can't be repeated") {
			t.Errorf("%s: %v", body, err)
		}
	}
	for _, body := range []string{`<?before a>* a`, `<!before b>? a`, `<?after x>? a`, `<?alpha>? a`, `<.alpha>? a`,
		`[ ^ ]? a`, `[ <?{ t($/) }> ]+ a`, `^: a`, `<?{ t($/) }>: a`, `a+ % ^`, `a+ % <?>`} {
		if _, err := Compile(`grammar G { token TOP { `+body+` } }`, yes); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
}
