package action

import (
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

		field := NewField(node.String())

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

func NewField (node string) TemplateField {
	re := regexp.MustCompile(`^{{(?:default\s+([^\s]+)\s+)?\.([a-zA-Z0-9]+)}}$`)

	submatches := re.FindStringSubmatch(node)

	return TemplateField{
		Name: submatches[2],
		Default: strings.Trim(submatches[1], "\""),
		Optional: submatches[1] != "",
	}
}