package markform

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/russross/blackfriday.v2"
)

var (
	formVarPattern = regexp.MustCompile(`(\w+)(\*??) = (.+)`)
)

func Unmarshal(mdInput []byte, v interface{}) error {
	md := blackfriday.New()
	tree := md.Parse(mdInput)
	t := reflect.Indirect(reflect.ValueOf(v)).Type()
	tree.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if node.Type == blackfriday.Text {
			groups := formVarPattern.FindStringSubmatch(string(node.Literal))
			if groups != nil {
				fn := groups[1]
				value := groups[3]
				f, ok := t.FieldByName(fn)
				if ok {
					fv := reflect.Indirect(reflect.ValueOf(v)).FieldByName(fn)
					if boolCheckBoxPattern.MatchString(string(f.Tag)) {
						if strings.HasPrefix(value, "[x]") {
							fv.SetBool(true)
						} else if strings.HasPrefix(value, "[]") {
							fv.SetBool(false)
						}
					} else if textPattern.MatchString(string(f.Tag)) {
						endOfText := strings.Index(value, "___")
						if endOfText != -1 {
							value = value[:endOfText]
						}
						g := textPattern.FindStringSubmatch(string(f.Tag))
						if g[2] != "" {
							size, _ := strconv.Atoi(g[4])
							if len(value) > size {
								value = value[:size]
							}
						}
						// TODO handle multi-line
						fv.SetString(value)
					}
				}
			}
		}
		return blackfriday.GoToNext
	})
	return nil
}
