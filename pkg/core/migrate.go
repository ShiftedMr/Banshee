package core

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/actions"
)

func (b *Banshee) Migrate() error {

	org := b.MigrationConfig.Organisation
	if b.MigrationConfig.Organisation == "" {
		org = b.GlobalConfig.Defaults.Organisation
	}

	// query := fmt.Sprintf("org:%s %s", org, b.MigrationConfig.SearchQuery)
	// repos, err := b.GithubClient.GetMatchingRepos(query)
	// if err != nil {
	// 	return err
	// }

	repos := []string{fmt.Sprintf("%s/containers", org)}

	for _, repo := range repos {
		_, repoErr := b.handleRepo(org, repo)
		if repoErr != nil {
			return repoErr
		}
	}

	// Get list of repos
	// For every repo:
	//		Shallow clone repo
	//		Create new git branch
	//		for each action
	// 			perform the action
	//			add changed files and commit with action description
	// 		Create a PR the changes
	return nil
}

// Handle the migration for a repo
func (b *Banshee) handleRepo(org, repo string) (string, error) {
	madeChanges := false
	repoNameOnly := strings.Replace(repo, org+"/", "", -1)

	dir, gitRepo, defaultBranch, cloneErr := b.cloneRepo(org, repo)
	if cloneErr != nil {
		return "", cloneErr
	}

	// Delete the repo directory when this function returns
	defer os.RemoveAll(dir)

	for _, action := range b.MigrationConfig.Actions {
		actionErr := actions.RunAction(action.Action, dir, action.Description, action.Input)
		if actionErr != nil {
			return "", actionErr
		}

		tree, _ := gitRepo.Worktree()
		state, _ := tree.Status()
		logrus.Debug("Checking if dirty...")
		// check if git dirty
		if !state.IsClean() {
			logrus.Debug("Is dirty, committing changes")
			madeChanges = true
			// if dirty, commit with action.Description as message

			addStr := "./"
			logrus.Debug(addStr)
			addErr := tree.AddGlob(addStr)
			if addErr != nil {
				return "", addErr
			}

			commit, commitErr := tree.Commit(action.Description, &git.CommitOptions{
				Author: &object.Signature{
					Name:  b.GlobalConfig.Defaults.GitName,
					Email: b.GlobalConfig.Defaults.GitEmail,
					When:  time.Now(),
				},
			})

			if commitErr != nil {
				return "", commitErr
			}

			obj, _ := gitRepo.CommitObject(commit)
			fmt.Println(obj)

			newState, _ := tree.Status()
			logrus.Debug("After commit: ", newState.IsClean())
		}
	}

	if madeChanges {
		// If we made at least one change, push to the remote
		htmlURL, err := b.pushChanges(gitRepo, org, repoNameOnly, defaultBranch)
		if err != nil {
			return "", err
		}

		logrus.Info("Created PR for ", repo, ": ", htmlURL)
		return htmlURL, nil
	}

	return "", nil
}

// Push changes to aGitHub nd create a Pull Request
func (b *Banshee) pushChanges(gitRepo *git.Repository, org, repoName, defaultBranch string) (string, error) {
	pushError := b.GithubClient.Push(b.MigrationConfig.BranchName, gitRepo)
	if pushError != nil {
		return "", fmt.Errorf("push error: %s", pushError)
	}

	bodyText, prFileErr := os.ReadFile(b.MigrationConfig.PRBodyFile)
	if prFileErr != nil {
		return "", prFileErr
	}

	htmlURL, prErr := b.GithubClient.CreatePullRequest(
		org, repoName, b.MigrationConfig.PRTitle, string(bodyText), defaultBranch, b.MigrationConfig.BranchName)
	if prErr != nil {
		return "", prErr
	}

	return htmlURL, nil
}

// Clone a new repo, and fetch info about its default branch
func (b *Banshee) cloneRepo(org, repo string) (string, *git.Repository, string, error) {
	repoNameOnly := strings.Replace(repo, org+"/", "", -1)
	dir, err := os.MkdirTemp(os.TempDir(), strings.Replace(repo, "/", "-", -1))
	if err != nil {
		return "", nil, "", err
	}
	logrus.Debug("Created ", dir)

	gitRepo, cloneErr := b.GithubClient.ShallowClone(repo, dir, b.MigrationConfig.BranchName)
	if cloneErr != nil {
		return "", nil, "", fmt.Errorf("clone error: %s", cloneErr)
	}

	defaultBranch, defaultBranchErr := b.GithubClient.GetDefaultBranch(org, repoNameOnly)
	if defaultBranchErr != nil {
		return "", nil, "", defaultBranchErr
	}

	return dir, gitRepo, defaultBranch, nil
}
