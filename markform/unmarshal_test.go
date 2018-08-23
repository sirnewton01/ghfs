package markform

import (
	"testing"
)

func TestUnmarshalDocument(t *testing.T) {
	type Person struct {
		Name         string   `* = ___[50]`           // text field maximum size 50
		Gender       string   `* = () male () female` // one of the specified values
		Student      bool     `* = []`                // true/false, checked/not
		Affiliations []string ` = ,, ___`             // list of any values from the user
		Description  string   ` = ___`                // Unbounded, maybe  multi-line string
		Education    []string ` = [] elementary [] secondary [] post-secondary`
	}

	document :=
		`# Name* = John Doe___[50]  - Personal Information

Please ensure that the information is entered correctly. If you have any
questions you can email the [support team](mailto:support@example.com).

Description = Conscientious student___

Gender* = (x) male () female

Student* = [x]

Affiliations = ,, Chess Club ,, ___

Education = [x] elementary [x] secondary [] post-secondary

Save this file to record any changes to the person record.

`

	person := Person{}
	err := Unmarshal([]byte(document), &person)
	if err != nil {
		t.Error(err)
	}

	if !person.Student {
		t.Errorf("Student flag not set")
	}

	if person.Description != "Conscientious student" {
		t.Errorf("Unexpected description: %s\n", person.Description)
	}

	if person.Name != "John Doe" {
		t.Errorf("Unexpected name: %s\n", person.Name)
	}
}
