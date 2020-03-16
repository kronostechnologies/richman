package action

import (
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/pelletier/go-toml"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

type ChartUpdate struct {
	Filename     string
	AppFilters   []string
	ChartFilters []string
	Apply        bool
	RepoUpdate   bool
}

func (c *ChartUpdate) Run() error {
	data, err := toml.LoadFile(c.Filename)
	if err != nil {
		return err
	}
	apps := data.Get("apps").(*toml.Tree)

	if c.RepoUpdate {
		err := helmUpdateRepos()
		if err != nil {
			return err
		}

	}

	if err = updateCommand(apps, c); err != nil {
		return err
	}

	if c.Apply {
		err = writeTomlFile(c.Filename, data)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Use '--apply' to write changes")
	}

	return nil
}

func updateCommand(apps *toml.Tree, o *ChartUpdate) error {
	getHelmChart := createHelmChartRepo()

	noFilter := len(o.AppFilters) == 0 && len(o.ChartFilters) == 0
	chartMatchers := CreateChartMatchers(o.ChartFilters)
	appMatchers := CreateAppMatchers(o.AppFilters)

	fmt.Printf("%-20s %-30s %7s    %7s\n", "APP", "CHART", "LOCAL", "REMOTE")

	for _, name := range apps.Keys() {
		app := apps.Get(name).(*toml.Tree)
		chartName := app.Get("chart").(string)
		localVersion := app.Get("version").(string)

		if noFilter || findMatch(name, appMatchers) || findMatch(chartName, chartMatchers) {
			chart := getHelmChart(chartName)
			if chart == nil {
				return fmt.Errorf("%s has no repository", chartName)
			}

			repoVersion := chart.version

			if hasUpdate(localVersion, repoVersion) {
				fmt.Printf("%-20s %-30s %7s -> %7s\n", name, chartName, localVersion, repoVersion)
				app.Set("version", repoVersion)
			}
		}

	}

	return nil
}

func createHelmChartRepo() func(chart string) *Chart {
	var cache = make(map[string]*Repository)

	return func(chart string) *Chart {
		repo := getRepoFromKeyword(chart)

		if cache[repo] == nil {
			cache[repo], _ = helmSearchRepo(repo)
		}

		return cache[repo].charts[chart]
	}
}

func helmSearchRepo(name string) (*Repository, error) {
	out, err := exec.Command("helm", "search", "repo", name, "-o", "json").Output()
	if err != nil {
		return nil, err
	}

	var charts = make(map[string]*Chart)

	var result []map[string]string
	err = json.Unmarshal(out, &result)
	if err != nil {
		return nil, err
	}

	for _, i := range result {
		charts[i["name"]] = &Chart{
			name:    i["name"],
			version: i["version"],
		}
	}

	return &Repository{name, charts}, nil
}

func helmUpdateRepos() error {
	fmt.Println("Updating chart repositories upstream...")
	_, err := exec.Command("helm", "repo", "update").Output()

	return err
}

func getRepoFromKeyword(keyword string) string {
	slashSplitter := func(s rune) bool {
		return s == '/'
	}

	splitted := strings.FieldsFunc(keyword, slashSplitter)

	if len(splitted) > 0 {
		return splitted[0]
	}

	return ""
}

func writeTomlFile(filename string, toml *toml.Tree) error {
	_, _ = fmt.Fprintf(os.Stderr, "Writing changes to %s...\n", filename)
	return ioutil.WriteFile(filename, []byte(toml.String()), 0644)
}

func hasUpdate(current string, upstream string) bool {
	c, _ := semver.NewConstraint("> " + current)
	v, _ := semver.NewVersion(upstream)

	return c.Check(v)
}