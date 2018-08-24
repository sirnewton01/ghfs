package markform

import (
	"bytes"
	"testing"
	"text/template"
	"time"
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
		list         []string ` = ,, ___`
		requiredList []string `* = ,, ___`
	}

	v := astruct{list: []string{"value1", "another value", "something else"}}
	m := Marshal(v, "list")
	if "list = ,, value1 ,, another value ,, something else ,, ___" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{list: nil}
	m = Marshal(v, "list")
	if "list = ,, ___" != m {
		t.Errorf("Unexpected value %s\n", m)
	}

	v = astruct{requiredList: []string{"value"}}
	m = Marshal(v, "requiredList")
	if "requiredList* = ,, value ,, ___" != m {
		t.Errorf("Unexpected value %x\n", m)
	}
}

func TestMarshalTemplate(t *testing.T) {
	type Person struct {
		Name         string    `* = ___[50]`           // text field maximum size 50
		Gender       string    `* = () male () female` // one of the specified values
		Student      bool      `* = []`                // true/false, checked/not
		Affiliations []string  ` = ,, ___`             // list of any values from the user
		Description  string    ` = ___`                // Unbounded, maybe  multi-line string
		Education    []string  ` = [] elementary [] secondary [] post-secondary`
		DateOfBirth  time.Time ` = 2006-01-02T15:04:05Z07:00`
	}

	funcMap := map[string]interface{}{"markform": Marshal}

	personTemplate := template.Must(template.New("person").Funcs(funcMap).Parse(

		`# {{ markform . "Name" }} - Personal Information

Please ensure that the information is entered correctly. If you have any
questions you can email the [support team](mailto:support@example.com).

{{ markform . "Description" }}

{{ markform . "Gender" }}

{{ markform . "Student" }}

{{ markform . "Affiliations" }}

{{ markform . "Education" }}

{{ markform . "DateOfBirth" }}

Save this file to record any changes to the person record.

`))

	dob, err := time.Parse(time.RFC3339, "2010-01-02T15:04:05Z")
	if err != nil {
		panic(err)
	}

	person := Person{Name: "John Doe", Gender: "male", Student: true, Affiliations: []string{"Chess Club"}, Description: "Conscientious student", Education: []string{"elementary", "secondary"}, DateOfBirth: dob}
	buf := bytes.Buffer{}
	err = personTemplate.Execute(&buf, person)
	if err != nil {
		t.Error(err)
	}

	expected := `# Name* = John Doe___[50] - Personal Information

Please ensure that the information is entered correctly. If you have any
questions you can email the [support team](mailto:support@example.com).

Description = Conscientious student___

Gender* = (x) male () female

Student* = [x]

Affiliations = ,, Chess Club ,, ___

Education = [x] elementary [x] secondary [] post-secondary

DateOfBirth = 2010-01-02T15:04:05Z

Save this file to record any changes to the person record.

`

	if expected != string(buf.Bytes()) {
		t.Errorf("Unexpected value: %s\n", buf.Bytes())
	}
}
