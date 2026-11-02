package todo

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	priorityPattern = regexp.MustCompile(`^p[1-4]$`)
	datePattern     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// ParseDraft reads the quick-add syntax. Whitespace-separated tokens may appear
// anywhere in the input:
//
//	#project  @label  p1..p4  today  tomorrow  YYYY-MM-DD
//
// All other words form the text. Tokens are case-sensitive, a label may repeat,
// and for the other fields the last token wins. A leading backslash keeps a
// token as literal text, as in `\p1` or `\#123`.
func ParseDraft(input string, now time.Time) (Draft, error) {
	var draft Draft
	var words []string
	for field := range strings.FieldsSeq(input) {
		if isEscaped(field) {
			words = append(words, field[1:])
			continue
		}
		if !isToken(field) {
			words = append(words, field)
			continue
		}
		switch {
		case field[0] == '#':
			draft.Project = field[1:]
		case field[0] == '@':
			if label := field[1:]; !slices.Contains(draft.Labels, label) {
				draft.Labels = append(draft.Labels, label)
			}
		case priorityPattern.MatchString(field):
			draft.Priority = int(field[1] - '0')
		case field == "today":
			draft.Due = now.Format(DueLayout)
		case field == "tomorrow":
			draft.Due = now.AddDate(0, 0, 1).Format(DueLayout)
		default:
			if _, err := time.Parse(DueLayout, field); err != nil {
				return Draft{}, fmt.Errorf("invalid due date %q", field)
			}
			draft.Due = field
		}
	}
	draft.Text = strings.Join(words, " ")
	return draft, nil
}

// FormatDraft renders an item as quick-add input, for editing. Parsing the result
// gives back the item's text, project, labels, priority and due date.
func FormatDraft(item Item) string {
	var parts []string
	for word := range strings.FieldsSeq(item.Text) {
		if isToken(strings.TrimLeft(word, `\`)) {
			word = `\` + word
		}
		parts = append(parts, word)
	}
	parts = append(parts, "#"+item.Project)
	for _, label := range item.Labels {
		parts = append(parts, "@"+label)
	}
	parts = append(parts, fmt.Sprintf("p%d", item.Priority))
	if item.Due != "" {
		parts = append(parts, item.Due)
	}
	return strings.Join(parts, " ")
}

func isEscaped(field string) bool {
	return strings.HasPrefix(field, `\`) && isToken(strings.TrimLeft(field, `\`))
}

func isToken(field string) bool {
	if len(field) > 1 && (field[0] == '#' || field[0] == '@') {
		return true
	}
	return field == "today" || field == "tomorrow" ||
		priorityPattern.MatchString(field) || datePattern.MatchString(field)
}
