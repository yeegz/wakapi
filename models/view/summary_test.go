package view

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var t0 = time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)

func item(project, entity string, fromMinute, minutes int) *HourlyBreakdownItem {
	return &HourlyBreakdownItem{
		FromTime: t0.Add(time.Duration(fromMinute) * time.Minute),
		Duration: time.Duration(minutes) * time.Minute,
		Entity:   entity,
		Project:  project,
		Timeout:  10 * time.Minute,
	}
}

func TestCoalesceHourlyBreakdownItems_MergesSegmentsWithinTimeout(t *testing.T) {
	items := HourlyBreakdownItems{
		item("wakapi", "a.go", 0, 2),
		item("wakapi", "b.go", 2, 3),  // touching
		item("wakapi", "c.go", 12, 1), // 7 min gap
	}

	result := coalesceHourlyBreakdownItems(items)

	assert.Len(t, result, 1)
	assert.Equal(t, t0, result[0].FromTime)
	assert.Equal(t, 13*time.Minute, result[0].Duration)
	assert.Equal(t, "a.go", result[0].Entity)
}

func TestCoalesceHourlyBreakdownItems_KeepsSegmentsAfterLongerGaps(t *testing.T) {
	items := HourlyBreakdownItems{
		item("wakapi", "a.go", 0, 5),
		item("wakapi", "b.go", 16, 5), // 11 min gap
		item("wakapi", "c.go", 30, 5), // 9 min gap
	}

	result := coalesceHourlyBreakdownItems(items)

	assert.Len(t, result, 2)
	assert.Equal(t, t0, result[0].FromTime)
	assert.Equal(t, 5*time.Minute, result[0].Duration)
	assert.Equal(t, t0.Add(16*time.Minute), result[1].FromTime)
	assert.Equal(t, 19*time.Minute, result[1].Duration)
	assert.Equal(t, "b.go", result[1].Entity)
}

func TestCoalesceHourlyBreakdownItems_ContainedSegmentDoesNotShortenBlock(t *testing.T) {
	items := HourlyBreakdownItems{
		item("wakapi", "a.go", 0, 30),
		item("wakapi", "b.go", 5, 1),
	}

	result := coalesceHourlyBreakdownItems(items)

	assert.Len(t, result, 1)
	assert.Equal(t, 30*time.Minute, result[0].Duration)
}

func TestCoalesceHourlyBreakdownItems_DoesNotChangeInput(t *testing.T) {
	items := HourlyBreakdownItems{
		item("wakapi", "a.go", 0, 2),
		item("wakapi", "b.go", 3, 2),
	}

	coalesceHourlyBreakdownItems(items)

	assert.Equal(t, 2*time.Minute, items[0].Duration)
	assert.Equal(t, 2*time.Minute, items[1].Duration)
}

func TestCoalesceHourlyBreakdownItems_Empty(t *testing.T) {
	assert.Empty(t, coalesceHourlyBreakdownItems(HourlyBreakdownItems{}))
}

func TestNewHourlyBreakdownViewModel_CoalescesPerProject(t *testing.T) {
	items := HourlyBreakdownItems{
		item("wakapi", "c.go", 4, 1), // unsorted on purpose
		item("other", "x.py", 1, 1),
		item("wakapi", "a.go", 0, 2),
		item("wakapi", "b.go", 2, 1),
		item("wakapi", "d.go", 60, 1),
	}

	vm := NewHourlyBreakdownViewModel(items)

	byProject := map[string]HourlyBreakdownItems{}
	for _, p := range vm {
		byProject[p.Project] = p.Items
	}
	assert.Len(t, vm, 2)
	assert.Len(t, byProject["wakapi"], 2)
	assert.Equal(t, t0, byProject["wakapi"][0].FromTime)
	assert.Equal(t, 5*time.Minute, byProject["wakapi"][0].Duration)
	assert.Equal(t, t0.Add(60*time.Minute), byProject["wakapi"][1].FromTime)
	assert.Len(t, byProject["other"], 1)
	assert.Equal(t, 3, vm.NumItems())
}

func TestNewHourlyBreakdownViewModel_ManySegmentsStayUnderLimit(t *testing.T) {
	// https://github.com/muety/wakapi/issues/952: 400 short segments of continuous work used to hide the chart
	items := make(HourlyBreakdownItems, 0, 400)
	for i := 0; i < 400; i++ {
		items = append(items, &HourlyBreakdownItem{
			FromTime: t0.Add(time.Duration(i) * 30 * time.Second),
			Duration: 20 * time.Second,
			Entity:   "file.go",
			Project:  "wakapi",
			Timeout:  10 * time.Minute,
		})
	}

	vm := NewHourlyBreakdownViewModel(items)

	assert.Equal(t, 1, vm.NumItems())
}
