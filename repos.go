package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"path"
	"strings"
	"sync"
	"text/template"

	"github.com/Harvey-OS/ninep/protocol"
	"github.com/google/go-github/github"
	"github.com/russross/blackfriday/v2"
	"github.com/sirnewton01/ghfs/dynamic"
	"github.com/sirnewton01/ghfs/markform"
)

var (
	repoMarkdown = template.Must(template.New("repository").Funcs(funcMap).Parse(
		`# {{ .Repository.FullName }} {{ if .Repository.GetFork }}[{{ .Repository.GetSource.FullName }}](../../{{ .Repository.GetSource.Owner.Login }}/{{ .Repository.GetSource.Name }}/repo.md){{ end }}

* {{ markform .Form "Description" }}
* {{ markform .Form "Starred" }}
* {{ markform .Form "Notifications" }}
* Created: {{ .Repository.CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
* Watchers: {{ .Repository.WatchersCount }}
* Stars: {{ .Repository.StargazersCount }}
* Forks: {{ .Repository.ForksCount }}
* Default branch: {{ .Repository.DefaultBranch }}
* Pushed: {{ .Repository.PushedAt.Format "2006-01-02T15:04:05Z07:00" }}
* Commit: {{ .Branch.GetCommit.SHA }} {{ .Branch.GetCommit.Commit.Author.Date.Format "2006-01-02T15:04:05Z07:00" }}

    git clone {{ .Repository.CloneURL }}
`))

	userMarkdown = template.Must(template.New("user").Funcs(funcMap).Parse(
		`# {{ .User.Name }} - {{ .User.Login }}

* Location: {{ .User.Location }}
* Email: {{ .User.Email }}

{{ .User.Bio }}

* Created: {{ .User.CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
* Updated: {{ .User.UpdatedAt.Format "2006-01-02T15:04:05Z07:00" }}
* Followers: {{ .User.Followers }}
* {{ markform .Form "Follow" }}
`))

	orgMarkdown = template.Must(template.New("org").Funcs(funcMap).Parse(
		`# {{ .Name }} - {{ .Login }}

* Location: {{ .Location }}
* Email: {{ .Email }}

{{ .Description }}

* Created: {{ .CreatedAt.Format "2006-01-02T15:04:05Z07:00" }}
* Updated: {{ .UpdatedAt.Format "2006-01-02T15:04:05Z07:00" }}
* Followers: {{ .Followers }}
`))

	starMarkdown = template.Must(template.New("star").Funcs(funcMap).Parse(
		`# Starred repositories

{{ range . }}  * repos/{{ .Repository.Owner.Login }}/{{ .Repository.Name }}
{{ end }}
`))
)

type repoMarkdownForm struct {
	Description string ` = ___`
}

type repoMarkdownModel struct {
	Form   repoMarkdownForm
	Rest   *github.Repository
	Branch *github.Branch
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
			log.Printf("Listing following for %s\n", currentUser)
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
	log.Printf("Checking whether owner %s is an organization\n", owner)
	org, _, err := client.Organizations.Get(context.Background(), owner)
	if err != nil {
		// It could be a user
		log.Printf("Checking whether owner %s is a user\n", owner)
		user, _, err := client.Users.Get(context.Background(), owner)
		if err != nil {
			return -1, err
		}
		NewUserHandler(*user.Login)
		return idx, nil
	}
	NewOrgHandler(*org.Login)
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
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		log.Printf("Listing repositories owned by %s\n", owner)
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
			repoPath := path.Join("/repos", owner, *repo.Name)
			NewRepoOverviewHandler(repoPath)
			NewIssuesHandler(repoPath)
			NewRepoReadmeHandler(repoPath)
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

// UserHandler handles the displaying and updating of the
//  0user.md for a user.
type UserHandler struct {
	User *github.User
	Form struct {
		Follow bool ` = []`
	}

	readbuf  *bytes.Buffer
	writefid protocol.FID
	writebuf *bytes.Buffer
	mu       sync.Mutex
}

func NewUserHandler(name string) {
	server.AddFileEntry(path.Join("/repos", name, "0user.md"), &UserHandler{readbuf: &bytes.Buffer{}})
}

func (uh *UserHandler) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("No children of the 0user.md file")
}

func (uh *UserHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	username := path.Base(path.Dir(name))

	log.Printf("Reading user %s\n", username)
	u, _, err := client.Users.Get(context.Background(), username)
	if err != nil {
		return err
	}

	following, _, err := client.Users.IsFollowing(context.Background(), "", username)
	if err != nil {
		return err
	}

	uh.mu.Lock()
	defer uh.mu.Unlock()

	uh.User = u
	uh.Form.Follow = following

	if mode == protocol.OREAD {
		buf := bytes.Buffer{}
		err = userMarkdown.Execute(&buf, uh)
		if err != nil {
			return err
		}
		uh.readbuf = &buf
	}

	if mode&protocol.ORDWR != 0 || mode&protocol.OWRITE != 0 {
		if uh.writefid != 0 {
			return fmt.Errorf("User metadata doesn't support concurrent writes")
		}

		uh.writefid = fid
		uh.writebuf = &bytes.Buffer{}
	}

	return nil
}

func (uh *UserHandler) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	uh.mu.Lock()
	defer uh.mu.Unlock()

	if fid != uh.writefid {
		return int64(len(buf)), nil
	}

	// TODO consider offset
	length, err := uh.writebuf.Write(buf)
	if err != nil {
		return int64(length), err
	}

	return int64(length), nil
}

func (uh *UserHandler) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	uh.mu.Lock()
	defer uh.mu.Unlock()

	if offset >= int64(uh.readbuf.Len()) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(uh.readbuf.Len()) {
		return uh.readbuf.Bytes()[offset:], nil
	}

	return uh.readbuf.Bytes()[offset : offset+count], nil
}

func (uh *UserHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating a child of a 0user.md is not supported")
}

func (uh *UserHandler) Stat(name string) (protocol.Dir, error) {
	uh.mu.Lock()
	defer uh.mu.Unlock()

	// There's only one version and it is always a file
	return protocol.Dir{QID: protocol.QID{Version: 0, Type: protocol.QTFILE}, Length: uint64(uh.readbuf.Len())}, nil
}

func (uh *UserHandler) Wstat(name string, dir protocol.Dir) error {
	uh.mu.Lock()
	defer uh.mu.Unlock()

	uh.writebuf.Truncate(int(dir.Length))
	return nil
}

func (uh *UserHandler) Remove(name string) error {
	return fmt.Errorf("Removing 0user.md isn't supported.")
}

func (uh *UserHandler) Clunk(name string, fid protocol.FID) error {
	username := path.Base(path.Dir(name))

	uh.mu.Lock()
	defer uh.mu.Unlock()

	if fid != uh.writefid {
		return nil
	}
	uh.writefid = 0

	// No bytes were written this time, leave it alone
	if len(uh.writebuf.Bytes()) == 0 {
		return nil
	}

	newuh := &UserHandler{}
	md := blackfriday.New()
	tree := md.Parse(uh.writebuf.Bytes())
	err := markform.Unmarshal(tree, &newuh.Form)
	if err != nil {
		return err
	}

	if newuh.Form.Follow != uh.Form.Follow {
		if newuh.Form.Follow {
			log.Printf("Following %s\n", username)
			_, err := client.Users.Follow(context.Background(), username)
			if err != nil {
				return err
			}
		} else {
			log.Printf("Unfollowing %s\n", username)
			_, err := client.Users.Unfollow(context.Background(), username)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func NewOrgHandler(name string) {
	server.AddFileEntry(path.Join("/repos", name, "0org.md"), &OrgHandler{StaticFileHandler: dynamic.StaticFileHandler{[]byte{}}})
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
	Repository *github.Repository
	Branch     *github.Branch
	Form       struct {
		Description   string ` = ___`
		Starred       bool   ` = []`
		Notifications string ` = () not watching () watching () ignoring`
	}

	readbuf  *bytes.Buffer
	writefid protocol.FID
	writebuf *bytes.Buffer
	mu       sync.Mutex
}

func NewRepoOverviewHandler(repoPath string) {
	server.AddFileEntry(path.Join(repoPath, "repo.md"), &RepoOverviewHandler{readbuf: &bytes.Buffer{}})
}

func (roh *RepoOverviewHandler) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("No children of the repo.md file")
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

	b, _, err := client.Repositories.GetBranch(context.Background(), owner, repo, *r.DefaultBranch)
	if err != nil {
		return err
	}

	s, _, err := client.Activity.IsStarred(context.Background(), owner, repo)
	if err != nil {
		return err
	}

	subs, _, err := client.Activity.GetRepositorySubscription(context.Background(), owner, repo)
	if err != nil {
		return err
	}

	roh.Repository = r
	roh.Branch = b

	if r.Description != nil {
		roh.Form.Description = *r.Description
	}
	roh.Form.Starred = s
	if subs == nil || (!*subs.Subscribed && !*subs.Ignored) {
		roh.Form.Notifications = "not watching"
	} else if *subs.Subscribed {
		roh.Form.Notifications = "watching"
	} else if *subs.Ignored {
		roh.Form.Notifications = "ignoring"
	}

	if mode == protocol.OREAD {
		buf := bytes.Buffer{}
		err = repoMarkdown.Execute(&buf, roh)
		if err != nil {
			return err
		}
		roh.readbuf = &buf
	}

	if mode&protocol.ORDWR != 0 || mode&protocol.OWRITE != 0 {
		if roh.writefid != 0 {
			return fmt.Errorf("Repo metadata doesn't support concurrent writes")
		}

		roh.writefid = fid
		roh.writebuf = &bytes.Buffer{}
	}

	return nil
}

func (roh *RepoOverviewHandler) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	roh.mu.Lock()
	defer roh.mu.Unlock()

	if fid != roh.writefid {
		return int64(len(buf)), nil
	}

	// TODO consider offset
	length, err := roh.writebuf.Write(buf)
	if err != nil {
		return int64(length), err
	}

	return int64(length), nil
}

func (roh *RepoOverviewHandler) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	roh.mu.Lock()
	defer roh.mu.Unlock()

	if offset >= int64(roh.readbuf.Len()) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(roh.readbuf.Len()) {
		return roh.readbuf.Bytes()[offset:], nil
	}

	return roh.readbuf.Bytes()[offset : offset+count], nil
}

func (roh *RepoOverviewHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creating a child of a repo.md is not supported")
}

func (roh *RepoOverviewHandler) Stat(name string) (protocol.Dir, error) {
	roh.mu.Lock()
	defer roh.mu.Unlock()

	// There's only one version and it is always a file
	return protocol.Dir{QID: protocol.QID{Version: 0, Type: protocol.QTFILE}, Length: uint64(roh.readbuf.Len())}, nil
}

func (roh *RepoOverviewHandler) Wstat(name string, dir protocol.Dir) error {
	roh.mu.Lock()
	defer roh.mu.Unlock()

	roh.writebuf.Truncate(int(dir.Length))
	return nil
}

func (roh *RepoOverviewHandler) Remove(name string) error {
	return fmt.Errorf("Removing repo.md isn't supported.")
}

func (roh *RepoOverviewHandler) Clunk(name string, fid protocol.FID) error {
	owner := path.Base(path.Dir(path.Dir(name)))
	repo := path.Base(path.Dir(name))

	roh.mu.Lock()
	defer roh.mu.Unlock()

	if fid != roh.writefid {
		return nil
	}
	roh.writefid = 0

	// No bytes were written this time, leave it alone
	if len(roh.writebuf.Bytes()) == 0 {
		return nil
	}

	newroh := &RepoOverviewHandler{}
	md := blackfriday.New()
	tree := md.Parse(roh.writebuf.Bytes())
	err := markform.Unmarshal(tree, &newroh.Form)
	if err != nil {
		return err
	}

	if newroh.Form.Description != roh.Form.Description {
		roh.Repository.Description = &newroh.Form.Description
		log.Printf("Setting repository description for %s\n", repo)
		_, _, err := client.Repositories.Edit(context.Background(), owner, repo, roh.Repository)
		if err != nil {
			return err
		}
	}

	if newroh.Form.Starred != roh.Form.Starred {
		if newroh.Form.Starred {
			log.Printf("Starring repository %s\n", repo)
			_, err := client.Activity.Star(context.Background(), owner, repo)
			if err != nil {
				return err
			}
		} else {
			log.Printf("Unstarring repository %s\n", repo)
			_, err := client.Activity.Unstar(context.Background(), owner, repo)
			if err != nil {
				return err
			}
		}
	}

	subs := &github.Subscription{}
	f := false
	t := true

	if newroh.Form.Notifications != roh.Form.Notifications {
		log.Printf("Changing repository subscription for %s\n", repo)
		if newroh.Form.Notifications == "not watching" {
			subs.Subscribed = &f
			subs.Ignored = &f
			_, _, err := client.Activity.SetRepositorySubscription(context.Background(), owner, repo, subs)
			if err != nil {
				return err
			}

			_, err = client.Activity.DeleteRepositorySubscription(context.Background(), owner, repo)
			if err != nil {
				return err
			}
		} else if newroh.Form.Notifications == "watching" {
			subs.Subscribed = &t
			subs.Ignored = &f
			_, _, err := client.Activity.SetRepositorySubscription(context.Background(), owner, repo, subs)
			if err != nil {
				return err
			}
		} else if newroh.Form.Notifications == "ignoring" {
			subs.Subscribed = &f
			subs.Ignored = &t
			_, _, err := client.Activity.SetRepositorySubscription(context.Background(), owner, repo, subs)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type RepoReadmeHandler struct {
	dynamic.StaticFileHandler
	mu sync.Mutex
}

func NewRepoReadmeHandler(repoPath string) {
	server.AddFileEntry(path.Join(repoPath, "README.md"), &RepoReadmeHandler{StaticFileHandler: dynamic.StaticFileHandler{[]byte{}}})
}

func (rrh *RepoReadmeHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	owner := path.Base(path.Dir(path.Dir(name)))
	repo := path.Base(path.Dir(name))

	rrh.mu.Lock()
	defer rrh.mu.Unlock()

	log.Printf("Getting project readme for %s\n", repo)
	readme, _, err := client.Repositories.GetReadme(context.Background(), owner, repo, nil)
	if err != nil {
		return err
	}

	c, err := readme.GetContent()
	if err != nil {
		return err
	}

	rrh.StaticFileHandler.Content = []byte(c)

	return rrh.StaticFileHandler.Open(name, fid, mode)
}

type StarredReposHandler struct {
	dynamic.StaticFileHandler
	mu sync.Mutex
}

func NewStarredReposHandler() {
	server.AddFileEntry(path.Join("/stars.md"), &StarredReposHandler{StaticFileHandler: dynamic.StaticFileHandler{[]byte{}}})
}

func (srh *StarredReposHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	srh.mu.Lock()
	defer srh.mu.Unlock()

	log.Printf("Retrieving the current user's starred repositories\n")
	stars, _, err := client.Activity.ListStarred(context.Background(), currentUser, nil)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	err = starMarkdown.Execute(&buf, stars)
	if err != nil {
		return err
	}

	srh.StaticFileHandler.Content = buf.Bytes()

	return srh.StaticFileHandler.Open(name, fid, mode)
}
