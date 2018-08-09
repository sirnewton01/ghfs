package main

import (
	"context"
	"flag"
	"github.com/google/go-github/github"
	"github.com/sirnewton01/ghfs/dynamic"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net"
	"strings"
)

var (
	client  *github.Client
	funcMap = map[string]interface{}{"markdown": markdown}

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

	s, d, err := dynamic.NewServer(
		[]dynamic.FileEntry{
			dynamic.NewFileEntry("/README.md", &dynamic.StaticFileHandler{[]byte(`
# GitHub File System

Welcome to a file system view of GitHub. Using the site is easy once you learn a few tricks. Since GitHub is a very large site parts of the system are hidden and load on-demand. In particular, the repos directory is empty until you attempt to access something inside. You can "cd _ghfsdir_/repos/sirnewton01" or even "cd _ghfsdir_/repos/sirnewton01/ghfs". From there you will see start to see parts of the filesystem fill in.

Files are rendered in Markdown or even simple text so that you can interact with it using simple text editors.

For each repo, the issues are shown under "_ghfs_/repos/_owner_/_repo_/issues".

Happy browsing!
`)}),
		})

	d.AddFileEntry("/repos", &ReposHandler{&dynamic.BasicDirHandler{d}})

	if err := s.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
