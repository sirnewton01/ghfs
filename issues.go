package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Harvey-OS/ninep/protocol"
	"github.com/google/go-github/github"
	"github.com/sirnewton01/ghfs/dynamic"
	"github.com/sirnewton01/ghfs/markform"
	"log"
	"path"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
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

	issueFilterMarkdown = template.Must(template.New("issueFilter").Funcs(funcMap).Parse(
		`# Filters

Use these filters to control the issues that are shown in this directory. This file
uses restful markdown. See the README.md at the top level of this filesystem
for more details on how to work with the format.

{{ markform . "Milestone" }}

{{ markform . "State" }}

{{ markform . "Assignee" }}

{{ markform . "Creator" }}

{{ markform . "Mentioned" }}

Commonly used labels include bug, enhancement and task.

{{ markform . "Labels" }}

{{ markform . "Since" }}
`))
)

type IssuesFilter struct {
	Milestone string    ` = ___`
	State     string    ` = () open () closed () all`
	Assignee  string    ` = ___`
	Creator   string    ` = ___`
	Mentioned string    ` = ___`
	Labels    []string  ` = ,, ___`
	Since     time.Time ` = 2006-01-02T15:04:05Z`
}

type IssuesHandler struct {
	handler *dynamic.BasicDirHandler
	options *github.IssueListByRepoOptions
	filter  map[string]bool
	mutex   sync.Mutex
}

func NewIssuesHandler(server *dynamic.Server, repoPath string) {
	handler := &IssuesHandler{}
	handler.options = &github.IssueListByRepoOptions{State: "open"}
	handler.handler = &dynamic.BasicDirHandler{server, func(name string) bool {
		if handler.filter == nil {
			return true
		}

		_, ok := handler.filter[name]
		return ok
	}}

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
	ih.mutex.Lock()
	defer ih.mutex.Unlock()

	log.Printf("Listing issues for repo %v/%v\n", owner, repo)
	ih.options.ListOptions = github.ListOptions{PerPage: 1}
	ih.filter = make(map[string]bool)
	ih.filter["/repos/"+owner+"/"+repo+"/issues/filter.md"] = true

	for {
		issues, resp, err := client.Issues.ListByRepo(context.Background(), owner, repo, ih.options)
		if err != nil {
			return err
		}

		for _, issue := range issues {
			NewIssue(ih.handler.S, owner, repo, *issue.Number)
			ih.filter[fmt.Sprintf("/repos/%s/%s/issues/%d.md", owner, repo, *issue.Number)] = true
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

func (ih *IssuesHandler) Read(name string, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 {
		repo := path.Base(path.Dir(name))
		owner := path.Base(path.Dir(path.Dir(name)))
		err := ih.refresh(owner, repo)
		if err != nil {
			return []byte{}, err
		}
	}
	return ih.handler.Read(name, offset, count)
}

func (ih *IssuesHandler) Write(name string, offset int64, buf []byte) (int64, error) {
	return ih.handler.Write(name, offset, buf)
}

type IssuesCtl struct {
	ih       *IssuesHandler
	readbuf  *bytes.Buffer
	writebuf *bytes.Buffer
	mutex    sync.Mutex
}

func NewIssuesCtl(server *dynamic.Server, issuesPath string, ih *IssuesHandler) {
	handler := &IssuesCtl{ih: ih, readbuf: &bytes.Buffer{}, writebuf: &bytes.Buffer{}}
	server.AddFileEntry(path.Join(issuesPath, "filter.md"), handler)

	isf := IssuesFilter{}
	isf.Mentioned = handler.ih.options.Mentioned
	isf.State = handler.ih.options.State
	isf.Assignee = handler.ih.options.Assignee
	isf.Creator = handler.ih.options.Creator
	isf.Labels = handler.ih.options.Labels
	isf.Since = handler.ih.options.Since

	issueFilterMarkdown.Execute(handler.readbuf, isf)
}

func (ic *IssuesCtl) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("No children of the issues filter.md file")
}

func (ic *IssuesCtl) Open(name string, mode protocol.Mode) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	ic.readbuf = &bytes.Buffer{}
	ic.writebuf = &bytes.Buffer{}

	isf := IssuesFilter{}
	isf.Mentioned = ic.ih.options.Mentioned
	isf.State = ic.ih.options.State
	isf.Assignee = ic.ih.options.Assignee
	isf.Creator = ic.ih.options.Creator
	isf.Labels = ic.ih.options.Labels
	isf.Since = ic.ih.options.Since

	issueFilterMarkdown.Execute(ic.readbuf, isf)

	return nil
}

func (ic *IssuesCtl) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating a child of an issue filter.md is not supported")
}

func (ic *IssuesCtl) Stat(name string) (protocol.QID, error) {
	// There's only one version and it is always a file
	return protocol.QID{Version: 0, Type: protocol.QTFILE}, nil
}

func (ic *IssuesCtl) Length(name string) (uint64, error) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	return uint64(ic.readbuf.Len()), nil
}

func (ic *IssuesCtl) Wstat(name string, qid protocol.QID, length uint64) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	// TODO catch potential panic
	ic.writebuf.Truncate(int(length))
	return nil
}

func (ic *IssuesCtl) Remove(name string) error {
	return fmt.Errorf("Removing issues filter.md isn't supported.")
}

func (ic *IssuesCtl) Read(name string, offset int64, count int64) ([]byte, error) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	if offset >= int64(ic.readbuf.Len()) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(ic.readbuf.Len()) {
		return ic.readbuf.Bytes()[offset:], nil
	}

	return ic.readbuf.Bytes()[offset : offset+count], nil
}

func (ic *IssuesCtl) Write(name string, offset int64, buf []byte) (int64, error) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	// TODO consider offset
	length, err := ic.writebuf.Write(buf)
	if err != nil {
		return int64(length), err
	}

	// TODO handle multiple writes for the entire file
	isf := IssuesFilter{}
	err = markform.Unmarshal(ic.writebuf.Bytes(), &isf)
	if err != nil {
		return int64(length), err
	}

	ic.ih.options.Milestone = isf.Milestone
	ic.ih.options.State = isf.State
	ic.ih.options.Assignee = isf.Assignee
	ic.ih.options.Creator = isf.Creator
	ic.ih.options.Mentioned = isf.Mentioned
	ic.ih.options.Labels = isf.Labels
	ic.ih.options.Since = isf.Since

	err = ic.ih.refresh(path.Base(path.Dir(path.Dir(path.Dir(name)))), path.Base(path.Dir(path.Dir(name))))

	return int64(length), err
}

type Issue struct {
	number  int
	readbuf *bytes.Buffer
	mutex   sync.Mutex
}

func NewIssue(server *dynamic.Server, owner string, repo string, number int) {
	issue := &Issue{number: number, readbuf: &bytes.Buffer{}}
	server.AddFileEntry(path.Join("/repos", owner, repo, "issues", fmt.Sprintf("%d.md", number)), issue)
}

func (i *Issue) Read(name string, offset int64, count int64) ([]byte, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

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
	i.mutex.Lock()
	defer i.mutex.Unlock()

	return uint64(i.readbuf.Len()), nil
}

func (i *Issue) Wstat(name string, qid protocol.QID, length uint64) error {
	return fmt.Errorf("Truncation of issues isn't supported.")
}

func (i *Issue) Remove(name string) error {
	return fmt.Errorf("Removing issues isn't supported.")
}
