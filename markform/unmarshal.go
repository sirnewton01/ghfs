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
					} else if radioPattern.MatchString(string(f.Tag)) {
						g := radioPattern.FindStringSubmatch(string(f.Tag))
						options := strings.Split(g[2], "() ")
						for _, option := range options {
							option = strings.TrimRight(option, " ")
							if option != "" && strings.Contains(value, "(x) "+option) {
								fv.SetString(option)
								break
							}
						}
					} else if checkboxPattern.MatchString(string(f.Tag)) {
						g := checkboxPattern.FindStringSubmatch(string(f.Tag))
						options := strings.Split(g[2], "[] ")
						fv.Set(reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf("")), 0, 0))
						for _, option := range options {
							option = strings.TrimRight(option, " ")
							if option != "" && strings.Contains(value, "[x] "+option) {
								fv.Set(reflect.Append(fv, reflect.ValueOf(option)))
							}
						}
					} else if listPattern.MatchString(string(f.Tag)) {
						fv.Set(reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf("")), 0, 0))
						listitems := strings.Split(value, ",, ")
						for _, listitem := range listitems {
							if listitem == "" || listitem == "___" {
								continue
							}

							// Trim the trailing space
							listitem = listitem[:len(listitem)-1]

							fv.Set(reflect.Append(fv, reflect.ValueOf(listitem)))
						}
					}
				}
			}
		}
		return blackfriday.GoToNext
	})
	return nil
}
