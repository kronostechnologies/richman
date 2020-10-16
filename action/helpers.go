package action

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"
)

type TemplateField struct {
	Name string
	Optional bool
	Default string
}

func ListTemplateFields(t *template.Template) []TemplateField {
	return listNodeFields(t.Tree.Root, nil)
}

func listNodeFields(node parse.Node, res []TemplateField) []TemplateField {
	if node.Type() == parse.NodeAction {

		field, err := NewField(node.String())
		if err != nil {
			log.Printf("Error: %s. Skipping node %s", err, node.String())
			return res
		}

		add := true
		for _, v := range res {
			if v == field {
				add = false
				break
			}
		}

		if add {
			res = append(res, field)
		}
	}

	if ln, ok := node.(*parse.ListNode); ok {
		for _, n := range ln.Nodes {
			res = listNodeFields(n, res)
		}
	}
	return res
}

func NewField (node string) (TemplateField, error) {
	re := regexp.MustCompile(`^{{(?:default\s+([^\s]+)\s+)?\.([a-zA-Z0-9]+)}}$`)

	submatches := re.FindStringSubmatch(node)
	if submatches == nil {
		err := errors.New(fmt.Sprintf("No match found for string %s", node))
		return TemplateField{}, err
	}

	return TemplateField{
		Name: submatches[2],
		Default: strings.Trim(submatches[1], "\""),
		Optional: submatches[1] != "",
	}, nil
}