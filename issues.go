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
		`# Title = {{ .Title  }}___

* State = {{ .State }}___
* OpenedBy: [{{ .User.Login }}](../../../{{ .User.Login }})
* CreatedAt: {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
* Assignee = {{if .Assignee}} [{{ .Assignee.Login }}](../../../{{ .Assignee.Login }}) {{else}}Not Assigned{{end}}
* Labels = {{range .Labels}},, {{.Name}} {{end}} ,, ___

Body =

{{ .Body }}___

`))

	commentMarkdown = template.Must(template.New("comment").Funcs(funcMap).Parse(
		`## Comment

* User: [{{ .User.Login }}](../../../{{ .User.Login }}) 
* CreatedAt: {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}

Body =

{{ .Body }}___

`))

	issuesListMarkdown = template.Must(template.New("issueList").Funcs(funcMap).Parse(
		`# Issues

This is a list of issues for the project. You can change the filter by editing filter.md, save it and Get this list again.

{{ range . }}  * {{ .Number }}.md [{{ .State }}] - {{ .Title }} - [ {{ range .Labels }}{{ .Name }} {{ end }}] - {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }} - {{ .Comments }}
{{ end }}

`))

	issueFilterMarkdown = template.Must(template.New("issueFilter").Funcs(funcMap).Parse(
		`# Filter

Use this filter to control the issues that are shown in this directory and the issues list. This file
uses restful markdown. See the 0intro.md at the top level of this filesystem
for more details on how to work with the format.

* {{ markform . "Milestone" }}
* {{ markform . "State" }}
* {{ markform . "Assignee" }}
* {{ markform . "Creator" }}
* {{ markform . "Mentioned" }}

Commonly used labels include bug, enhancement and task.

* {{ markform . "Labels" }}
* {{ markform . "Since" }}

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
	dynamic.BasicDirHandler
	options *github.IssueListByRepoOptions
	filter  map[string]bool
	mutex   sync.Mutex
}

func NewIssuesHandler(repoPath string) {
	handler := &IssuesHandler{}
	handler.options = &github.IssueListByRepoOptions{State: "open"}
	handler.BasicDirHandler = dynamic.BasicDirHandler{server, func(name string) bool {
		if handler.filter == nil {
			return true
		}

		_, ok := handler.filter[name]
		return ok
	}}

	server.AddFileEntry(path.Join(repoPath, "issues"), handler)
	NewIssuesCtl(server, path.Join(repoPath, "issues"), handler)
	NewIssuesListHandler(path.Join(repoPath, "issues"), handler)
}

func (ih *IssuesHandler) WalkChild(name string, child string) (int, error) {
	idx, _ := ih.BasicDirHandler.WalkChild(name, child)
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

		NewIssue(server, owner, repo, issue)
	}

	return ih.BasicDirHandler.WalkChild(name, child)
}

func (ih *IssuesHandler) refresh(owner string, repo string) error {
	ih.mutex.Lock()
	defer ih.mutex.Unlock()

	log.Printf("Listing issues for repo %v/%v\n", owner, repo)
	ih.options.ListOptions = github.ListOptions{PerPage: 1}
	ih.filter = make(map[string]bool)
	ih.filter["/repos/"+owner+"/"+repo+"/issues/filter.md"] = true
	ih.filter["/repos/"+owner+"/"+repo+"/issues/0list.md"] = true

	for {
		issues, resp, err := client.Issues.ListByRepo(context.Background(), owner, repo, ih.options)
		if err != nil {
			return err
		}

		for _, issue := range issues {
			NewIssue(server, owner, repo, issue)
			ih.filter[fmt.Sprintf("/repos/%s/%s/issues/%d.md", owner, repo, *issue.Number)] = true
		}

		if resp.NextPage == 0 {
			break
		}

		ih.options.Page = resp.NextPage
	}

	return nil
}

func (ih *IssuesHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	return nil
}

func (ih *IssuesHandler) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 {
		repo := path.Base(path.Dir(name))
		owner := path.Base(path.Dir(path.Dir(name)))
		err := ih.refresh(owner, repo)
		if err != nil {
			return []byte{}, err
		}
	}
	return ih.BasicDirHandler.Read(name, fid, offset, count)
}

type IssuesCtl struct {
	ih       *IssuesHandler
	readbuf  *bytes.Buffer
	writefid protocol.FID
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

func (ic *IssuesCtl) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	if mode&protocol.ORDWR != 0 || mode&protocol.OWRITE != 0 {
		if ic.writefid != 0 {
			return fmt.Errorf("Filter doesn't support concurrent writes")
		}

		ic.writefid = fid
		ic.writebuf = &bytes.Buffer{}

		isf := IssuesFilter{}
		isf.Mentioned = ic.ih.options.Mentioned
		isf.State = ic.ih.options.State
		isf.Assignee = ic.ih.options.Assignee
		isf.Creator = ic.ih.options.Creator
		isf.Labels = ic.ih.options.Labels
		isf.Since = ic.ih.options.Since

		issueFilterMarkdown.Execute(ic.writebuf, isf)
	}

	if mode == protocol.OREAD {
		ic.readbuf = &bytes.Buffer{}

		isf := IssuesFilter{}
		isf.Mentioned = ic.ih.options.Mentioned
		isf.State = ic.ih.options.State
		isf.Assignee = ic.ih.options.Assignee
		isf.Creator = ic.ih.options.Creator
		isf.Labels = ic.ih.options.Labels
		isf.Since = ic.ih.options.Since

		issueFilterMarkdown.Execute(ic.readbuf, isf)
	}

	return nil
}

func (ic *IssuesCtl) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating a child of an issue filter.md is not supported")
}

func (ic *IssuesCtl) Stat(name string) (protocol.Dir, error) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	// There's only one version and it is always a file
	return protocol.Dir{QID: protocol.QID{Version: 0, Type: protocol.QTFILE}, Length: uint64(ic.readbuf.Len())}, nil
}

func (ic *IssuesCtl) Wstat(name string, dir protocol.Dir) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	ic.writebuf.Truncate(int(dir.Length))
	return nil
}

func (ic *IssuesCtl) Remove(name string) error {
	return fmt.Errorf("Removing issues filter.md isn't supported.")
}

func (ic *IssuesCtl) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
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

func (ic *IssuesCtl) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	if fid != ic.writefid {
		return int64(len(buf)), nil
	}

	// TODO consider offset
	length, err := ic.writebuf.Write(buf)
	if err != nil {
		return int64(length), err
	}

	return int64(length), nil
}

func (ic *IssuesCtl) Clunk(name string, fid protocol.FID) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	if fid != ic.writefid {
		return nil
	}
	ic.writefid = 0

	if len(ic.writebuf.Bytes()) == 0 {
		return nil
	}

	isf := IssuesFilter{}
	err := markform.Unmarshal(ic.writebuf.Bytes(), &isf)
	if err != nil {
		return err
	}

	ic.ih.options.Milestone = isf.Milestone
	ic.ih.options.State = isf.State
	ic.ih.options.Assignee = isf.Assignee
	ic.ih.options.Creator = isf.Creator
	ic.ih.options.Mentioned = isf.Mentioned
	ic.ih.options.Labels = isf.Labels
	ic.ih.options.Since = isf.Since

	return ic.ih.refresh(path.Base(path.Dir(path.Dir(path.Dir(name)))), path.Base(path.Dir(path.Dir(name))))
}

type Issue struct {
	number  int
	readbuf *bytes.Buffer
	mtime   time.Time
	mutex   sync.Mutex
}

func NewIssue(server *dynamic.Server, owner string, repo string, i *github.Issue) {
	issue := &Issue{number: *i.Number, readbuf: &bytes.Buffer{}}

	issue.mtime = i.GetUpdatedAt()
	issueMarkdown.Execute(issue.readbuf, *i)
	// TODO consider comments too that can change the length and mtime of the issue

	comments, _, _ := client.Issues.ListComments(context.Background(), owner, repo, *i.Number, nil)
	for _, comment := range comments {
		if issue.mtime.Before(comment.GetUpdatedAt()) {
			issue.mtime = comment.GetUpdatedAt()
		}
		bb := bytes.Buffer{}
		err := commentMarkdown.Execute(&bb, *comment)
		if err == nil {
			issue.readbuf.Write(bb.Bytes())
		}
	}

	server.AddFileEntry(path.Join("/repos", owner, repo, "issues", fmt.Sprintf("%d.md", *i.Number)), issue)
}

func (i *Issue) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
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
		i.mtime = issue.GetUpdatedAt()
		err = issueMarkdown.Execute(i.readbuf, *issue)
		if err != nil {
			return []byte{}, err
		}
		comments, _, err := client.Issues.ListComments(context.Background(), owner, repo, i.number, nil)
		for _, comment := range comments {
			if i.mtime.Before(comment.GetUpdatedAt()) {
				i.mtime = comment.GetUpdatedAt()
			}
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

func (i *Issue) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Writing issues is not supported")
}

func (i *Issue) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("No children of issues")
}

func (i *Issue) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	return nil
}

func (i *Issue) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating a child of an issue is not supported")
}

func (i *Issue) Stat(name string) (protocol.Dir, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	t := i.mtime.Unix()
	if i.mtime.IsZero() {
		t = 0
	}

	// There's only one version and it is always a file
	return protocol.Dir{QID: protocol.QID{Version: 0, Type: protocol.QTFILE}, Length: uint64(i.readbuf.Len()), Mtime: uint32(t)}, nil
}

func (i *Issue) Wstat(name string, dir protocol.Dir) error {
	return fmt.Errorf("Truncation of issues isn't supported.")
}

func (i *Issue) Remove(name string) error {
	return fmt.Errorf("Removing issues isn't supported.")
}

func (i *Issue) Clunk(name string, fid protocol.FID) error {
	return nil
}

type IssuesListHandler struct {
	dynamic.StaticFileHandler
	ih *IssuesHandler
	mu sync.Mutex
}

func NewIssuesListHandler(repoIssuesPath string, ih *IssuesHandler) {
	server.AddFileEntry(path.Join(repoIssuesPath, "0list.md"), &IssuesListHandler{StaticFileHandler: dynamic.StaticFileHandler{[]byte{}}, ih: ih})
}

func (ilh *IssuesListHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	ilh.mu.Lock()
	defer ilh.mu.Unlock()

	issues := []*github.Issue{}

	repo := path.Base(path.Dir(path.Dir(name)))
	owner := path.Base(path.Dir(path.Dir(path.Dir(name))))

	ilh.ih.mutex.Lock()
	defer ilh.ih.mutex.Unlock()

	ilh.ih.options.ListOptions = github.ListOptions{PerPage: 1}

	for {
		i, resp, err := client.Issues.ListByRepo(context.Background(), owner, repo, ilh.ih.options)
		if err != nil {
			return err
		}

		for _, issue := range i {
			issues = append(issues, issue)
		}

		if resp.NextPage == 0 {
			break
		}

		ilh.ih.options.Page = resp.NextPage
	}

	buf := bytes.Buffer{}
	err := issuesListMarkdown.Execute(&buf, issues)
	if err != nil {
		return err
	}

	ilh.StaticFileHandler.Content = buf.Bytes()

	return ilh.StaticFileHandler.Open(name, fid, mode)
}
