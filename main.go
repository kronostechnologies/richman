package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/pelletier/go-toml"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprint(os.Stderr, "Usage: richman [command]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "%-15s %-15s %s\n", "Command", "Arguments", "Description")
		_, _ = fmt.Fprintf(os.Stderr, "%-15s %-15s %s\n", "update", "FILE [APP]", "Update helm charts in .toml file")
	}
	flag.Parse()

	cmd := flag.Arg(0)
	filename := flag.Arg(1)
	match := flag.Arg(2)

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	data, err := toml.LoadFile(filename)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(-1)
	}

	if cmd == "update" && flag.NArg() <= 3 {
		apps := data.Get("apps").(*toml.Tree)
		updateCommand(apps, match)
		err = writeTomlFile(filename, data)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err.Error())
		}
	} else {
		flag.Usage()
		os.Exit(1)
	}

}

func updateCommand(apps *toml.Tree, match string) error {
	err := helmUpdateRepos()
	if err != nil {
		return err
	}

	getHelmChart := createHelmChartRepo()

	fmt.Printf("%-20s %-30s %7s    %7s\n", "APP", "CHART", "LOCAL", "REMOTE")

	for _, name := range apps.Keys() {
		if match == "" || match == name {
			app := apps.Get(name).(*toml.Tree)
			chartName := app.Get("chart").(string)
			localVersion := app.Get("version").(string)

			chart := getHelmChart(chartName)
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
	fmt.Println("Updating chart info...")
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
