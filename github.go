package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/sirnewton01/ghfs/dynamic"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net"
	"path"
	"strings"
	"text/template"
)

var (
	funcMap = map[string]interface{}{"markdown": markdown}

	issueMarkdown = template.Must(template.New("issue").Funcs(funcMap).Parse(
		`# {{ .Title  }} (#{{ .Number }})

State: {{ .State }} - [{{ .User.Login }}](../../../{{ .User.Login }}) opened this issue {{ .CreatedAt }} - {{ .Comments }} comments

Assignee: {{if .Assignee}} [{{ .Assignee.Login }}](../../../{{ .Assignee.Login }}) {{else}} <Not Assigned> {{end}}

{{ markdown .Body }}

`))

	commentMarkdown = template.Must(template.New("comment").Funcs(funcMap).Parse(
		`## [{{ .User.Login }}](../../../{{ .User.Login }}) commented {{ .CreatedAt }} ({{ .AuthorAssociation }})

{{ markdown .Body }}

`))

	repoMarkdown = template.Must(template.New("repository").Funcs(funcMap).Parse(
		`# {{ .FullName }} {{ if .GetFork }} [Forked] {{ end }}

{{ .Description }}

Created: {{ .CreatedAt }}
Pushed: {{ .PushedAt }}

Watchers: {{ .WatchersCount }}
Stars: {{ .StargazersCount }}
Forks: {{ .ForksCount }}

Default branch: {{ .DefaultBranch }}

git clone {{ .CloneURL }}
`))

	ntype    = flag.String("ntype", "tcp4", "Default network type")
	naddr    = flag.String("addr", ":5640", "Network address")
	apitoken = flag.String("apitoken", "", "Personal API Token for authentication")
	lognet   = flag.Bool("lognet", false, "Log network requests")
)

func markdown(content string) string {
	return "    " + strings.Replace(content, "\n", "\n    ", -1)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flag.Parse()

	var client *github.Client
	if *apitoken != "" {
		log.Printf("Using Personal API Token for authentication.\n")
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *apitoken},
		)

		tc := oauth2.NewClient(context.Background(), ts)
		client = github.NewClient(tc)
	} else {
		log.Printf("Using no authentication. Note that rate limits will apply.\n")
		client = github.NewClient(nil)
	}

	if !*lognet {
		log.SetOutput(ioutil.Discard)
	}

	ln, err := net.Listen(*ntype, *naddr)
	if err != nil {
		return
	}

	issueHandler := func(s *dynamic.Server, name string) error {
		if s.HasChildren(name) {
			return nil
		}

		repo := path.Base(path.Dir(name))
		owner := path.Base(path.Dir(path.Dir(name)))

		log.Printf("Listing issues for repo %v/%v\n", owner, repo)

		options := github.IssueListByRepoOptions{
			ListOptions: github.ListOptions{PerPage: 10},
		}

		for {
			issues, resp, err := client.Issues.ListByRepo(context.Background(), owner, repo, &options)
			if err != nil {
				return err
			}

			for _, issue := range issues {
				childPath := path.Join(name, fmt.Sprintf("%d.md", *issue.Number))
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

				s.AddFileEntry(dynamic.NewFileEntry(childPath, &dynamic.StaticFileHandler{buf.Bytes()}))
			}

			if resp.NextPage == 0 {
				break
			}

			options.Page = resp.NextPage
		}

		return nil
	}

	reposHandler := func(s *dynamic.Server, name string, child string) (dynamic.FileHandler, error) {
		log.Printf("Listing repository for owner %v\n", child)

		options := github.RepositoryListOptions{
			ListOptions: github.ListOptions{PerPage: 10},
		}

		for {
			repos, resp, err := client.Repositories.List(context.Background(), child, &options)
			if err != nil {
				return nil, err
			}

			if len(repos) == 0 {
				return nil, nil
			}

			for _, repo := range repos {
				s.AddFileEntry(dynamic.NewFileEntry(path.Join(name, child, *repo.Name), dynamic.BasicDirHandler{}))
				buf := bytes.Buffer{}
				err = repoMarkdown.Execute(&buf, repo)
                                if err != nil {
                                        return nil, err
                                }
				s.AddFileEntry(dynamic.NewFileEntry(path.Join(name, child, *repo.Name, "README.md"), &dynamic.StaticFileHandler{buf.Bytes()}))
				s.AddFileEntry(dynamic.NewFileEntry(path.Join(name, child, *repo.Name, "issues"), dynamic.BasicDirHandler{nil, issueHandler}))
			}

			if resp.NextPage == 0 {
				break
			}
			options.Page = resp.NextPage
		}

		return &dynamic.BasicDirHandler{}, nil
	}

	s, err := dynamic.NewServer(
		[]dynamic.FileEntry{
			dynamic.NewFileEntry("/README.md", &dynamic.StaticFileHandler{[]byte(`
# GitHub File System

Welcome to a file system view of GitHub. Using the site is easy once you learn a few tricks. Since GitHub is a very large site parts of the system are hidden and load on-demand. In particular, the repos directory is empty until you attempt to access something inside. You can "cd <ghfsdir>/repos/sirnewton01" or even "cd <ghfsdir>/repos/sirnewton01/ghfs". From there you will see start to see parts of the filesystem fill in.

Files are rendered in Markdown or even simple text so that you can interact with it using simple text editors.

For each repo, the issues are shown under "<ghfs>/repose/<owner>/<repo>/issues".

Happy browsing!
`)}),
			dynamic.NewFileEntry("/repos", &dynamic.BasicDirHandler{reposHandler, nil}),
		})

	if err := s.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
