package todo

import (
	"cmp"
	"slices"
	"time"
)

const upcomingDays = 7

// Filter selects items by the fields that are set; zero values match everything.
type Filter struct {
	Project  string
	Label    string
	Priority int
}

// Select returns the matching items, including completed ones, in display order.
func Select(items []Item, filter Filter) []Item {
	return sorted(items, func(item Item) bool {
		return (filter.Project == "" || item.Project == filter.Project) &&
			(filter.Label == "" || slices.Contains(item.Labels, filter.Label)) &&
			(filter.Priority == 0 || item.Priority == filter.Priority)
	})
}

// Today returns unfinished items that are due today or overdue, in display order.
func Today(items []Item, now time.Time) []Item {
	today := now.Format(DueLayout)
	return sorted(items, func(item Item) bool {
		return !item.Done && item.Due != "" && item.Due <= today
	})
}

// Upcoming returns unfinished items due in the next seven days, excluding
// today, in display order.
func Upcoming(items []Item, now time.Time) []Item {
	today := now.Format(DueLayout)
	last := now.AddDate(0, 0, upcomingDays).Format(DueLayout)
	return sorted(items, func(item Item) bool {
		return !item.Done && item.Due > today && item.Due <= last
	})
}

// Projects returns the sorted names of all projects other than DefaultProject,
// which always exists and is shown separately as the Inbox.
func Projects(items []Item) []string {
	var names []string
	for _, item := range items {
		if item.Project != DefaultProject {
			names = append(names, item.Project)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// sorted returns the matching items in display order: unfinished first, then by
// due date with undated items last, then by priority, then by ID.
func sorted(items []Item, match func(Item) bool) []Item {
	var result []Item
	for _, item := range items {
		if match(item) {
			result = append(result, item)
		}
	}
	slices.SortFunc(result, func(a, b Item) int {
		if a.Done != b.Done {
			if a.Done {
				return 1
			}
			return -1
		}
		if a.Due != b.Due {
			if a.Due == "" {
				return 1
			}
			if b.Due == "" {
				return -1
			}
			return cmp.Compare(a.Due, b.Due)
		}
		return cmp.Or(cmp.Compare(a.Priority, b.Priority), cmp.Compare(a.ID, b.ID))
	})
	return result
}
