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
	"strings"
	"sync"
	"text/template"
)

var (
	repoMarkdown = template.Must(template.New("repository").Funcs(funcMap).Parse(
		`# {{ .Rest.FullName }} {{ if .Rest.GetFork }}[{{ .Rest.GetSource.FullName }}](../../{{ .Rest.GetSource.Owner.Login }}/{{ .Rest.GetSource.Name }}/repo.md){{ end }}

{{ markform .Form "Description" }}

Created: {{ .Rest.CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
Pushed: {{ .Rest.PushedAt.Format "2006-01-02T15:04:05Z07:00" }}

Watchers: {{ .Rest.WatchersCount }}
Stars: {{ .Rest.StargazersCount }}
Forks: {{ .Rest.ForksCount }}

Default branch: {{ .Rest.DefaultBranch }}

git clone {{ .Rest.CloneURL }}
`))

	userMarkdown = template.Must(template.New("user").Funcs(funcMap).Parse(
		`# {{ .Name }} - {{ .Login }}

Location: {{ .Location }}
Email: {{ .Email }}

{{ .Bio }}

Created: {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
Updated: {{ .UpdatedAt.Format "2006-01-02T15:04:05Z07:00" }}

Followers: {{ .Followers }}
`))

	orgMarkdown = template.Must(template.New("org").Funcs(funcMap).Parse(
		`# {{ .Name }} - {{ .Login }}

Location: {{ .Location }}
Email: {{ .Email }}

{{ .Description }}

Created: {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
Updated: {{ .UpdatedAt.Format "2006-01-02T15:04:05Z07:00" }}

Followers: {{ .Followers }}
`))
)

type repoMarkdownForm struct {
	Description string ` = ___`
}

type repoMarkdownModel struct {
	Form repoMarkdownForm
	Rest *github.Repository
}

// ReposHandler handles the repos directory dynamically loading
//  owners as they are looked up so that they show up in directory
//  listings afterwards. If the connection is authenticated then
//  the authenticated user shows up right away.
type ReposHandler struct {
	dynamic.BasicDirHandler
}

func (rh *ReposHandler) WalkChild(name string, child string) (int, error) {
	idx, err := rh.BasicDirHandler.WalkChild(name, child)

	if idx == -1 {
		log.Printf("Checking if owner %v exists\n", child)

		idx, err = NewOwnerHandler(child)
		if idx == -1 {
			return -1, fmt.Errorf("Child not found: %s", child)
		}
	}

	return idx, err
}

func (rh *ReposHandler) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 && currentUser != "" {
		_, err := NewOwnerHandler(currentUser)
		if err != nil {
			return []byte{}, err
		}

		options := github.ListOptions{PerPage: 10}
		// Add following
		for {
			users, resp, err := client.Users.ListFollowing(context.Background(), currentUser, &options)

			if err != nil {
				return []byte{}, err
			}

			if len(users) == 0 {
				break
			}

			for _, user := range users {
				log.Printf("Adding following %v\n", *user.Login)
				_, err = NewOwnerHandler(*user.Login)
				if err != nil {
					return []byte{}, err
				}
			}

			if resp.NextPage == 0 {
				break
			}
			options.Page = resp.NextPage
		}

	}
	return rh.BasicDirHandler.Read(name, fid, offset, count)
}

func (rh *ReposHandler) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Creating a new user or organization is not supported.")
}

func NewOwnerHandler(owner string) (int, error) {
	// Skip hidden files as they are not owners on GitHub
	if strings.HasPrefix(owner, ".") {
		return -1, nil
	}

	idx := server.AddFileEntry(path.Join("/repos", owner), &OwnerHandler{dynamic.BasicDirHandler{server, nil}})

	// Check if it is an organization
	org, _, err := client.Organizations.Get(context.Background(), owner)
	if err != nil {
		// It could be a user
		user, _, err := client.Users.Get(context.Background(), owner)
		if err != nil {
			return -1, err
		}
		NewUserHandler(*user.Login)
		return idx, nil
	}
	NewOrgHandler(*org.Login)
	//server.AddFileEntry(path.Join("/repos", owner, "org.md"), NewOrgHandler(org))
	return idx, nil
}

// OwnerHandler handles a owner within the repos directory listing
//  out all of their repositories.
type OwnerHandler struct {
	dynamic.BasicDirHandler
}

func (oh *OwnerHandler) WalkChild(name string, child string) (int, error) {
	idx, err := oh.BasicDirHandler.WalkChild(name, child)

	// No hidden files as repo names on github
	// Also, Mac probes heavily for them costing
	//  significant performance.
	if idx == -1 && strings.HasPrefix(child, ".") {
		return idx, err
	}

	if idx == -1 {
		owner := path.Base(name)
		err = oh.refresh(owner)
		if err != nil {
			return -1, err
		}
	}

	return oh.BasicDirHandler.WalkChild(name, child)
}

func (oh *OwnerHandler) refresh(owner string) error {
	log.Printf("Listing all of the repos for owner %v\n", owner)
	options := github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	}

	for {
		repos, resp, err := client.Repositories.List(context.Background(), owner, &options)
		if err != nil {
			return err
		}

		if len(repos) == 0 {
			return nil
		}

		for _, repo := range repos {
			log.Printf("Adding repo %v\n", *repo.Name)
			server.AddFileEntry(path.Join("/repos", owner, *repo.Name), &dynamic.BasicDirHandler{server, nil})
			server.AddFileEntry(path.Join("/repos", owner, *repo.Name, "repo.md"), &RepoOverviewHandler{StaticFileHandler: dynamic.StaticFileHandler{[]byte{}}})
			NewIssuesHandler(server, path.Join("/repos", owner, *repo.Name))
		}

		if resp.NextPage == 0 {
			break
		}
		options.Page = resp.NextPage
	}

	return nil
}

func (oh *OwnerHandler) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 {
		err := oh.refresh(path.Base(name))
		if err != nil {
			return []byte{}, err
		}
	}

	return oh.BasicDirHandler.Read(name, fid, offset, count)
}

func (oh *OwnerHandler) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Creating repos is not supported.")
}

func NewUserHandler(name string) {
	server.AddFileEntry(path.Join("/repos", name, "0user.md"), &UserHandler{StaticFileHandler: dynamic.StaticFileHandler{[]byte{}}})
}

// UserHandler handles the displaying and updating of the
//  0user.md for a user.
type UserHandler struct {
	dynamic.StaticFileHandler
	mu sync.Mutex
}

func (uh *UserHandler) WalkChild(name string, child string) (int, error) {
	return uh.StaticFileHandler.WalkChild(name, child)
}

func (uh *UserHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	user := path.Base(path.Dir(name))

	log.Printf("Reading user %s\n", user)

	uh.mu.Lock()
	defer uh.mu.Unlock()

	u, _, err := client.Users.Get(context.Background(), user)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	err = userMarkdown.Execute(&buf, u)
	if err != nil {
		return err
	}

	uh.StaticFileHandler.Content = buf.Bytes()

	return uh.StaticFileHandler.Open(name, fid, mode)
}

func NewOrgHandler(name string) {
	server.AddFileEntry(path.Join("/repos", name, "0org.md"), &UserHandler{StaticFileHandler: dynamic.StaticFileHandler{[]byte{}}})
}

// UserHandler handles the displaying and updating of the
//  0org.md for a user.
type OrgHandler struct {
	dynamic.StaticFileHandler
	mu sync.Mutex
}

func (oh *OrgHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	user := path.Base(path.Dir(name))

	log.Printf("Reading user %s\n", user)

	oh.mu.Lock()
	defer oh.mu.Unlock()

	u, _, err := client.Users.Get(context.Background(), user)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	err = userMarkdown.Execute(&buf, u)
	if err != nil {
		return err
	}

	oh.StaticFileHandler.Content = buf.Bytes()

	return oh.StaticFileHandler.Open(name, fid, mode)
}

// RepoOverviewHandler handles the displaying and updating of the
//  repo.md for a repo.
type RepoOverviewHandler struct {
	dynamic.StaticFileHandler
	mu sync.Mutex
}

func (roh *RepoOverviewHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	owner := path.Base(path.Dir(path.Dir(name)))
	repo := path.Base(path.Dir(name))

	log.Printf("Reading repository %s/%s\n", owner, repo)

	roh.mu.Lock()
	defer roh.mu.Unlock()

	r, _, err := client.Repositories.Get(context.Background(), owner, repo)
	if err != nil {
		return err
	}

	model := repoMarkdownModel{Rest: r}
	if r.Description != nil {
		model.Form.Description = *r.Description
	}

	buf := bytes.Buffer{}
	err = repoMarkdown.Execute(&buf, model)
	if err != nil {
		return err
	}

	roh.StaticFileHandler.Content = buf.Bytes()

	return roh.StaticFileHandler.Open(name, fid, mode)
}

func (roh *RepoOverviewHandler) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	/*ic.mutex.Lock()
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

	  return int64(length), err*/
	return 0, fmt.Errorf("Modifying repos is not supported.")
}
