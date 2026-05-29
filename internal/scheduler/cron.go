// Package scheduler provides a lightweight cron expression parser and
// next-run-time calculator. It supports the standard 5-field cron format:
//
//	┌───────────── minute (0–59)
//	│ ┌───────────── hour (0–23)
//	│ │ ┌───────────── day of month (1–31)
//	│ │ │ ┌───────────── month (1–12)
//	│ │ │ │ ┌───────────── day of week (0–6, Sunday=0)
//	│ │ │ │ │
//	* * * * *
//
// Supported syntax: *, */n, n, n-m, n,m,... and combinations.
// Named months (JAN–DEC) and weekdays (SUN–SAT) are also supported.
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSchedule holds the parsed bit-sets for each cron field.
type CronSchedule struct {
	Minutes  [60]bool
	Hours    [24]bool
	Days     [32]bool // 1-indexed; index 0 unused
	Months   [13]bool // 1-indexed; index 0 unused
	Weekdays [7]bool  // 0=Sunday
}

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var weekdayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// ParseCron parses a 5-field cron expression and returns a CronSchedule.
func ParseCron(expr string) (*CronSchedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron: expected 5 fields, got %d in %q", len(fields), expr)
	}

	cs := &CronSchedule{}
	var err error

	if err = parseField(fields[0], 0, 59, cs.Minutes[:], nil); err != nil {
		return nil, fmt.Errorf("cron minute: %w", err)
	}
	if err = parseField(fields[1], 0, 23, cs.Hours[:], nil); err != nil {
		return nil, fmt.Errorf("cron hour: %w", err)
	}
	if err = parseField(fields[2], 1, 31, cs.Days[1:], nil); err != nil {
		return nil, fmt.Errorf("cron day-of-month: %w", err)
	}
	if err = parseField(fields[3], 1, 12, cs.Months[1:], monthNames); err != nil {
		return nil, fmt.Errorf("cron month: %w", err)
	}
	if err = parseField(fields[4], 0, 6, cs.Weekdays[:], weekdayNames); err != nil {
		return nil, fmt.Errorf("cron day-of-week: %w", err)
	}

	return cs, nil
}

// Next returns the next time after `from` that matches the cron schedule,
// evaluated in the given location. Returns zero time if no match within 4 years.
func (cs *CronSchedule) Next(from time.Time, loc *time.Location) time.Time {
	// Advance by 1 minute to avoid returning `from` itself.
	t := from.In(loc).Truncate(time.Minute).Add(time.Minute)

	// Search up to 4 years ahead to handle leap-year edge cases.
	limit := t.Add(4 * 365 * 24 * time.Hour)

	for t.Before(limit) {
		// Month check
		if !cs.Months[int(t.Month())] {
			// Advance to the first day of the next matching month.
			t = advanceToNextMonth(t, cs)
			continue
		}

		// Day-of-month AND day-of-week check.
		// Standard cron: if both are restricted (not *), either can match.
		// We use the simpler "both must match" interpretation here.
		if !cs.Days[t.Day()] || !cs.Weekdays[int(t.Weekday())] {
			t = t.AddDate(0, 0, 1)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
			continue
		}

		// Hour check
		if !cs.Hours[t.Hour()] {
			t = t.Add(time.Hour)
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)
			continue
		}

		// Minute check
		if !cs.Minutes[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	return time.Time{} // no match found
}

// ─────────────────────────────────────────────────────────────────────────────
// Convenience constructors for common schedule types
// ─────────────────────────────────────────────────────────────────────────────

// DailyCron returns a cron expression for "every day at HH:MM".
func DailyCron(hour, minute int) string {
	return fmt.Sprintf("%d %d * * *", minute, hour)
}

// WeeklyCron returns a cron expression for "every weekday at HH:MM".
// weekday: 0=Sunday, 1=Monday, ..., 6=Saturday.
func WeeklyCron(weekday, hour, minute int) string {
	return fmt.Sprintf("%d %d * * %d", minute, hour, weekday)
}

// MonthlyCron returns a cron expression for "every month on day D at HH:MM".
func MonthlyCron(day, hour, minute int) string {
	return fmt.Sprintf("%d %d %d * *", minute, hour, day)
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// parseField parses a single cron field into a boolean slice.
// min/max define the valid range; names maps string aliases to integers.
// The slice must have length (max - min + 1).
func parseField(field string, min, max int, bits []bool, names map[string]int) error {
	parts := strings.Split(field, ",")
	for _, part := range parts {
		if err := parsePart(part, min, max, bits, names); err != nil {
			return err
		}
	}
	return nil
}

func parsePart(part string, min, max int, bits []bool, names map[string]int) error {
	// Step: */n or n-m/n
	step := 1
	if idx := strings.Index(part, "/"); idx >= 0 {
		var err error
		step, err = strconv.Atoi(part[idx+1:])
		if err != nil || step < 1 {
			return fmt.Errorf("invalid step %q", part[idx+1:])
		}
		part = part[:idx]
	}

	// Wildcard
	if part == "*" {
		for i := min; i <= max; i += step {
			bits[i-min] = true
		}
		return nil
	}

	// Range: n-m
	if idx := strings.Index(part, "-"); idx >= 0 {
		lo, err1 := parseValue(part[:idx], names)
		hi, err2 := parseValue(part[idx+1:], names)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid range %q", part)
		}
		if lo < min || hi > max || lo > hi {
			return fmt.Errorf("range %d-%d out of bounds [%d,%d]", lo, hi, min, max)
		}
		for i := lo; i <= hi; i += step {
			bits[i-min] = true
		}
		return nil
	}

	// Single value
	v, err := parseValue(part, names)
	if err != nil {
		return err
	}
	if v < min || v > max {
		return fmt.Errorf("value %d out of bounds [%d,%d]", v, min, max)
	}
	bits[v-min] = true
	return nil
}

func parseValue(s string, names map[string]int) (int, error) {
	if names != nil {
		if v, ok := names[strings.ToLower(s)]; ok {
			return v, nil
		}
	}
	return strconv.Atoi(s)
}

func advanceToNextMonth(t time.Time, cs *CronSchedule) time.Time {
	// Move to the first day of the next month and keep searching.
	year, month := t.Year(), t.Month()
	for {
		month++
		if month > 12 {
			month = 1
			year++
		}
		if cs.Months[int(month)] {
			return time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
		}
		// Safety: don't loop forever.
		if year > t.Year()+5 {
			return t.Add(4 * 365 * 24 * time.Hour) // trigger limit
		}
	}
}
