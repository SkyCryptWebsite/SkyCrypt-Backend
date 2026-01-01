package notenoughupdates

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

func InitializeNEURepository() error {
	if _, err := os.Stat("NotEnoughUpdates-REPO"); os.IsNotExist(err) {
		err := os.MkdirAll("NotEnoughUpdates-REPO", 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	gitDir := filepath.Join("NotEnoughUpdates-REPO", ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Println("[NOT-ENOUGH-UPDATES] Cloning NEU repository...")

		_, err := git.PlainClone("NotEnoughUpdates-REPO", false, &git.CloneOptions{
			URL:           "https://github.com/NotEnoughUpdates/NotEnoughUpdates-REPO",
			Progress:      os.Stdout,
			Depth:         1,
			ReferenceName: "master",
			SingleBranch:  true,
		})

		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		fmt.Println("[NOT-ENOUGH-UPDATES] Repository cloned successfully")
	} else {
		_ = 0
		// fmt.Println("[NOT-ENOUGH-UPDATES] Repository already exists")
	}

	return nil
}

func UpdateNEURepository() error {
	repo, err := git.PlainOpen("NotEnoughUpdates-REPO")
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	workTree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// fmt.Println("[NOT-ENOUGH-UPDATES] Pulling latest changes...")

	err = workTree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Progress:   os.Stdout,
		Depth:      1,
	})

	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			// fmt.Println("[NOT-ENOUGH-UPDATES] Already up to date")
			return nil
		}

		fmt.Printf("[NOT-ENOUGH-UPDATES] Pull failed (%v), removing and re-cloning repository...\n", err)
		if removeErr := os.RemoveAll("NotEnoughUpdates-REPO"); removeErr != nil {
			return fmt.Errorf("failed to remove corrupted repository: %w", removeErr)
		}

		if initErr := InitializeNEURepository(); initErr != nil {
			return fmt.Errorf("failed to re-clone repository: %w", initErr)
		}

		fmt.Println("[NOT-ENOUGH-UPDATES] Repository re-cloned successfully")
		return nil
	}

	ref, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}

	fmt.Printf("[NOT-ENOUGH-UPDATES] Updated to commit: %s\n", commit.Hash.String()[:8])

	return nil
}
