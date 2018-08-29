# GitHub File System

![travis ci](https://api.travis-ci.org/sirnewton01/ghfs.svg?branch=master)

With this filesystem you can use GitHub from your favorite shell and text editor.
In particular, you can use this to mount GitHub onto a Plan 9 system and use it to collaborate
on projects with other Plan 9 users. Content is presented either in plain text or rich markdown
for ease of viewing the content with few distractions. In some cases the files can be modified
and saved to activate new funcionality. If you want to save, copy, snapshot or otherwise work
with the data you can use the standard OS tools like ls, cp, and even the Finder or Explorer.
You can experiment to find combinations of commands that suit your need.

## Current feature set
* Browse repositories by owner (user or organization)
* Read issues
* Filter issues based on milestone, labels, assignee and creator
* Vew user and organization metadata

## Examples

```
$ ls /github/repos/sirnewton01

9p-mdns				godev				plan9adapter
Rest.ServiceProxy		godev-oracle			projectcreator
dgit				gojazz				rpi-9front
eclipse-filesystem-example	mdns				rtcdocker
gdblib				ninep				society-tests
ghfs				orion.client			ttf2plan9
git				orion.server			xinu
go				p9-tutorial
godbg				plan9-font-hack

$ cat /github/repos/sirnewton01/ghfs/repo.md

```
# sirnewton01/ghfs 

Description = 9p GitHub filesystem written in Go for use with Plan 9/p9p___

Watchers: 3
Stars: 3
Forks: 0

Default branch: master

git clone  https://github.com/sirnewton01/ghfs.git

$ cat mnt/repos/sirnewton01/ghfs/issues/13.md

# Show last modified time and creation time on issues (#13)

State: open - [sirnewton01](../../../sirnewton01) opened this issue 2018-08-10 15:32:24 +0000 UTC - 1 comments

Assignee:  Not Assigned 

Labels: enhancement 

    This becomes really useful when you are looking at issues and want to view/sort them according to how old they are or if there is recent activity.
    
    You can do a simple ```ls -l``` to browse them yourself or even sort them using ```ls -lt``` or ```ls -Ult```

## [sirnewton01](../../../sirnewton01) commented 2018-08-10 16:47:23 +0000 UTC (OWNER)

    Also, it would be useful to have the issues owned by a particular user, except that would only be visible on Plan 9, since the FUSE filesystems generally set the owners of everything to a specific user.
```

![acme-screenshot](docs/screenshot-acme.png)

## End Goal
Once in a stable state it should be possible to use the GitHub filesystem to manage all of
your Plan 9 projects, create new ones, track issues and collaborate with other users. It
should be possible to use this in conjunction with a tool such as [dgit](https://github.com/driusan/dgit)
or a git filesystem to update/merge/patch/push changes to GitHub while keeping track
of the progress of the project.

## Get Started - Plan 9 Port
Install the latest plan9port. Run ghfs. Mount the filesystem with ```9 mount localhost:5640 <mount-point>```
assuming the default tcp port 5640.

## Authentication
The filesystem uses no authentication with GitHub by default. The rate limit is much lower in this mode.
You can generate a Personal Access Token in your Settings > Develper Settings screen. With a token you
can provide it in the command-line with the ```-apitoken``` flag.

## Useful tricks
You can navigate to any user or organization  you want, not just the ones you follow. Open the /repos
directory, type in the name you want and right-click on it. It will open a new directory with the repos
and metadata for the name you selected. If the name doesn't exist it shows an error.

From the acme editor, you can navigate some of the hyperlinks to other users or repos by selecting
the relative link and middle-click. On Mac with Plan 9 Port you can middle click by holding down
control, alt and clicking on the text.

The issues directory uses the number of the issues as the file names, which doesn't give a great
overview of the active issues. You can run ```grep '^# ' *.md``` in the issues directory by
typing it in there and middle-clicking on it. It opens a new panel like this with the summary:

```
grep '^ #' *.md

11.md:# Activity feeds for users (#11)
12.md:# Activity feed for projects (#12)
13.md:# Show last modified time and creation time on issues (#13)
16.md:# Update changes to handler state on clunk instead of write (#16)
2.md:# Edit repository metadata (#2)
22.md:# Show repository contents (#22)
23.md:# File that shows the commit log of the current branch/tag (#23)
24.md:# Watch/unwatch/ignore repos (#24)
25.md:# Follow/unfollow users (#25)
4.md:# Issue search (#4)
5.md:# Code search (#5)
7.md:# Create/Edit Issues (#7)
8.md:# Pull requests are showing up as issues (#8)
```
