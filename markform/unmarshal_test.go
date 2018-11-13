package markform

import (
	"testing"
	"time"

	"gopkg.in/russross/blackfriday.v2"
)

func TestUnmarshalDocument(t *testing.T) {
	type Person struct {
		Name         string    `* = ___[50]`           // text field maximum size 50
		Gender       string    `* = () male () female` // one of the specified values
		Student      bool      `* = []`                // true/false, checked/not
		Affiliations []string  ` = ,, ___`             // list of any values from the user
		Description  string    ` = ___`                // Unbounded, maybe  multi-line string
		Education    []string  ` = [] elementary [] secondary [] post-secondary`
		DateOfBirth  time.Time ` = 2006-01-02T15:04:05Z`
	}

	document :=
		`# Name* = John Doe___[50]  - Personal Information

Please ensure that the information is entered correctly. If you have any
questions you can email the [support team](mailto:support@example.com).

Description = Conscientious
student___

* Gender* = (x) male () female
* Student* = [x]
* Affiliations = ,, Chess Club ,, ___
* Education = [x] elementary [x] secondary [] post-secondary
* DateOfBirth = 2010-01-02T15:04:05Z

Save this file to record any changes to the person record.

`

	person := Person{}
	md := blackfriday.New(blackfriday.WithExtensions(blackfriday.FencedCode))
	tree := md.Parse([]byte(document))
	err := Unmarshal(tree, &person)
	if err != nil {
		t.Error(err)
	}

	if !person.Student {
		t.Errorf("Student flag not set")
	}

	if person.Description != "Conscientious\nstudent" {
		t.Errorf("Unexpected description: %s\n", person.Description)
	}

	if person.Name != "John Doe" {
		t.Errorf("Unexpected name: %s\n", person.Name)
	}

	if person.Gender != "male" {
		t.Errorf("Unexpected gender: %s\n", person.Gender)
	}

	if len(person.Education) != 2 {
		t.Errorf("Expected two education entries\n")
	}
	if person.Education[0] != "elementary" && person.Education[1] != "secondary" {
		t.Errorf("Elementary not found in education: %v\n", person.Education)
	}
	if person.Education[0] != "secondary" && person.Education[1] != "secondary" {
		t.Errorf("Secondary not found in education: %v\n", person.Education)
	}

	if len(person.Affiliations) != 1 {
		t.Errorf("Expected one affiliation\n")
	}
	if person.Affiliations[0] != "Chess Club" {
		t.Errorf("Unexpected affiliation: %v\n", person.Affiliations[0])
	}

	if person.DateOfBirth.Format(time.RFC3339) != "2010-01-02T15:04:05Z" {
		t.Errorf("Unexpected date of birth: %v\n", person.DateOfBirth)
	}

	document =
		`# Name* = John Doe___[50]  - Personal Information

Please ensure that the information is entered correctly. If you have any
questions you can email the [support team](mailto:support@example.com).

Description = 
` + "```" + `
Conscientious
student

* more info
` + "```" + `
___

* Gender* = (x) male () female
* Student* = [x]
* Affiliations = ,, Chess Club ,, ___
* Education = [x] elementary [x] secondary [] post-secondary
* DateOfBirth = 2010-01-02T15:04:05Z

Save this file to record any changes to the person record.

`

	person = Person{}
	md = blackfriday.New(blackfriday.WithExtensions(blackfriday.FencedCode))
	tree = md.Parse([]byte(document))
	err = Unmarshal(tree, &person)
	if err != nil {
		t.Error(err)
	}

	if person.Description != "Conscientious\nstudent\n\n* more info" {
		t.Errorf("Unexpected description: %s\n", person.Description)
	}
}
