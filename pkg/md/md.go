package md

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	cf "github.com/kentaro-m/blackfriday-confluence"
	bf "github.com/russross/blackfriday/v2"

	"github.com/rethab/jira-cli/pkg/md/jirawiki"
)

// mentionPattern matches Jira user mentions, e.g. [~displayname] or [~display name].
var mentionPattern = regexp.MustCompile(`\[~[^\[\]\n]+\]`)

// ToJiraMD translates CommonMark to Jira flavored markdown.
func ToJiraMD(md string) string {
	if md == "" {
		return md
	}

	// blackfriday-confluence's escaper backslash-escapes `[`, `~` and `]`,
	// which mangles a Jira mention like "[~displayname]" into
	// "\[\~displayname\]" and stops it from pinging the user. Swap mentions
	// out for placeholders that survive the render untouched, then restore
	// them afterwards.
	nonce := placeholderNonce()
	var mentions []string
	protected := mentionPattern.ReplaceAllStringFunc(md, func(mention string) string {
		mentions = append(mentions, mention)
		return placeholder(nonce, len(mentions)-1)
	})

	renderer := &cf.Renderer{Flags: cf.IgnoreMacroEscaping}
	r := bf.New(bf.WithRenderer(renderer), bf.WithExtensions(bf.CommonExtensions))

	out := string(renderer.Render(r.Parse([]byte(protected))))

	for i, mention := range mentions {
		out = strings.ReplaceAll(out, placeholder(nonce, i), mention)
	}

	return out
}

// placeholderNonce returns a random token used to make mention placeholders
// practically impossible to collide with real input text.
func placeholderNonce() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a fixed token rather than failing
		// the whole render.
		return "jiraclimention"
	}
	return hex.EncodeToString(b)
}

// placeholder returns a marker that is unlikely to appear in real markdown
// and contains no characters that the confluence renderer would escape.
func placeholder(nonce string, i int) string {
	return fmt.Sprintf("%s%d", nonce, i)
}

// FromJiraMD translates Jira flavored markdown to CommonMark.
func FromJiraMD(jfm string) string {
	return jirawiki.Parse(jfm)
}
