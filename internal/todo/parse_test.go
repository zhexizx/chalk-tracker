package todo

import (
	"reflect"
	"testing"
	"time"
)

func TestParseDraft(t *testing.T) {
	now := time.Date(2026, 12, 31, 23, 30, 0, 0, time.Local)
	cases := map[string]struct {
		input string
		want  Draft
	}{
		"plain text":       {"买牛奶", Draft{Text: "买牛奶"}},
		"all tokens":       {"买牛奶 #生活 @超市 p1 tomorrow", Draft{Text: "买牛奶", Project: "生活", Labels: []string{"超市"}, Priority: 1, Due: "2027-01-01"}},
		"tokens anywhere":  {"p2 买 #生活 牛奶 @超市", Draft{Text: "买 牛奶", Project: "生活", Labels: []string{"超市"}, Priority: 2}},
		"today":            {"a today", Draft{Text: "a", Due: "2026-12-31"}},
		"explicit date":    {"a 2027-03-01", Draft{Text: "a", Due: "2027-03-01"}},
		"repeated labels":  {"a @x @y @x", Draft{Text: "a", Labels: []string{"x", "y"}}},
		"last token wins":  {"a #x #y p1 p2 today 2027-03-01", Draft{Text: "a", Project: "y", Priority: 2, Due: "2027-03-01"}},
		"bare symbols":     {"a # @ b", Draft{Text: "a # @ b"}},
		"email and url":    {"a@b.com x.com/#top", Draft{Text: "a@b.com x.com/#top"}},
		"not priority":     {"p0 p5 p10 P1", Draft{Text: "p0 p5 p10 P1"}},
		"case sensitive":   {"Today Tomorrow", Draft{Text: "Today Tomorrow"}},
		"escaped tokens":   {`\#123 \@x \p1 \today \2027-03-01`, Draft{Text: "#123 @x p1 today 2027-03-01"}},
		"other backslash":  {`C:\temp \x`, Draft{Text: `C:\temp \x`}},
		"collapses spaces": {"  a   b  ", Draft{Text: "a b"}},
		"tokens only":      {"#x p1", Draft{Project: "x", Priority: 1}},
		"empty":            {"", Draft{}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseDraft(tc.input, now)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseDraftRejectsImpossibleDate(t *testing.T) {
	if _, err := ParseDraft("a 2026-02-30", time.Now()); err == nil {
		t.Error("expected an error")
	}
}

func TestParsedDraftWithoutTextIsRejectedByStore(t *testing.T) {
	draft, err := ParseDraft("#x p1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	store, _ := newTestStore(t, "")
	if _, err := store.Add(draft); err == nil {
		t.Error("expected an error")
	}
}

func TestFormatDraftRoundTrips(t *testing.T) {
	items := map[string]Item{
		"minimal":    {Text: "买牛奶", Project: DefaultProject, Priority: DefaultPriority},
		"all fields": {Text: "买 牛奶", Project: "生活", Labels: []string{"x", "y"}, Priority: 1, Due: "2026-12-31"},
		"tokens in text": {
			Text: `修复 #123 @me p1 today tomorrow 2026-01-01 \p2 \\#x`, Project: "Work", Priority: 2,
		},
		"ordinary symbols": {Text: `# @ a@b.com C:\temp`, Project: DefaultProject, Priority: 3},
	}
	for name, item := range items {
		t.Run(name, func(t *testing.T) {
			got, err := ParseDraft(FormatDraft(item), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			want := Draft{Text: item.Text, Project: item.Project, Labels: item.Labels, Priority: item.Priority, Due: item.Due}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}
