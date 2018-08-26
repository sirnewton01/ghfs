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
	"sync"
	"text/template"
)

var (
	repoMarkdown = template.Must(template.New("repository").Funcs(funcMap).Parse(
		`# {{ .FullName }} {{ if .GetFork }} [Forked] {{ end }}

{{ .Description }}

Created: {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
Pushed: {{ .PushedAt.Format "2006-01-02T15:04:05Z07:00" }}

Watchers: {{ .WatchersCount }}
Stars: {{ .StargazersCount }}
Forks: {{ .ForksCount }}

Default branch: {{ .DefaultBranch }}

git clone {{ .CloneURL }}
`))
)

// ReposHandler handles the repos directory dynamically loading
//  owners as they are looked up so that they show up in directory
//  listings afterwards. If the connection is authenticated then
//  the authenticated user shows up right away.
type ReposHandler struct {
	dirhandler *dynamic.BasicDirHandler
}

func (rh *ReposHandler) WalkChild(name string, child string) (int, error) {
	idx, err := rh.dirhandler.WalkChild(name, child)

	if idx == -1 {
		log.Printf("Checking if owner %v exists\n", child)

		options := github.RepositoryListOptions{
			ListOptions: github.ListOptions{PerPage: 1},
		}

		_, _, err := client.Repositories.List(context.Background(), child, &options)
		if err != nil {
			return -1, err
		}

		return rh.dirhandler.S.AddFileEntry(path.Join(name, child), &OwnerHandler{&dynamic.BasicDirHandler{rh.dirhandler.S}}), nil
	}

	return idx, err
}

func (rh *ReposHandler) Open(name string, mode protocol.Mode) error {
	return rh.dirhandler.Open(name, mode)
}

func (rh *ReposHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating organizations or users is not supported.")
}

func (rh *ReposHandler) Stat(name string) (protocol.QID, error) {
	return rh.dirhandler.Stat(name)
}

func (rh *ReposHandler) Length(name string) (uint64, error) {
	return rh.dirhandler.Length(name)
}

func (rh *ReposHandler) Wstat(name string, qid protocol.QID, length uint64) error {
	return fmt.Errorf("Unsupported operation")
}

func (rh *ReposHandler) Remove(name string) error {
	return fmt.Errorf("The repos folder cannot be removed")
}

func (rh *ReposHandler) Read(name string, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 && currentUser != "" {
		rh.dirhandler.S.AddFileEntry(path.Join(name, currentUser), &OwnerHandler{&dynamic.BasicDirHandler{rh.dirhandler.S}})

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
				rh.dirhandler.S.AddFileEntry(path.Join(name, *user.Login), &OwnerHandler{&dynamic.BasicDirHandler{rh.dirhandler.S}})
			}

			if resp.NextPage == 0 {
				break
			}
			options.Page = resp.NextPage
		}

	}
	return rh.dirhandler.Read(name, offset, count)
}

func (rh *ReposHandler) Write(name string, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Creating a new user or organization is not supported.")
}

// OwnerHandler handles a owner within the repos directory listing
//  out all of their repositories.
// TODO add a README.md describing the owner themselves
type OwnerHandler struct {
	dirhandler *dynamic.BasicDirHandler
}

func (oh *OwnerHandler) WalkChild(name string, child string) (int, error) {
	idx, err := oh.dirhandler.WalkChild(name, child)

	if idx == -1 {
		owner := path.Base(name)
		err = oh.refresh(owner)
		if err != nil {
			return -1, err
		}
	}

	return oh.dirhandler.WalkChild(name, child)
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
			oh.dirhandler.S.AddFileEntry(path.Join("/repos", owner, *repo.Name), &dynamic.BasicDirHandler{oh.dirhandler.S})
			oh.dirhandler.S.AddFileEntry(path.Join("/repos", owner, *repo.Name, "repo.md"), &RepoOverviewHandler{filehandler: &dynamic.StaticFileHandler{[]byte{}}})
			NewIssuesHandler(oh.dirhandler.S, path.Join("/repos", owner, *repo.Name))
		}

		if resp.NextPage == 0 {
			break
		}
		options.Page = resp.NextPage
	}

	return nil
}

func (oh *OwnerHandler) Open(name string, mode protocol.Mode) error {
	return oh.dirhandler.Open(name, mode)
}

func (oh *OwnerHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating repos is not supported.")
}

func (oh *OwnerHandler) Stat(name string) (protocol.QID, error) {
	return oh.dirhandler.Stat(name)
}

func (oh *OwnerHandler) Length(name string) (uint64, error) {
	return oh.dirhandler.Length(name)
}

func (oh *OwnerHandler) Wstat(name string, qid protocol.QID, length uint64) error {
	return fmt.Errorf("Unsupported operation")
}

func (oh *OwnerHandler) Remove(name string) error {
	return fmt.Errorf("An owner cannot be removed.")
}

func (oh *OwnerHandler) Read(name string, offset int64, count int64) ([]byte, error) {
	if offset == 0 && count > 0 {
		err := oh.refresh(path.Base(name))
		if err != nil {
			return []byte{}, err
		}
	}

	return oh.dirhandler.Read(name, offset, count)
}

func (oh *OwnerHandler) Write(name string, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Creating repos is not supported.")
}

// RepoOverviewHandler handles the displaying and updating of the
//  README.md for a repo.
type RepoOverviewHandler struct {
	filehandler *dynamic.StaticFileHandler
	mu          sync.Mutex
}

func (roh *RepoOverviewHandler) WalkChild(name string, child string) (int, error) {
	return roh.filehandler.WalkChild(name, child)
}

func (roh *RepoOverviewHandler) refresh(owner string, repo string) error {
	log.Printf("Reading repository %s/%s\n", owner, repo)

	roh.mu.Lock()
	defer roh.mu.Unlock()

	r, _, err := client.Repositories.Get(context.Background(), owner, repo)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	err = repoMarkdown.Execute(&buf, r)
	if err != nil {
		return err
	}

	roh.filehandler.Content = buf.Bytes()
	return nil
}

func (roh *RepoOverviewHandler) Open(name string, mode protocol.Mode) error {
	err := roh.refresh(path.Base(path.Dir(path.Dir(name))), path.Base(path.Dir(name)))
	if err != nil {
		return err
	}

	return roh.filehandler.Open(name, mode)
}

func (roh *RepoOverviewHandler) CreateChild(name string, child string) (int, error) {
	return roh.filehandler.CreateChild(name, child)
}

func (roh *RepoOverviewHandler) Stat(name string) (protocol.QID, error) {
	return roh.filehandler.Stat(name)
}

func (roh *RepoOverviewHandler) Length(name string) (uint64, error) {
	return roh.filehandler.Length(name)
}

func (roh *RepoOverviewHandler) Wstat(name string, qid protocol.QID, length uint64) error {
	return fmt.Errorf("Unsupported operation")
}

func (roh *RepoOverviewHandler) Remove(name string) error {
	return fmt.Errorf("A repo cannot be removed.")
}

func (roh *RepoOverviewHandler) Read(name string, offset int64, count int64) ([]byte, error) {
	return roh.filehandler.Read(name, offset, count)
}

func (roh *RepoOverviewHandler) Write(name string, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Modifying repos is not supported.")
}
