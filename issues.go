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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"gopkg.in/russross/blackfriday.v2"
)

var (
	issueMarkdown = template.Must(template.New("issue").Funcs(funcMap).Parse(
		`# {{ markform .Form "Title" }}

* {{ markform .Form "State" }}
* OpenedBy: [{{ .Issue.User.Login }}](../../../{{ .Issue.User.Login }})
* CreatedAt: {{ .Issue.CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
* {{ markform .Form "Assignee" }}
* {{ markform .Form "Labels" }}

{{ markform .Form "Body" }}


`))

	commentMarkdown = template.Must(template.New("comment").Funcs(funcMap).Parse(
		`## Comment
{{if .Comment}}
* User: [{{ .Comment.User.Login }}](../../../{{ .Comment.User.Login }}) 
* CreatedAt: {{ .Comment.CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
{{end}}
{{ markform .Form "Body" }}


`))

	issuesListMarkdown = template.Must(template.New("issueList").Funcs(funcMap).Parse(
		`# Issues

This is a list of issues for the project. You can change the filter by editing filter.md, save it and Get this list again. You can create a new issue by opening {{ .NewIssueNumber }}.md .

{{ range .Issues }}  * {{ .Number }}.md [{{ .State }}] - {{ .Title }} - [ {{ range .Labels }}{{ .Name }} {{ end }}] - {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }} - {{ .Comments }}
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
		issue, resp, err := uncachedClient.Issues.Get(context.Background(), owner, repo, number)
		if resp != nil && resp.Response.StatusCode == 404 {
			// We'll create a new issue provided that the number is just one greater
			//  than the largest issue number
			log.Printf("Checking if this could be a new issue\n")
			_, _, err2 := uncachedClient.Issues.Get(context.Background(), owner, repo, number-1)
			if err2 != nil {
				return idx, err2
			}

			log.Printf("Creating a new issue\n")
			title := "New Issue"
			body := ""
			labels := []string{}
			_, _, err2 = client.Issues.Create(context.Background(), owner, repo, &github.IssueRequest{Title: &title, Body: &body, Labels: &labels})
			if err2 != nil {
				return idx, err
			}

			issue, _, err = uncachedClient.Issues.Get(context.Background(), owner, repo, number)
		}
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
		issues, resp, err := uncachedClient.Issues.ListByRepo(context.Background(), owner, repo, ih.options)
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
	md := blackfriday.New(blackfriday.WithExtensions(blackfriday.FencedCode))
	tree := md.Parse(ic.writebuf.Bytes())
	err := markform.Unmarshal(tree, &isf)
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

type Comment struct {
	Comment *github.IssueComment
	Form    struct {
		Body string ` = ___`
	}
}

type Issue struct {
	mtime    time.Time
	Issue    *github.Issue
	Comments []Comment
	Form     struct {
		Title    string   ` = ___`
		Assignee string   ` = ___`
		State    string   ` = () open () closed`
		Labels   []string ` = ,, ___`
		Body     string   ` = ___`
	}

	readbuf  *bytes.Buffer
	writefid protocol.FID
	writebuf *bytes.Buffer
	mutex    sync.Mutex
}

func NewIssue(server *dynamic.Server, owner string, repo string, i *github.Issue) {
	issue := &Issue{readbuf: &bytes.Buffer{}}

	issue.mtime = i.GetUpdatedAt()

	log.Printf("Listing comments for issue %d\n", *i.Number)
	comments, _, _ := uncachedClient.Issues.ListComments(context.Background(), owner, repo, *i.Number, nil)
	for _, comment := range comments {
		if issue.mtime.Before(comment.GetUpdatedAt()) {
			issue.mtime = comment.GetUpdatedAt()
		}
	}

	server.AddFileEntry(path.Join("/repos", owner, repo, "issues", fmt.Sprintf("%d.md", *i.Number)), issue)
}

func (i *Issue) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if offset >= int64(i.readbuf.Len()) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(i.readbuf.Len()) {
		return i.readbuf.Bytes()[offset:], nil
	}

	return i.readbuf.Bytes()[offset : offset+count], nil
}

func (i *Issue) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if fid != i.writefid {
		return int64(len(buf)), nil
	}

	// TODO consider offset
	length, err := i.writebuf.Write(buf)
	if err != nil {
		return int64(length), err
	}

	return int64(length), nil
}

func (i *Issue) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("No children of issues")
}

func (i *Issue) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	owner := path.Base(path.Dir(path.Dir(path.Dir(name))))
	repo := path.Base(path.Dir(path.Dir(name)))
	fn := path.Base(name)
	n, err := strconv.Atoi(strings.Replace(fn, ".md", "", 1))
	if err != nil {
		return err
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	if mode == protocol.OREAD {
		i.readbuf.Truncate(0)
		log.Printf("Loading issue %d\n", n)
		issue, _, err := uncachedClient.Issues.Get(context.Background(), owner, repo, n)
		if err != nil {
			return err
		}
		i.mtime = issue.GetUpdatedAt()
		i.Issue = issue

		i.Form.Title = *issue.Title
		i.Form.Assignee = ""
		if issue.Assignee != nil {
			i.Form.Assignee = *issue.Assignee.Login
		}
		i.Form.State = *issue.State
		if issue.Body != nil {
			i.Form.Body = "\n```\n" + *issue.Body + "\n```\n"
		} else {
			i.Form.Body = "\n```\n\n```\n"
		}
		i.Form.Labels = []string{}
		if issue.Labels != nil {
			for _, l := range issue.Labels {
				i.Form.Labels = append(i.Form.Labels, *l.Name)
			}
		}

		err = issueMarkdown.Execute(i.readbuf, i)
		if err != nil {
			return err
		}

		i.Comments = []Comment{}
		log.Printf("Listing comments for issue %d\n", n)
		comments, _, err := uncachedClient.Issues.ListComments(context.Background(), owner, repo, n, nil)
		for idx, comment := range comments {
			if i.mtime.Before(comment.GetUpdatedAt()) {
				i.mtime = comment.GetUpdatedAt()
			}

			i.Comments = append(i.Comments, Comment{})
			i.Comments[idx].Comment = comment
			i.Comments[idx].Form.Body = "\n```\n" + *comment.Body + "\n```\n"

			bb := bytes.Buffer{}
			err := commentMarkdown.Execute(&bb, i.Comments[idx])
			if err != nil {
				return err
			}
			i.readbuf.Write(bb.Bytes())
		}

		// Comment template
		commentTemplate := Comment{}
		commentTemplate.Form.Body = "\n```\n\n```\n"
		i.Comments = append(i.Comments, commentTemplate)

		bb := bytes.Buffer{}
		err = commentMarkdown.Execute(&bb, commentTemplate)
		if err != nil {
			return err
		}
		i.readbuf.Write(bb.Bytes())
	}

	if mode&protocol.ORDWR != 0 || mode&protocol.OWRITE != 0 {
		if i.writefid != 0 {
			return fmt.Errorf("Issue doesn't support concurrent writes")
		}

		i.writefid = fid
		i.writebuf = &bytes.Buffer{}
	}

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
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.writebuf.Truncate(int(dir.Length))
	return nil
}

func (i *Issue) Remove(name string) error {
	return fmt.Errorf("Removing issues isn't supported.")
}

func (i *Issue) Clunk(name string, fid protocol.FID) error {
	owner := path.Base(path.Dir(path.Dir(path.Dir(name))))
	repo := path.Base(path.Dir(path.Dir(name)))
	fn := path.Base(name)
	n, err := strconv.Atoi(strings.Replace(fn, ".md", "", 1))
	if err != nil {
		return err
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	if fid != i.writefid {
		return nil
	}
	i.writefid = 0

	// No bytes were written this time, leave it alone
	if len(i.writebuf.Bytes()) == 0 {
		return nil
	}

	newi := &Issue{}
	md := blackfriday.New(blackfriday.WithExtensions(blackfriday.FencedCode))
	tree := md.Parse(i.writebuf.Bytes())

	// Split out the comments into their own documents
	newparent := tree
	comments := []*blackfriday.Node{}

	node := tree.FirstChild
	for ; node != nil; node = node.Next {
		if node.Type == blackfriday.Heading {
			if node.Prev != nil {
				newparent.LastChild = node.Prev
				node.Prev.Next = nil
			}
			node.Prev = nil

			newparent = blackfriday.NewNode(blackfriday.Document)
			newparent.FirstChild = node
			comments = append(comments, newparent)
		}

		node.Parent = newparent
	}
	comments = comments[1:]

	newparent.LastChild = node

	err = markform.Unmarshal(tree, &newi.Form)
	if err != nil {
		return err
	}

	// TODO collapse these individual edits into one

	if newi.Form.Body != i.Form.Body {
		log.Printf("Setting issue body for %s\n", n)
		_, _, err := client.Issues.Edit(context.Background(), owner, repo, n, &github.IssueRequest{Body: &newi.Form.Body})
		if err != nil {
			return err
		}
	}

	if newi.Form.Title != i.Form.Title {
		log.Printf("Setting issue title for %s\n", n)
		_, _, err := client.Issues.Edit(context.Background(), owner, repo, n, &github.IssueRequest{Title: &newi.Form.Title})
		if err != nil {
			return err
		}
	}

	if newi.Form.State != i.Form.State {
		log.Printf("Changing issue state for %s\n", n)
		_, _, err := client.Issues.Edit(context.Background(), owner, repo, n, &github.IssueRequest{State: &newi.Form.State})
		if err != nil {
			return err
		}
	}

	if !reflect.DeepEqual(newi.Form.Labels, i.Form.Labels) {
		log.Printf("Changing labels for %s\n", n)
		_, _, err := client.Issues.Edit(context.Background(), owner, repo, n, &github.IssueRequest{Labels: &newi.Form.Labels})
		if err != nil {
			return err
		}
	}

	if newi.Form.Assignee != i.Form.Assignee {
		log.Printf("Assigning issue %s\n", n)
		_, _, err = client.Issues.Edit(context.Background(), owner, repo, n, &github.IssueRequest{Assignee: &newi.Form.Assignee})
		if err != nil {
			return err
		}
	}

	for idx, c := range comments {
		comment := &Comment{}
		markform.Unmarshal(c, &comment.Form)

		// New comment
		if len(i.Comments) <= idx && len(strings.TrimSpace(comment.Form.Body)) != 0 {
			log.Printf("Creating a comment for issue %d\n", n)
			gc, _, err := client.Issues.CreateComment(context.Background(), owner, repo, n, &github.IssueComment{Body: &comment.Form.Body})
			if err != nil {
				return err
			}
			i.Comments = append(i.Comments, Comment{Comment: gc})
			i.Comments[idx].Form.Body = comment.Form.Body
		} else if i.Comments[idx].Form.Body == "\n```\n\n```\n" && len(strings.TrimSpace(comment.Form.Body)) != 0 {
			log.Printf("Creating a comment for issue %d\n", n)
			gc, _, err := client.Issues.CreateComment(context.Background(), owner, repo, n, &github.IssueComment{Body: &comment.Form.Body})
			if err != nil {
				return err
			}
			i.Comments[idx].Comment = gc
			i.Comments[idx].Form.Body = comment.Form.Body
			// Edit existing comment
		} else if i.Comments[idx].Form.Body != comment.Form.Body && i.Comments[idx].Form.Body != "\n```\n\n```\n" {
			log.Printf("Editing comment for issue %d\n", n)
			_, _, err := client.Issues.EditComment(context.Background(), owner, repo, *i.Comments[idx].Comment.ID, &github.IssueComment{Body: &comment.Form.Body})
			if err != nil {
				return err
			}
			i.Comments[idx].Form.Body = comment.Form.Body
		}
	}

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

	list := struct {
		Issues         []*github.Issue
		NewIssueNumber int
	}{}
	list.Issues = []*github.Issue{}

	repo := path.Base(path.Dir(path.Dir(name)))
	owner := path.Base(path.Dir(path.Dir(path.Dir(name))))

	ilh.ih.mutex.Lock()
	defer ilh.ih.mutex.Unlock()

	ilh.ih.options.ListOptions = github.ListOptions{PerPage: 10}

	for {
		log.Printf("Listing issues for repo %s\n", repo)
		i, resp, err := uncachedClient.Issues.ListByRepo(context.Background(), owner, repo, ilh.ih.options)
		if err != nil {
			return err
		}

		for _, issue := range i {
			list.Issues = append(list.Issues, issue)
			if list.NewIssueNumber < *issue.Number {
				list.NewIssueNumber = *issue.Number
			}
		}

		if resp.NextPage == 0 {
			break
		}

		ilh.ih.options.Page = resp.NextPage
	}

	for {
		list.NewIssueNumber++
		log.Printf("Finding new issue number for repo %s\n", repo)
		_, _, err := uncachedClient.Issues.Get(context.Background(), owner, repo, list.NewIssueNumber)
		if err != nil {
			break
		}
	}

	buf := bytes.Buffer{}
	err := issuesListMarkdown.Execute(&buf, list)
	if err != nil {
		return err
	}

	ilh.StaticFileHandler.Content = buf.Bytes()

	return ilh.StaticFileHandler.Open(name, fid, mode)
}
