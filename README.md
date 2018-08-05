# GitHub File System

With this filesystem you can use GitHub from the freedom of your favorite shell and text editor.
In particular, you can use this to mount GitHub onto a Plan 9 system and use it to collaborate
on projects with other Plan 9 users. Content is presented either in plain text or rich markdown
for ease of viewing or converting them into graphical formats for offline reading.

## Current feature set
* Browse repositories by owner
* Read open issues

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
The filesystem uses not authentication with GitHub by default. The rate limit is much lower in this mode.
You can generate a Personal Access Token in your Settings > Develper Settings screen. With a token you
can provide it in the command-line with the ```-apitoken``` flag.
