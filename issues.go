package main

import (
	"bytes"
	"context"
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

{{ markdown .Body }}

`))

	commentMarkdown = template.Must(template.New("comment").Funcs(funcMap).Parse(
		`## [{{ .User.Login }}](../../../{{ .User.Login }}) commented {{ .CreatedAt }} ({{ .AuthorAssociation }})

{{ markdown .Body }}

`))
)

type IssuesHandler struct {
	handler *dynamic.BasicDirHandler
	options github.IssueListByRepoOptions
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

		issue, _, err := client.Issues.Get(context.Background(), owner, repo, number)
		if err != nil {
			return idx, err
		}

		err = ih.addIssue(owner, repo, issue)
		if err != nil {
			return idx, err
		}
	}

	return ih.handler.WalkChild(name, child)
}

func (ih *IssuesHandler) addIssue(owner string, repo string, issue *github.Issue) error {
	// TODO pass this responsibility to an issue handler that can refresh itself on read
	childPath := path.Join("/repos", owner, repo, "issues", fmt.Sprintf("%d.md", *issue.Number))
	buf := bytes.Buffer{}
	err := issueMarkdown.Execute(&buf, *issue)
	if err != nil {
		return err
	}
	comments, _, err := client.Issues.ListComments(context.Background(), owner, repo, *issue.Number, nil)
	for _, comment := range comments {
		bb := bytes.Buffer{}
		err := commentMarkdown.Execute(&bb, *comment)
		if err != nil {
			return err
		}
		buf.Write(bb.Bytes())
	}

	ih.handler.S.AddFileEntry(childPath, &dynamic.StaticFileHandler{buf.Bytes()})
	return nil
}

func (ih *IssuesHandler) refresh(owner string, repo string) error {
	log.Printf("Listing issues for repo %v/%v\n", owner, repo)

	ih.options.ListOptions = github.ListOptions{PerPage: 1}

	for {
		issues, resp, err := client.Issues.ListByRepo(context.Background(), owner, repo, &ih.options)
		if err != nil {
			return err
		}

		for _, issue := range issues {
			err = ih.addIssue(owner, repo, issue)
			if err != nil {
				return err
			}
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
