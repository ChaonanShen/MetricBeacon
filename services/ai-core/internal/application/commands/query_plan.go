package commands

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	"mini-torchbearing.local/services/ai-core/internal/domain/task"
)

const (
	defaultRange      = 30 * time.Minute
	minRange          = 30 * time.Second
	maxRange          = 6 * time.Hour
	maxAutoPoints     = 300
	maxExplicitPoints = 1000
)

var allowedSteps = []int{5, 10, 15, 30, 60, 120, 300}
var rangePattern = regexp.MustCompile(`(?:最近|过去|近)\s*([0-9零〇一二两三四五六七八九十百]+)\s*(秒|分钟|小时|s|m|h)`)
var stepPattern = regexp.MustCompile(`每(?:个)?\s*([0-9零〇一二两三四五六七八九十百]+)\s*(秒|分钟|s|m)`)

type RequestedTimeRange struct {
	Absolute         *common.AbsoluteTimeRange
	RelativeDuration time.Duration
}

func ResolveQueryPlan(message string, requested RequestedTimeRange, requestedStep *int, now time.Time) (common.AbsoluteTimeRange, task.QueryPlan, error) {
	now = now.UTC()
	duration := requested.RelativeDuration
	if duration == 0 {
		duration = defaultRange
	}
	if requested.Absolute == nil {
		if parsed, ok := parseMessageDuration(message, rangePattern); ok {
			duration = parsed
		}
	}
	var timeRange common.AbsoluteTimeRange
	var err error
	if requested.Absolute != nil {
		timeRange = *requested.Absolute
		duration = timeRange.To.Sub(timeRange.From)
	} else {
		timeRange, err = common.NewAbsoluteTimeRange(now.Add(-duration), now)
		if err != nil {
			return common.AbsoluteTimeRange{}, task.QueryPlan{}, err
		}
	}
	if duration < minRange || duration > maxRange {
		return common.AbsoluteTimeRange{}, task.QueryPlan{}, common.NewError(common.InvalidArgument, "query range must be between 30 seconds and 6 hours", false)
	}

	explicitStep := requestedStep
	if parsed, ok := parseMessageDuration(message, stepPattern); ok {
		seconds := int(parsed / time.Second)
		explicitStep = &seconds
	}
	step := autoStep(duration)
	if explicitStep != nil {
		step = *explicitStep
		if !containsStep(step) {
			return common.AbsoluteTimeRange{}, task.QueryPlan{}, common.NewError(common.InvalidArgument, "query step must be one of 5, 10, 15, 30, 60, 120 or 300 seconds", false)
		}
		if theoreticalPoints(duration, step) > maxExplicitPoints {
			return common.AbsoluteTimeRange{}, task.QueryPlan{}, common.NewError(common.InvalidArgument, "query resolution exceeds the 1000 point budget", false)
		}
	}
	window := 300
	if duration <= 10*time.Minute {
		window = 30
	} else if duration <= time.Hour {
		window = 60
	}
	plan, err := task.NewQueryPlan(step, window)
	return timeRange, plan, err
}

func autoStep(duration time.Duration) int {
	for _, step := range allowedSteps {
		if theoreticalPoints(duration, step) <= maxAutoPoints {
			return step
		}
	}
	return allowedSteps[len(allowedSteps)-1]
}

func theoreticalPoints(duration time.Duration, step int) int {
	return int(duration/(time.Duration(step)*time.Second)) + 1
}

func containsStep(value int) bool {
	for _, allowed := range allowedSteps {
		if value == allowed {
			return true
		}
	}
	return false
}

func parseMessageDuration(message string, pattern *regexp.Regexp) (time.Duration, bool) {
	match := pattern.FindStringSubmatch(strings.ToLower(message))
	if len(match) != 3 {
		return 0, false
	}
	value, ok := parsePositiveInteger(match[1])
	if !ok {
		return 0, false
	}
	unit := map[string]time.Duration{"s": time.Second, "秒": time.Second, "m": time.Minute, "分钟": time.Minute, "h": time.Hour, "小时": time.Hour}[match[2]]
	if unit == 0 {
		return 0, false
	}
	return time.Duration(value) * unit, true
}

func parsePositiveInteger(raw string) (int, bool) {
	if value, err := strconv.Atoi(raw); err == nil && value > 0 {
		return value, true
	}
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, current := 0, 0
	for _, char := range raw {
		if unicode.IsSpace(char) {
			continue
		}
		if digit, ok := digits[char]; ok {
			current = digit
			continue
		}
		switch char {
		case '十':
			if current == 0 {
				current = 1
			}
			total += current * 10
			current = 0
		case '百':
			if current == 0 {
				current = 1
			}
			total += current * 100
			current = 0
		default:
			return 0, false
		}
	}
	total += current
	return total, total > 0
}
