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

func TestNewFieldAcceptedFormat(t *testing.T) {
	correctFieldFormat := []string{"{{default \"0.1\" .lowercase}}", "{{default \"0.1\" .lowercase123}}", "{{default \"0.1\" .camelCase}}", "{{default \"0.1\" .camelCase123}}"}
	for _, s := range correctFieldFormat {
		_, err := NewField(s)
		if err != nil {
			t.Errorf("Field '%s' should not return an error. Got error '%s'", s, err)
		}
	}
}