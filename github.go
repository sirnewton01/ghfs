package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/gregjones/httpcache"
	"github.com/sirnewton01/ghfs/dynamic"
	"github.com/sirnewton01/ghfs/markform"
	"golang.org/x/oauth2"
)

var (
	client         *github.Client
	uncachedClient *github.Client
	funcMap        = map[string]interface{}{"markdown": markdown, "markform": markform.Marshal}
	currentUser    string
	ntype          = flag.String("ntype", "tcp4", "Default network type")
	naddr          = flag.String("addr", ":5640", "Network address")
	apitoken       = flag.String("apitoken", "", "Personal API Token for authentication")
	lognet         = flag.Bool("lognet", false, "Log network requests")
	server         *dynamic.Server
)

func markdown(content string) string {
	return "    " + strings.Replace(content, "\n", "\n    ", -1)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flag.Parse()

	if *apitoken != "" {
		log.Printf("Using Personal API Token for authentication. Caching is enabled.\n")
		authTs := oauth2.Transport{Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *apitoken})}
		cacheTs := httpcache.NewMemoryCacheTransport()
		cacheTs.Transport = &authTs

		client = github.NewClient(&http.Client{Transport: cacheTs})

		tc := oauth2.NewClient(context.Background(), authTs.Source)
		uncachedClient = github.NewClient(tc)

		cu, _, err := client.Users.Get(context.Background(), "")
		if err != nil {
			panic(err)
		}
		currentUser = *cu.Login
	} else {
		log.Printf("Using no authentication. Note that rate limits will apply. Caching is enabled.\n")
		client = github.NewClient(httpcache.NewMemoryCacheTransport().Client())
		uncachedClient = github.NewClient(nil)
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
			dynamic.NewFileEntry("/0intro.md", &dynamic.StaticFileHandler{[]byte(`
# GitHub File System

Welcome to a file system view of GitHub. Using the site is easy once you learn a few tricks. Since GitHub is a very large site parts of the system are hidden and load on-demand. In particular, the repos directory is empty until you attempt to access something inside. You can "cd _ghfsdir_/repos/sirnewton01" or even "cd _ghfsdir_/repos/sirnewton01/ghfs". From there you will see start to see parts of the filesystem fill in.

Files are rendered in Markdown or even simple text so that you can interact with it using simple text editors.

For each repo the open issues are shown under "_ghfs_/repos/_owner_/_repo_/issues". In that directory there is a
filter.md file that you can modify to change the issue filters. When you refresh the directory listing only the
issues matching the filter are shown.

## Markform

Various files are modifiable using "markform", which is a format built on top of markdown for highlighting
portions of a file that you can modify to perform certain actions, such as making a comment, changing the owner of
an issues filter, etc. When you make the change and save the file the system takes the necessary actions based
on what you entered or changed in the highlighted regions.

Markform has a number of different controls, such as text, checkbox, radio and list. Here is an example of a
text field.

Description = ___

The presence of a paragraph with an equal sign indicates that this is a form control. The three underscores
signify that the type of control is text. You can start writing the description by putting your cursor before
the underscores and type out your description. You do not need to remove the underscores. In fact, the underscores
tell the system where your description ends. Also, markform is optimized for editing and tries to avoid any
excess typing, such as the delete key, or extra cursor tricks. You can just place your cursor and type!

Description = Here is my excellent description!___

This is a simple check box example.

Student = []

Just put a lower case x inside the square braces and that's it!

Student = [x]

Radios are much the same except that there are labels for each option. The default option is sometimes
pre-checked with an "x."

Education = (x) elementary () high school () post-secondary

Delete the x from the default and put it in the option that you want.

Education = () elementary (x) hig school () post-secondary

There are also check box groups, which work much the same as the radios except that you can put an "x"
on all of the options you want or remove them the ones you don't want.

Lists look something like this.

Labels = ,, ___

You can add your own values like this. You don't need (and shouldn't) to remove the template at the end.
Just type in your new elements or remove existing elements.

Labels = ,, enhancement ,, ___

Date fields are shown in an RFC3339 (or ISO-8601) format that you can modify to specify the date that you
would like.

StartDate = 2010-01-02T15:04:05Z

That's about all there is to know about markform. The format is designed to be readable, make it clear
the expected format and make it easy to modify.

`)}),
		})

	d.AddFileEntry("/repos", &ReposHandler{dynamic.BasicDirHandler{d, nil}})

	server = d

	NewStarredReposHandler()

	if err := s.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
