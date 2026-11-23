package view

import (
	"time"

	"github.com/duke-git/lancet/v2/slice"
	conf "github.com/muety/wakapi/config"
	"github.com/muety/wakapi/models"
)

type SummaryViewModel struct {
	SharedLoggedInViewModel
	*models.Summary
	*models.SummaryParams
	AvailableFilters    AvailableFilters
	AvatarURL           string
	EditorColors        map[string]string
	LanguageColors      map[string]string
	OSColors            map[string]string
	Timeline            []*TimelineViewModel
	HourlyBreakdown     []*HourlyBreakdownViewModel
	HourlyBreakdownFrom time.Time
	RawQuery            string
	UserFirstData       time.Time
	DataRetentionMonths int
}

type AvailableFilters struct {
	ProjectNames  []string
	LanguageNames []string
	MachineNames  []string
	LabelNames    []string
	CategoryNames []string
}

type TimelineViewModel struct {
	Date     time.Time       `json:"date"`
	Projects []*TimelineItem `json:"projects"`
}

type TimelineItem struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
}

type HourlyBreakdownsViewModel []*HourlyBreakdownViewModel

type HourlyBreakdownViewModel struct {
	Items   []*HourlyBreakdownItem `json:"items"`
	Project string                 `json:"project"`
}

type HourlyBreakdownItems []*HourlyBreakdownItem

type HourlyBreakdownItem struct {
	FromTime time.Time     `json:"from_time"`
	Duration time.Duration `json:"duration"`
	Entity   string        `json:"entity"`
	Project  string        `json:"-"`
	// Timeout is the user's heartbeat timeout. Items of the same project that are less than this apart are merged into one block.
	Timeout time.Duration `json:"-"`
}

func (h *HourlyBreakdownItem) ToTime() time.Time {
	return h.FromTime.Add(h.Duration)
}

func NewTimelineViewModel(summaries []*models.Summary) []*TimelineViewModel {
	vm := make([]*TimelineViewModel, 0)
	for _, summary := range summaries {
		vm = append(vm, &TimelineViewModel{
			Date: summary.FromTime.T(),
			Projects: slice.Map(summary.Projects, func(_ int, curProject *models.SummaryItem) *TimelineItem {
				return &TimelineItem{
					Name:     curProject.Key,
					Duration: curProject.Total,
				}
			}),
		})
	}
	return vm
}

func NewHourlyBreakdownItems(durations models.Durations, resolve models.AliasResolver) HourlyBreakdownItems {
	hourlyBreakdowns := slice.
		Map(durations, func(_ int, duration *models.Duration) *HourlyBreakdownItem {
			return &HourlyBreakdownItem{
				FromTime: duration.Time.T(),
				Duration: duration.Duration,
				Entity:   duration.Entity,
				Project:  duration.Project,
				Timeout:  duration.Timeout,
			}
		})

	hourlyBreakdowns = slice.Map(hourlyBreakdowns, func(_ int, item *HourlyBreakdownItem) *HourlyBreakdownItem {
		item.Project = resolve(models.SummaryProject, item.Project)
		item.Entity = resolve(models.SummaryEntity, item.Entity)
		return item
	})

	return hourlyBreakdowns
}

func NewHourlyBreakdownViewModel(items HourlyBreakdownItems) HourlyBreakdownsViewModel {
	hourlyBreakdownMap := slice.GroupWith(items, func(item *HourlyBreakdownItem) string { return item.Project })

	hourlyBreakdown := make([]*HourlyBreakdownViewModel, 0)
	for project, projectItems := range hourlyBreakdownMap {
		slice.SortBy(projectItems, func(i, j *HourlyBreakdownItem) bool {
			return i.FromTime.Before(j.FromTime)
		})
		hourlyBreakdown = append(hourlyBreakdown, &HourlyBreakdownViewModel{
			Items:   coalesceHourlyBreakdownItems(projectItems),
			Project: project,
		})
	}

	return hourlyBreakdown
}

// NumItems returns the number of bars the chart would draw.
func (vm HourlyBreakdownsViewModel) NumItems() int {
	var n int
	for _, project := range vm {
		n += len(project.Items)
	}
	return n
}

// coalesceHourlyBreakdownItems merges items that are at most their timeout apart into one block (see https://github.com/muety/wakapi/issues/952).
// Durations are split whenever the entity, branch, language, etc. changes, so a normal day can have hundreds of tiny segments of the same project.
// Items must be of one project and sorted by start time. A merged block keeps the entity of its first item.
func coalesceHourlyBreakdownItems(items HourlyBreakdownItems) HourlyBreakdownItems {
	if len(items) == 0 {
		return items
	}

	merged := make(HourlyBreakdownItems, 0, len(items))
	current := *items[0]
	for _, next := range items[1:] {
		if next.FromTime.Sub(current.ToTime()) <= current.Timeout {
			if end := next.ToTime(); end.After(current.ToTime()) {
				current.Duration = end.Sub(current.FromTime)
			}
			continue
		}
		block := current
		merged = append(merged, &block)
		current = *next
	}
	merged = append(merged, &current)

	return merged
}

func (s SummaryViewModel) UserDataExpiring() bool {
	cfg := conf.Get()
	return cfg.Subscriptions.Enabled &&
		cfg.App.DataRetentionMonths > 0 &&
		!s.UserFirstData.IsZero() &&
		!s.SharedLoggedInViewModel.User.HasActiveSubscription() &&
		time.Now().AddDate(0, -cfg.App.DataRetentionMonths, 0).After(s.UserFirstData)
}

func (s *SummaryViewModel) WithSuccess(m string) *SummaryViewModel {
	s.SetSuccess(m)
	return s
}

func (s *SummaryViewModel) WithError(m string) *SummaryViewModel {
	s.SetError(m)
	return s
}
