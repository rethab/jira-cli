package mention

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/jira"
)

var (
	jane   = &jira.User{AccountID: "acc-jane", Name: "jane", DisplayName: "Jane Doe", Email: "jane@example.com", Active: true}
	janet  = &jira.User{AccountID: "acc-janet", Name: "janet", DisplayName: "Janet Roe", Active: true}
	gone   = &jira.User{AccountID: "acc-gone", Name: "gone", DisplayName: "Jane Gone", Active: false}
	twin1  = &jira.User{AccountID: "acc-t1", Name: "t1", DisplayName: "Sam Lee", Email: "sam1@example.com", Active: true}
	twin2  = &jira.User{AccountID: "acc-t2", Name: "t2", DisplayName: "Sam Lee", Active: true}
	hidden = &jira.User{AccountID: "acc-hidden", Name: "hidden", DisplayName: "Hidden Mail", Active: true}
)

func lookupOf(users ...*jira.User) (Lookup, *[]string) {
	var calls []string
	return func(q string) ([]*jira.User, error) {
		calls = append(calls, q)
		return users, nil
	}, &calls
}

func TestResolveOutput(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		local bool
		users []*jira.User
		want  string
	}{
		{name: "cloud", body: "hi @[Jane Doe], ok?", users: []*jira.User{jane}, want: "hi [~accountid:acc-jane], ok?"},
		{name: "local", body: "hi @[Jane Doe]", local: true, users: []*jira.User{jane}, want: "hi [~jane]"},
		{name: "no mention", body: "plain text with [~jane] and @jane", users: []*jira.User{jane}, want: "plain text with [~jane] and @jane"},
		{name: "empty", body: "", users: []*jira.User{jane}, want: ""},
		{name: "exact match among several", body: "@[Jane Doe]", users: []*jira.User{janet, jane}, want: "[~accountid:acc-jane]"},
		{name: "exact by email", body: "@[JANE@example.com]", users: []*jira.User{janet, jane}, want: "[~accountid:acc-jane]"},
		{name: "exact by username on local", body: "@[jane]", local: true, users: []*jira.User{janet, jane}, want: "[~jane]"},
		{name: "single fuzzy result", body: "@[jane@elsewhere.com]", users: []*jira.User{hidden}, want: "[~accountid:acc-hidden]"},
		{name: "inactive ignored", body: "@[Jane]", users: []*jira.User{gone, janet}, want: "[~accountid:acc-janet]"},
		{name: "query is trimmed", body: "@[ Jane Doe ]", users: []*jira.User{jane}, want: "[~accountid:acc-jane]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup, _ := lookupOf(tc.users...)

			got, err := Resolve(tc.body, tc.local, lookup, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveSkipsCodeAndLinks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "backtick fence", body: "```\n@[Jane Doe]\n```\n@[Jane Doe]", want: "```\n@[Jane Doe]\n```\n[~accountid:acc-jane]"},
		{name: "tilde fence", body: "~~~go\n@[Jane Doe]\n~~~", want: "~~~go\n@[Jane Doe]\n~~~"},
		{name: "longer fence is not closed by a shorter one", body: "````\n```\n@[Jane Doe]\n````", want: "````\n```\n@[Jane Doe]\n````"},
		{name: "unclosed fence runs to the end", body: "```\n@[Jane Doe]", want: "```\n@[Jane Doe]"},
		{name: "indented code", body: "text\n\n    @[Jane Doe]\n\nafter @[Jane Doe]", want: "text\n\n    @[Jane Doe]\n\nafter [~accountid:acc-jane]"},
		{name: "tab indented code", body: "\t@[Jane Doe]", want: "\t@[Jane Doe]"},
		{name: "indented continuation is not code", body: "text\n    @[Jane Doe]", want: "text\n    [~accountid:acc-jane]"},
		{name: "inline code", body: "use `@[Jane Doe]` or @[Jane Doe]", want: "use `@[Jane Doe]` or [~accountid:acc-jane]"},
		{name: "double backtick code", body: "``a ` @[Jane Doe]`` @[Jane Doe]", want: "``a ` @[Jane Doe]`` [~accountid:acc-jane]"},
		{name: "unclosed backtick is text", body: "a ` @[Jane Doe]", want: "a ` [~accountid:acc-jane]"},
		{name: "markdown link", body: "see @[docs](https://example.com)", want: "see @[docs](https://example.com)"},
		{name: "escaped", body: `\@[Jane Doe]`, want: `\@[Jane Doe]`},
		{name: "existing markup", body: "[~accountid:other] and @[Jane Doe]", want: "[~accountid:other] and [~accountid:acc-jane]"},
		{name: "nested bracket", body: "@[a [b] c]", want: "@[a [b] c]"},
		{name: "empty", body: "@[] @[  ]", want: "@[] @[  ]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup, _ := lookupOf(jane)

			got, err := Resolve(tc.body, false, lookup, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveLooksUpEachQueryOnce(t *testing.T) {
	lookup, calls := lookupOf(jane)

	got, err := Resolve("@[Jane Doe] and @[jane doe] and @[Jane Doe]", false, lookup, nil)
	require.NoError(t, err)
	assert.Equal(t, "[~accountid:acc-jane] and [~accountid:acc-jane] and [~accountid:acc-jane]", got)
	assert.Equal(t, []string{"Jane Doe"}, *calls)
}

func TestResolveErrors(t *testing.T) {
	t.Run("no user", func(t *testing.T) {
		lookup, _ := lookupOf()

		_, err := Resolve("@[Nobody]", false, lookup, nil)
		assert.EqualError(t, err, "no user found for mention @[Nobody]")
	})

	t.Run("only inactive users", func(t *testing.T) {
		lookup, _ := lookupOf(gone)

		_, err := Resolve("@[Jane Gone]", false, lookup, nil)
		assert.EqualError(t, err, "no user found for mention @[Jane Gone]")
	})

	t.Run("ambiguous without pick lists the candidates", func(t *testing.T) {
		lookup, _ := lookupOf(jane, janet)

		_, err := Resolve("@[Jan]", false, lookup, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mention @[Jan] is ambiguous")
		assert.Contains(t, err.Error(), "\n  Jane Doe <jane@example.com> (acc-jane)\n  Janet Roe (acc-janet)")
	})

	t.Run("candidates fall back to the name without an account id", func(t *testing.T) {
		local1 := &jira.User{Name: "a", DisplayName: "A One", Active: true}
		local2 := &jira.User{Name: "b", DisplayName: "A Two", Active: true}
		lookup, _ := lookupOf(local1, local2)

		_, err := Resolve("@[Al]", true, lookup, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "  A One (a)\n  A Two (b)")
	})

	t.Run("lookup failure", func(t *testing.T) {
		boom := errors.New("boom")
		lookup := func(string) ([]*jira.User, error) { return nil, boom }

		_, err := Resolve("@[Jane]", false, lookup, nil)
		assert.ErrorIs(t, err, boom)
	})

	t.Run("one failure aborts everything", func(t *testing.T) {
		lookup := func(q string) ([]*jira.User, error) {
			if q == "Jane Doe" {
				return []*jira.User{jane}, nil
			}
			return nil, nil
		}

		got, err := Resolve("@[Jane Doe] @[Nobody]", false, lookup, nil)
		assert.Error(t, err)
		assert.Empty(t, got)
	})
}

func TestResolvePick(t *testing.T) {
	t.Run("picks among the candidates", func(t *testing.T) {
		lookup, _ := lookupOf(jane, janet)

		var offered []*jira.User
		pick := func(_ string, candidates []*jira.User) (*jira.User, error) {
			offered = candidates
			return candidates[1], nil
		}

		got, err := Resolve("@[Jan] and @[jan]", false, lookup, pick)
		require.NoError(t, err)
		assert.Equal(t, "[~accountid:acc-janet] and [~accountid:acc-janet]", got)
		assert.Equal(t, []*jira.User{jane, janet}, offered)
	})

	t.Run("only the exact matches are offered when several share the name", func(t *testing.T) {
		lookup, _ := lookupOf(twin1, janet, twin2)

		var offered []*jira.User
		pick := func(_ string, candidates []*jira.User) (*jira.User, error) {
			offered = candidates
			return candidates[0], nil
		}

		_, err := Resolve("@[Sam Lee]", false, lookup, pick)
		require.NoError(t, err)
		assert.Equal(t, []*jira.User{twin1, twin2}, offered)
	})

	t.Run("pick error aborts", func(t *testing.T) {
		lookup, _ := lookupOf(jane, janet)
		boom := errors.New("interrupted")

		_, err := Resolve("@[Jan]", false, lookup, func(string, []*jira.User) (*jira.User, error) { return nil, boom })
		assert.ErrorIs(t, err, boom)
	})

	t.Run("pick without a choice aborts", func(t *testing.T) {
		lookup, _ := lookupOf(jane, janet)

		_, err := Resolve("@[Jan]", false, lookup, func(string, []*jira.User) (*jira.User, error) { return nil, nil })
		assert.Error(t, err)
	})

	t.Run("pick is not called for an unambiguous mention", func(t *testing.T) {
		lookup, _ := lookupOf(jane)

		_, err := Resolve("@[Jane Doe]", false, lookup, func(string, []*jira.User) (*jira.User, error) {
			t.Fatal("pick called")
			return nil, nil
		})
		assert.NoError(t, err)
	})
}

func TestHas(t *testing.T) {
	assert.True(t, Has("hi @[Jane]"))
	assert.False(t, Has("hi @Jane"))
	assert.False(t, Has("`@[Jane]`"))
	assert.False(t, Has(""))
}
