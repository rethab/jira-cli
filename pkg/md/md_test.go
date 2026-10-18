package md

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An image must not swallow the block separation that follows it: in Jira wiki
// a table or list that doesn't start on its own line is not rendered.
func TestToJiraMDKeepsBlockSeparationAfterImage(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "image alone",
			input:    "![](i.png)\n",
			expected: "!i.png!",
		},
		{
			name:     "image followed by paragraph and table",
			input:    "![](i.png)\n\nsome text\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n",
			expected: "!i.png!\n\nsome text\n\n||a||b||\n|1|2|",
		},
		{
			name:     "image followed by paragraph and list",
			input:    "![](i.png)\n\nsome text\n\n* a\n* b\n",
			expected: "!i.png!\n\nsome text\n\n* a\n* b",
		},
		{
			name:     "image with alt text followed by table",
			input:    "![description](i.png)\n\n| a |\n| --- |\n| 1 |\n",
			expected: "!i.png!\n\n||a||\n|1|",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToJiraMD(tc.input))
		})
	}
}

// Plain text doesn't count as output for the renderer's block separation, so
// a heading or list after nothing but plain paragraphs must still start on its
// own line. Otherwise it is glued onto the preceding paragraph line and Jira
// renders it as literal text.
func TestToJiraMDKeepsBlockSeparationAfterPlainText(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "paragraph followed by list",
			input:    "some text\n\n* a\n* b\n",
			expected: "some text\n\n* a\n* b",
		},
		{
			name:     "paragraph followed by heading",
			input:    "some text\n\n### Title\n\nmore text\n",
			expected: "some text\n\nh3. Title\nmore text",
		},
		{
			name:     "paragraphs followed by heading and list",
			input:    "some text\n\nmore text\n\n### Title\n\n- a\n- b\n",
			expected: "some text\n\nmore text\n\nh3. Title\n* a\n* b",
		},
		{
			name:     "paragraph followed by table",
			input:    "some text\n\n| a |\n| --- |\n| 1 |\n",
			expected: "some text\n\n||a||\n|1|",
		},
		{
			name:     "paragraph followed by code block",
			input:    "some text\n\n```\ncode\n```\n",
			expected: "some text\n\n{code}\ncode\n{code}",
		},
		{
			name:     "paragraph followed by horizontal rule",
			input:    "some text\n\n---\n\nmore text\n",
			expected: "some text\n\n\n----\nmore text",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToJiraMD(tc.input))
		})
	}
}

func TestToJiraMD(t *testing.T) {
	jfm := `# H1
Some _Markdown_ text.

## H2
Foobar.

### H3
Fuga

> quote

- - - -

**strong text**
~~strikethrough text~~
[Example Domain](http://www.example.com/)
![](https://path.to/image.jpg)
![description](https://google.com)

* list1
* list2
* list3

Paragraph

1. number1
2. number2
3. number3

|a  |b  |c  |
|---|---|---|
|1  |2  |3  |
|4  |5  |6  |

{panel:title=My Title}
**Subtitle**

Some text with a title
{panel}

` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("hello world")
}` + "```"

	expected := `h1. H1
Some _Markdown_ text.

h2. H2
Foobar.

h3. H3
Fuga

{quote}
quote

{quote}


----
*strong text*
-strikethrough text-
[Example Domain|http://www.example.com/]
!https://path.to/image.jpg!
!https://google.com!

* list1
* list2
* list3

Paragraph

# number1
# number2
# number3

||a||b||c||
|1|2|3|
|4|5|6|

{panel:title=My Title}
*Subtitle*

Some text with a title
{panel}

` + "```go" + `
package main

import "fmt"

func main\(\) {
    fmt.Println\("hello world"\)
}` + "```"

	assert.Equal(t, expected, ToJiraMD(jfm))
}

func TestToJiraMDPreservesMentions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain mention",
			input:    `[~ankit]`,
			expected: "[~ankit]",
		},
		{
			name:     "mention with display name containing spaces",
			input:    `[~display name with spaces]`,
			expected: "[~display name with spaces]",
		},
		{
			name:     "mention inline in text",
			input:    `Hey [~ankit], can you take a look?`,
			expected: "Hey [~ankit], can you take a look?",
		},
		{
			name:     "mention inside a list item",
			input:    "- [~ankit] please review\n- [~jane doe] please approve",
			expected: "* [~ankit] please review\n* [~jane doe] please approve",
		},
		{
			name:     "multiple mentions",
			input:    `cc [~ankit] and [~jane]`,
			expected: "cc [~ankit] and [~jane]",
		},
		{
			name:     "more than ten mentions",
			input:    "[~user0] [~user1] [~user2] [~user3] [~user4] [~user5] [~user6] [~user7] [~user8] [~user9] [~user10] [~user11]",
			expected: "[~user0] [~user1] [~user2] [~user3] [~user4] [~user5] [~user6] [~user7] [~user8] [~user9] [~user10] [~user11]",
		},
		{
			name:     "mention in inline code",
			input:    "Use `[~ankit]` here",
			expected: "Use {{[~ankit]}} here",
		},
		{
			name:     "mention in code fence",
			input:    "```\n[~ankit]\n```",
			expected: "{code}\n[~ankit]\n{code}",
		},
		{
			name:     "non-mention brackets stay escaped",
			input:    `a [not a mention] b`,
			expected: "a \\[not a mention\\] b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToJiraMD(tc.input))
		})
	}
}
