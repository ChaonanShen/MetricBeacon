package mock

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
	"mini-torchbearing.local/services/ai-core/internal/ports/agent"
)

type Planner struct{}

var _ agent.IntentPlanner = Planner{}

var rangeIntentPattern = regexp.MustCompile(`(?:最近|过去|近)\s*([0-9零〇一二两三四五六七八九十百]+)\s*(秒|分钟|小时|sec|min|s|m|h)`)
var stepIntentPattern = regexp.MustCompile(`(?:每(?:个|隔)?|间隔)\s*([0-9零〇一二两三四五六七八九十百]+)\s*(秒|分钟|sec|min|s|m)`)
var pointStepIntentPattern = regexp.MustCompile(`([0-9零〇一二两三四五六七八九十百]+)\s*(秒|分钟|sec|min|s|m)\s*(?:一个)?数据(?:点)?`)
var englishRangePattern = regexp.MustCompile(`(?:last|past)\s+(\d+)\s*(seconds?|minutes?|hours?)`)
var englishStepPattern = regexp.MustCompile(`(?:every|each)\s+(\d+)\s*(seconds?|minutes?)`)

func (Planner) Plan(_ context.Context, _ requestcontext.Context, request agent.IntentPlanRequest) (agent.IntentPlan, error) {
	message := strings.ToLower(request.Message)
	views := parseViews(message)
	if len(views) == 0 {
		for index := len(request.PreviousIntents) - 1; index >= 0; index-- {
			views = append([]string(nil), request.PreviousIntents[index].Views...)
			if len(views) > 0 {
				break
			}
		}
	}
	if len(views) == 0 {
		return agent.IntentPlan{Status: agent.IntentUnsupported, Views: []string{}}, nil
	}
	plan := agent.IntentPlan{Status: agent.IntentPlanned, Views: views}
	if duration, ok := parseDuration(message, rangeIntentPattern, englishRangePattern); ok {
		plan.RangeDuration = &duration
	}
	if duration, ok := parseDuration(message, stepIntentPattern, pointStepIntentPattern, englishStepPattern); ok {
		seconds := int(duration / time.Second)
		plan.StepSeconds = &seconds
	}
	return plan, nil
}

func parseViews(message string) []string {
	views := make([]string, 0, 3)
	if strings.Contains(message, "cpu") || strings.Contains(message, "处理器") {
		views = append(views, "cpu")
	}
	if strings.Contains(message, "memory") || strings.Contains(message, "内存") {
		views = append(views, "memory")
	}
	if strings.Contains(message, "load") || strings.Contains(message, "负载") || strings.Contains(message, "负荷") {
		views = append(views, "load")
	}
	if len(views) == 0 && (strings.Contains(message, "node_exporter") || strings.Contains(message, "node exporter") || strings.Contains(message, "概览")) {
		return []string{"cpu", "memory", "load"}
	}
	return views
}

func parseDuration(message string, patterns ...*regexp.Regexp) (time.Duration, bool) {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(message)
		if len(match) != 3 {
			continue
		}
		value, ok := positiveInteger(match[1])
		if !ok {
			return 0, false
		}
		unit := strings.ToLower(match[2])
		multiplier := time.Second
		if unit == "分钟" || unit == "m" || unit == "min" || strings.HasPrefix(unit, "minute") {
			multiplier = time.Minute
		} else if unit == "小时" || unit == "h" || strings.HasPrefix(unit, "hour") {
			multiplier = time.Hour
		}
		return time.Duration(value) * multiplier, true
	}
	return 0, false
}

func positiveInteger(raw string) (int, bool) {
	if value, err := strconv.Atoi(raw); err == nil && value > 0 {
		return value, true
	}
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, current := 0, 0
	for _, char := range raw {
		if digit, exists := digits[char]; exists {
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
