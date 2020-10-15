package action

import(
	"github.com/Masterminds/sprig"
	"io/ioutil"
	"log"
	"testing"
	"text/template"
)

func TestListNodeField(t *testing.T) {
	fields := []string{"cpu", "memory", "name"}
	configMap, err := ioutil.ReadFile("../tests/ops-config-map.yaml")
	if err != nil {
		log.Fatal(err)
	}

	jobTemplate := template.Must(template.New("configmap").Funcs(sprig.TxtFuncMap()).Parse(string(configMap)))
	templateField := ListTemplateFields(jobTemplate)
	for _, field := range templateField {
		found := false
		for _, f := range fields {
			if f == field.Name {
				found = true
			}
		}
		if !found {
			t.Errorf("Field '%s' was not found in template", field.Name)
		}
	}
}