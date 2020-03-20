package action

import (
	"fmt"
	"github.com/pelletier/go-toml"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type AppsList struct {
	Filename     string
	AppFilters   []string
}

func (c *AppsList) Run() error {
	data, err := toml.LoadFile(c.Filename)
	if err != nil {
		return err
	}
	apps := data.Get("apps").(*toml.Tree)

	if err = listCommand(apps, c); err != nil {
		return err
	}


	return nil
}

func listCommand(apps *toml.Tree, o *AppsList) error {
	noFilter := len(o.AppFilters) == 0
	appMatchers := CreateAppMatchers(o.AppFilters)

	versions := make(map[string]string)
	width := 0

	for _, name := range apps.Keys() {
		app := apps.Get(name).(*toml.Tree)

		appVersions := getVersions(name, app)

		for k, version := range appVersions {
			if noFilter || findMatch(name, appMatchers) || findMatch(k, appMatchers) {
				versions[k] = version

				if len(k) > width {
					width = len(k)
				}

			}
		}
	}

	fmt.Printf("%-" + strconv.Itoa(width) + "s  %s\n", "APP", "VERSION")

	var keys []string
	for k, _ := range versions {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, x := range keys {
			fmt.Printf("%-" + strconv.Itoa(width) + "s  %s\n", x, versions[x])
	}

	return nil
}

func getVersions(name string, app *toml.Tree) map[string]string {
	var list = make(map[string]string)

	re := regexp.MustCompile(`^(?:([^.]*)\.)?image(\.t|T)ag$`)

	if sS, ok := app.Get("setString").(*toml.Tree); ok == true {

		for chartKey, chartVal := range sS.ToMap() {
			var listKey string

			substrings := re.FindStringSubmatch(chartKey)
			if len(substrings) == 0 {
				continue
			}

			if  subchart := substrings[1] ; subchart != "" {
				listKey = strings.Join([]string{name, subchart}, ".")
			} else {
				listKey = name
			}

			version := strings.TrimPrefix(chartVal.(string), "version-")
			list[listKey] = version
		}
	}
	return list
}
