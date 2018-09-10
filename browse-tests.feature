Feature: Repository browsing

Scenario: Browse an arbitrary repo
  Given ghfs is running and mounted
  When the user cd (chdir) to the repos/someuser/somerepo
  Then the chdir is successful and the user can see the repo.md file with the metadata

Scenario: Browse a repo that is owned by a followee
  Given ghfs is running, mounted and authenticated as the current user
  When the user cd (chdir) to the repos/someuser
  Then the chdir is successful and the user can see all of the repos for that user

Scenario: Browse a repo that is starred
  Given ghfs is running, mounted and authenticated as the current user
  And the user has starred a repo
  When the user opens the repos/stars.md file
  Then the user sees the starred repo in the list of starred repos

Scenario: Browse a repo that was the origin of a forked repo
  Given ghfs is runing and mounted
  When the user opens an arbitrary repo that has been forked from another repo
  And the user opens the path shown in the repo.md to the original repo
  Then the user is able to open and read the repo.md file of the original repo

