package markform

import (
	"testing"
)

func TestMarshal_Text(t *testing.T) {
	type astruct struct {
		textField         string ` = ___`
		textFieldLimited  string ` = ___[10]`
		textFieldRequired string `* = ___`
	}

	v := astruct{textField: "some value"}
	m := Marshal(v, "textField")
	if "textField = some value___" != m {
		t.Errorf("Unexpected value: %s != %s", m, "textField = some value___")
	}

	v = astruct{textFieldLimited: "some value"}
	m = Marshal(v, "textFieldLimited")
	if "textFieldLimited = some value___[10]" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{textFieldLimited: "same value longer than 10"}
	m = Marshal(v, "textFieldLimited")
	if "textFieldLimited = same value___[10]" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{textFieldRequired: "required"}
	m = Marshal(v, "textFieldRequired")
	if "textFieldRequired* = required___" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{textField: "line1\nline2"}
	m = Marshal(v, "textField")
	if "textField = line1\nline2___" != m {
		t.Errorf("Unexpected value %s\n", m)
	}
}

func TestMarshal_BasicCheckBox(t *testing.T) {
	type astruct struct {
		checkbox         bool ` = []`
		requiredcheckbox bool `* = []`
	}

	v := astruct{checkbox: true}
	m := Marshal(v, "checkbox")
	if "checkbox = [x]" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{checkbox: false}
	m = Marshal(v, "checkbox")
	if "checkbox = []" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{requiredcheckbox: true}
	m = Marshal(v, "requiredcheckbox")
	if "requiredcheckbox* = [x]" != m {
		t.Errorf("Unexpected value %s\n", m)
	}
}

func TestMarshal_Radio(t *testing.T) {
	type astruct struct {
		radio         string ` = () the fox () hare () other`
		requiredRadio string `* = () bard () troll () newt`
	}

	v := astruct{radio: "the fox"}
	m := Marshal(v, "radio")
	if "radio = (x) the fox () hare () other" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{requiredRadio: "troll"}
	m = Marshal(v, "requiredRadio")
	if "requiredRadio* = () bard (x) troll () newt" != m {
		t.Errorf("Unexpected value %s\n", m)
	}
}

func TestMarshal_CheckBox(t *testing.T) {
	type astruct struct {
		check         []string ` = [] value1 [] value 2 [] some other value`
		requiredCheck []string `* = [] blue [] yellow [] red`
	}

	v := astruct{check: []string{"value1", "some other value"}}
	m := Marshal(v, "check")
	if "check = [x] value1 [] value 2 [x] some other value" != m {
		t.Errorf("Unexpected value %s\n", m)
	}
}

func TestMarshal_List(t *testing.T) {
        type astruct struct {
                list []string ` = ,, ___`
                requiredList []string `* = ,, ___`
        }

        v := astruct{list: []string {"value1", "another value", "something else"}}
        m := Marshal(v, "list")
        if "list = ,, value1 ,, another value ,, something else ,, ___" != m {
                t.Errorf("Unexpected value %s\n", m)
        }

        v = astruct{list: nil}
        m = Marshal(v, "list")
        if "list = ,, ___" != m {
                t.Errorf("Unexpected value %s\n", m)
        }

        v = astruct{requiredList: []string {"value"}}
        m = Marshal(v, "requiredList")
        if "requiredList* = ,, value ,, ___" != m {
                t.Errorf("Unexpected value %x\n", m)
        }
}
