package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Harvey-OS/ninep/protocol"
	"github.com/google/go-github/github"
	"github.com/sirnewton01/ghfs/dynamic"
	"log"
	"path"
	"strconv"
	"strings"
	"text/template"
)

var (
	issueMarkdown = template.Must(template.New("issue").Funcs(funcMap).Parse(
		`# {{ .Title  }} (#{{ .Number }})

State: {{ .State }} - [{{ .User.Login }}](../../../{{ .User.Login }}) opened this issue {{ .CreatedAt }} - {{ .Comments }} comments

Assignee: {{if .Assignee}} [{{ .Assignee.Login }}](../../../{{ .Assignee.Login }}) {{else}} Not Assigned {{end}}

Labels: {{range .Labels}}{{.Name}} {{end}}

{{ markdown .Body }}

`))

	commentMarkdown = template.Must(template.New("comment").Funcs(funcMap).Parse(
		`## [{{ .User.Login }}](../../../{{ .User.Login }}) commented {{ .CreatedAt }} ({{ .AuthorAssociation }})

{{ markdown .Body }}

`))
)

type IssuesHandler struct {
	handler *dynamic.BasicDirHandler
	options *github.IssueListByRepoOptions
	filter  map[string]bool
}

func NewIssuesHandler(server *dynamic.Server, repoPath string) {
	handler := &IssuesHandler{}
	handler.options = &github.IssueListByRepoOptions{State: "open"}
	handler.handler = &dynamic.BasicDirHandler{server}

	server.AddFileEntry(path.Join(repoPath, "issues"), handler)
	NewIssuesCtl(server, path.Join(repoPath, "issues"), handler)
}

func (ih *IssuesHandler) WalkChild(name string, child string) (int, error) {
	idx, _ := ih.handler.WalkChild(name, child)
	if idx == -1 {
		number, err := strconv.Atoi(strings.Replace(child, ".md", "", 1))
		if err != nil {
			return idx, fmt.Errorf("Issue %s not found", child)
		}
		repo := path.Base(path.Dir(name))
		owner := path.Base(path.Dir(path.Dir(name)))

		log.Printf("Checking if issue %d exists\n", number)
		issue, _, err := client.Issues.Get(context.Background(), owner, repo, number)
		if err != nil {
			return idx, err
		}

		NewIssue(ih.handler.S, owner, repo, *issue.Number)
	}

	return ih.handler.WalkChild(name, child)
}

func (ih *IssuesHandler) refresh(owner string, repo string) error {
	log.Printf("Listing issues for repo %v/%v\n", owner, repo)
	ih.options.ListOptions = github.ListOptions{PerPage: 1}
	ih.filter = make(map[string]bool)
	ih.filter["ctl"] = true

	for {
		issues, resp, err := client.Issues.ListByRepo(context.Background(), owner, repo, ih.options)
		if err != nil {
			return err
		}

		for _, issue := range issues {
			NewIssue(ih.handler.S, owner, repo, *issue.Number)
			ih.filter[fmt.Sprintf("%d.md", *issue.Number)] = true
		}

		if resp.NextPage == 0 {
			break
		}

		ih.options.Page = resp.NextPage
	}

	return nil
}

func (ih *IssuesHandler) Open(name string, mode protocol.Mode) error {
	return nil
}

func (ih *IssuesHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating an issue is not supported")
}

func (ih *IssuesHandler) Stat(name string) (protocol.QID, error) {
	return ih.handler.Stat(name)
}

func (ih *IssuesHandler) Length(name string) (uint64, error) {
	return ih.handler.Length(name)
}

func (ih *IssuesHandler) Wstat(name string, qid protocol.QID, length uint64) error {
	return ih.handler.Wstat(name, qid, length)
}

func (ih *IssuesHandler) Remove(name string) error {
	return fmt.Errorf("Removing issues ins't supported.")
}

// This is a copy of what is in BasicDirHandler except that it does an extra filter check on the matches.
// This could be a candidate for an abstraction in the future if there are more cases where it needs to
//  be customized.
func (ih *IssuesHandler) getDir(name string, length bool) ([]byte, error) {
	matches := ih.handler.S.MatchFiles(func(f *dynamic.FileEntry) bool {
		matched := strings.HasPrefix(f.Name, name+"/") && strings.Count(name, "/") == strings.Count(f.Name, "/")-1
		if !matched {
			return false
		}

		_, ok := ih.filter[path.Base(f.Name)]
		return ok
	})

	var bb bytes.Buffer

	for _, idx := range matches {
		match := &ih.handler.S.Files[idx]

		var b bytes.Buffer
		dir := protocol.Dir{}
		qid, err := match.Handler.Stat(match.Name)
		if err != nil {
			return []byte{}, err
		}
		qid.Path = uint64(idx)
		dir.QID = qid

		m := 0755
		if dir.QID.Type&protocol.QTDIR != 0 {
			m = m | protocol.DMDIR
		}
		dir.Mode = uint32(m)

		if length {
			l, err := match.Handler.Length(match.Name)
			if err != nil {
				return []byte{}, err
			}
			dir.Length = l
		}
		dir.Name = path.Base(match.Name)

		protocol.Marshaldir(&b, dir)
		bb.Write(b.Bytes())
	}

	return bb.Bytes(), nil
}

func (ih *IssuesHandler) Read(name string, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 {
		repo := path.Base(path.Dir(name))
		owner := path.Base(path.Dir(path.Dir(name)))
		err := ih.refresh(owner, repo)
		if err != nil {
			return []byte{}, err
		}
	}

	content, err := ih.getDir(name, true)
	if err != nil {
		return []byte{}, err
	}

	if offset >= int64(len(content)) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(len(content)) {
		return content[offset:], nil
	}

	return content[offset : offset+count], nil
}

func (ih *IssuesHandler) Write(name string, offset int64, buf []byte) (int64, error) {
	return ih.handler.Write(name, offset, buf)
}

type IssuesCtl struct {
	ih       *IssuesHandler
	readbuf  *bytes.Buffer
	writebuf *bytes.Buffer
}

func NewIssuesCtl(server *dynamic.Server, issuesPath string, ih *IssuesHandler) {
	handler := &IssuesCtl{ih, &bytes.Buffer{}, &bytes.Buffer{}}
	server.AddFileEntry(path.Join(issuesPath, "ctl"), handler)

	contents, _ := json.MarshalIndent(handler.ih.options, "", "     ")
	handler.readbuf.Write(contents)
}

func (ic *IssuesCtl) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("No children of the issues ctl file")
}

func (ic *IssuesCtl) Open(name string, mode protocol.Mode) error {
	ic.readbuf = &bytes.Buffer{}
	ic.writebuf = &bytes.Buffer{}

	contents, _ := json.MarshalIndent(ic.ih.options, "", "     ")
	ic.readbuf.Write(contents)

	return nil
}

func (ic *IssuesCtl) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating a child of an issue ctl is not supported")
}

func (ic *IssuesCtl) Stat(name string) (protocol.QID, error) {
	// There's only one version and it is always a file
	return protocol.QID{Version: 0, Type: protocol.QTFILE}, nil
}

func (ic *IssuesCtl) Length(name string) (uint64, error) {
	return uint64(ic.readbuf.Len()), nil
}

func (ic *IssuesCtl) Wstat(name string, qid protocol.QID, length uint64) error {
	// TODO catch potential panic
	ic.writebuf.Truncate(int(length))
	return nil
}

func (ic *IssuesCtl) Remove(name string) error {
	return fmt.Errorf("Removing issues ctl isn't supported.")
}

func (ic *IssuesCtl) Read(name string, offset int64, count int64) ([]byte, error) {
	if offset >= int64(ic.readbuf.Len()) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(ic.readbuf.Len()) {
		return ic.readbuf.Bytes()[offset:], nil
	}

	return ic.readbuf.Bytes()[offset : offset+count], nil
}

func (ic *IssuesCtl) Write(name string, offset int64, buf []byte) (int64, error) {
	// TODO consider offset
	length, err := ic.writebuf.Write(buf)
	if err != nil {
		return int64(length), err
	}

	// TODO handle multiple writes for the entire file
	err = json.Unmarshal(ic.writebuf.Bytes(), ic.ih.options)
	if err != nil {
		return int64(length), err
	}

	err = ic.ih.refresh(path.Base(path.Dir(path.Dir(path.Dir(name)))), path.Base(path.Dir(path.Dir(name))))

	return int64(length), err
}

type Issue struct {
	number  int
	readbuf *bytes.Buffer
}

func NewIssue(server *dynamic.Server, owner string, repo string, number int) {
	issue := &Issue{number, &bytes.Buffer{}}
	server.AddFileEntry(path.Join("/repos", owner, repo, "issues", fmt.Sprintf("%d.md", number)), issue)
}

func (i *Issue) Read(name string, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 {
		log.Printf("Fetching issue %d\n", i.number)
		owner := path.Base(path.Dir(path.Dir(path.Dir(name))))
		repo := path.Base(path.Dir(path.Dir(name)))

		i.readbuf.Truncate(0)
		issue, _, err := client.Issues.Get(context.Background(), owner, repo, i.number)
		if err != nil {
			return []byte{}, err
		}
		err = issueMarkdown.Execute(i.readbuf, *issue)
		if err != nil {
			return []byte{}, err
		}
		comments, _, err := client.Issues.ListComments(context.Background(), owner, repo, i.number, nil)
		for _, comment := range comments {
			bb := bytes.Buffer{}
			err := commentMarkdown.Execute(&bb, *comment)
			if err != nil {
				return []byte{}, err
			}
			i.readbuf.Write(bb.Bytes())
		}

	}

	if offset >= int64(i.readbuf.Len()) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(i.readbuf.Len()) {
		return i.readbuf.Bytes()[offset:], nil
	}

	return i.readbuf.Bytes()[offset : offset+count], nil
}

func (i *Issue) Write(name string, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Writing issues is not supported")
}

func (i *Issue) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("No children of issues")
}

func (i *Issue) Open(name string, mode protocol.Mode) error {
	return nil
}

func (i *Issue) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating a child of an issue is not supported")
}

func (i *Issue) Stat(name string) (protocol.QID, error) {
	// There's only one version and it is always a file
	return protocol.QID{Version: 0, Type: protocol.QTFILE}, nil
}

func (i *Issue) Length(name string) (uint64, error) {
	return uint64(i.readbuf.Len()), nil
}

func (i *Issue) Wstat(name string, qid protocol.QID, length uint64) error {
	return fmt.Errorf("Truncation of issues isn't supported.")
}

func (i *Issue) Remove(name string) error {
	return fmt.Errorf("Removing issues isn't supported.")
}
