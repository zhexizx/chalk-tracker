package todo

import (
	"reflect"
	"testing"
	"time"
)

var viewNow = time.Date(2026, 12, 28, 23, 30, 0, 0, time.Local)

func ids(items []Item) []int {
	result := []int{}
	for _, item := range items {
		result = append(result, item.ID)
	}
	return result
}

func assertIDs(t *testing.T, got []Item, want ...int) {
	t.Helper()
	if want == nil {
		want = []int{}
	}
	if !reflect.DeepEqual(ids(got), want) {
		t.Errorf("got IDs %v, want %v", ids(got), want)
	}
}

func TestToday(t *testing.T) {
	items := []Item{
		{ID: 1, Due: "2026-12-28", Priority: 4},
		{ID: 2, Due: "2026-12-28", Priority: 1},
		{ID: 3, Due: "2026-12-20", Priority: 4},
		{ID: 4, Due: "2026-12-29", Priority: 1},
		{ID: 5, Priority: 1},
		{ID: 6, Due: "2026-12-20", Priority: 1, Done: true},
	}
	assertIDs(t, Today(items, viewNow), 3, 2, 1)
}

func TestUpcomingExcludesTodayAndCoversSevenDaysAcrossYearEnd(t *testing.T) {
	items := []Item{
		{ID: 1, Due: "2026-12-28"},
		{ID: 2, Due: "2026-12-29"},
		{ID: 3, Due: "2027-01-04"},
		{ID: 4, Due: "2027-01-05"},
		{ID: 5},
		{ID: 6, Due: "2026-12-30", Done: true},
	}
	assertIDs(t, Upcoming(items, viewNow), 2, 3)
}

func TestSelect(t *testing.T) {
	items := []Item{
		{ID: 1, Project: "Work", Priority: 1, Labels: []string{"x"}},
		{ID: 2, Project: "Work", Priority: 4},
		{ID: 3, Project: "Home", Priority: 1, Labels: []string{"x", "y"}},
	}
	assertIDs(t, Select(items, Filter{}), 1, 3, 2)
	assertIDs(t, Select(items, Filter{Project: "Work"}), 1, 2)
	assertIDs(t, Select(items, Filter{Label: "y"}), 3)
	assertIDs(t, Select(items, Filter{Priority: 1}), 1, 3)
	assertIDs(t, Select(items, Filter{Project: "Work", Label: "x", Priority: 4}))
}

func TestDisplayOrder(t *testing.T) {
	items := []Item{
		{ID: 1, Priority: 1, Done: true, Due: "2026-01-01"},
		{ID: 2, Priority: 1},
		{ID: 3, Priority: 4, Due: "2026-12-31"},
		{ID: 4, Priority: 1, Due: "2026-12-31"},
		{ID: 5, Priority: 4, Due: "2026-12-01"},
		{ID: 6, Priority: 1},
	}
	assertIDs(t, Select(items, Filter{}), 5, 4, 3, 2, 6, 1)
}

func TestProjects(t *testing.T) {
	items := []Item{
		{Project: "Work"}, {Project: DefaultProject}, {Project: "Home"}, {Project: "Work"},
	}
	want := []string{"Home", "Work"}
	if got := Projects(items); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
