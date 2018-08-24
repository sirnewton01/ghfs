package markform

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	textPattern         = regexp.MustCompilePOSIX(`(\*?) = ___((\[([0-9]+)\])?)`)
	boolCheckBoxPattern = regexp.MustCompile(`(\*??) = \[\]$`)
	radioPattern        = regexp.MustCompile(`(\*??) = ((\(\) .*)+)`)
	checkboxPattern     = regexp.MustCompile(`(\*??) = ((\[\] .*)+)`)
	listPattern         = regexp.MustCompile(`(\*??) = ,, ___`)
	timePattern         = regexp.MustCompile(`(\*??) = 2006-01-02T15:04:05Z`)
)

// Marshal a specified field from a struct
//  in markform.
func Marshal(v interface{}, fn string) string {
	t := reflect.TypeOf(v)

	f, ok := t.FieldByName(fn)

	if !ok {
		return ""
	}

	tag := string(f.Tag)

	if textPattern.MatchString(tag) {
		value := reflect.ValueOf(v).FieldByName(fn).String()
		components := textPattern.FindStringSubmatch(tag)
		if components[2] != "" {
			limit, _ := strconv.Atoi(components[4])
			if len(value) > limit {
				value = value[:limit]
			}
		}
		return fmt.Sprintf("%s%s = %s___%s", fn, components[1], value, components[2])
	} else if boolCheckBoxPattern.MatchString(tag) {
		value := reflect.ValueOf(v).FieldByName(fn).Bool()
		components := boolCheckBoxPattern.FindStringSubmatch(tag)
		checkbox := "["
		if value {
			checkbox = checkbox + "x]"
		} else {
			checkbox = checkbox + "]"
		}
		return fmt.Sprintf("%s%s = %s", fn, components[1], checkbox)
	} else if radioPattern.MatchString(tag) {
		value := reflect.ValueOf(v).FieldByName(fn).String()
		tag = strings.Replace(tag, "() "+value, "(x) "+value, 1)
		return fn + tag
	} else if checkboxPattern.MatchString(tag) {
		length := reflect.ValueOf(v).FieldByName(fn).Len()
		for idx := 0; idx < length; idx++ {
			value := reflect.ValueOf(v).FieldByName(fn).Index(idx).String()
			tag = strings.Replace(tag, "[] "+value, "[x] "+value, 1)
		}
		return fn + tag
	} else if listPattern.MatchString(tag) {
		components := listPattern.FindStringSubmatch(tag)
		list := ""
		length := reflect.ValueOf(v).FieldByName(fn).Len()
		for idx := 0; idx < length; idx++ {
			value := reflect.ValueOf(v).FieldByName(fn).Index(idx).String()
			list = list + " ,, " + value
		}
		list = list + " ,, ___"

		return fmt.Sprintf("%s%s =%s", fn, components[1], list)
	} else if timePattern.MatchString(tag) {
		components := timePattern.FindStringSubmatch(tag)
		t := reflect.ValueOf(v).FieldByName(fn).Interface().(time.Time)
		return fmt.Sprintf("%s%s = %s", fn, components[1], t.Format(time.RFC3339))
	}

	return ""
}
