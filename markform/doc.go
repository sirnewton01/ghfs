/*

Markform is a form language for describing forms on top of Markdown. The format is inspired
by Yevgeniy Brikman's https://github.com/brikis98/wmd notation with some modifications.
The goal is to support RESTful self-documenting entities that can be easily read and modified
using simple text editing and command line tools. More complex entities can be pretty printed
in a rich format, such as HTML or PDF for in-depth study.

The markform package provides tools to allow you to design markdown templates and link them to
your structs for the purpose of marshaling and unmarshaling values of the struct.

        type Person struct {
                Name           string   `* = ___[50]`            // text field maximum size 50
                Gender         string   `* = () male () female`  // one of the specified values
                Student        bool     `* = []`                 // true/false, checked/not
                Affiliations   []string ` = ,, ___`              // list of any values from the user
                Description    string   ` = ___`                 // Unbounded, maybe  multi-line string
                Education      []string ` = [] elementary [] secondary [] post-secondary`
        }

Note that the struct field tags provide the template of the suffix for each of the form elements.
They include information, such as the type of element (radio, text, check, multi-valued user defined
and multi-value predefined). It is subtle, but the template also indicates required field vs. optional
with an asterisk.

Along with the struct you can build a template for the entity using the templates package and helper
functions provided in this package like this. The template gives you freedom to layout the information
in a readable way and even add inline text that helps to guide the user.

        personTemplate := template.Must(template.New("person").Funcs(funcMap).Parse(
`# {{ markform . "Name" }} - Personal Information

Please ensure that the information is entered correctly. If you have any questions you can
email the [support team](mailto:support@example.com).

{{ markform . "Description" }}

{{ markform . "Gender" }}

{{ markform . "Student" }}

{{ markform . "Affiliations" }}

{{ markform . "Education" }}

Save this file to record any changes to the person record.

`))

*/
package markform
