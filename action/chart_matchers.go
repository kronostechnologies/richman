package action

import "strings"

type Matcher interface {
	Matches(s string) bool
}

type RepoMatcher struct {
	repo string
}

type ChartMatcher struct {
	chart string
}

type AppMatcher struct {
	app string
}

func (r RepoMatcher) Matches(s string) bool {
	return strings.HasPrefix(s, r.repo + "/")
}

func (r ChartMatcher) Matches(s string) bool {
	return s == r.chart
}

func (r AppMatcher) Matches(s string) bool {
	return s == r.app
}

func CreateChartMatchers(s []string) []Matcher {
	var matchers []Matcher

	for _, m := range s {
		var matcher Matcher

		fieldFunc := func(r rune) bool {
			return r == '/'
		}
		fields := strings.FieldsFunc(m, fieldFunc)
		if len(fields) == 1 {
			matcher = RepoMatcher{fields[0]}
		} else {
			matcher = ChartMatcher{m}
		}

		matchers = append(matchers, matcher)
	}
	return matchers
}

func CreateAppMatchers(s []string) []Matcher {
	var matchers []Matcher

	for _, m := range s {
		matchers = append(matchers, AppMatcher{m})
	}

	return matchers
}

func findMatch(name string, chartFilters []Matcher) bool {
	for _, matcher := range chartFilters {
		if matcher.Matches(name) {
			return true
		}
	}
	return false
}