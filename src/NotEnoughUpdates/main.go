package notenoughupdates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

		cmd := exec.Command("git", "clone", "--depth", "1", "--single-branch", "--branch", "master",
			"https://github.com/NotEnoughUpdates/NotEnoughUpdates-REPO", "NotEnoughUpdates-REPO")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		fmt.Println("[NOT-ENOUGH-UPDATES] Repository cloned successfully")
	} else {
		fmt.Println("[NOT-ENOUGH-UPDATES] Repository already exists")
		go func() {
			if err := UpdateNEURepository(); err != nil {
				fmt.Printf("[NOT-ENOUGH-UPDATES] Failed to update repository: %v\n", err)
			}
		}()
	}

	return nil
}

func UpdateNEURepository() error {
	if os.Getenv("FIBER_PREFORK_CHILD") != "" {
		return nil // Prefork children should not update the repository
	}

	// Get current HEAD commit hash before update
	beforeCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	beforeCmd.Dir = "NotEnoughUpdates-REPO"
	beforeHashBytes, _ := beforeCmd.Output()
	beforeHash := strings.TrimSpace(string(beforeHashBytes))

	// Fetch latest changes from remote
	fetchCmd := exec.Command("git", "fetch", "--depth", "1", "origin", "master")
	fetchCmd.Dir = "NotEnoughUpdates-REPO"
	output, err := fetchCmd.CombinedOutput()

	if err == nil {
		// Reset to origin/master to handle divergent branches gracefully
		resetCmd := exec.Command("git", "reset", "--hard", "origin/master")
		resetCmd.Dir = "NotEnoughUpdates-REPO"
		resetOutput, resetErr := resetCmd.CombinedOutput()
		output = append(output, resetOutput...)
		err = resetErr
	}

	if err != nil {
		outputStr := string(output)
		fmt.Printf("[NOT-ENOUGH-UPDATES] Update failed (%v: %s), removing and re-cloning repository...\n", err, outputStr)
		if removeErr := os.RemoveAll("NotEnoughUpdates-REPO"); removeErr != nil {
			return fmt.Errorf("failed to remove corrupted repository: %w", removeErr)
		}

		if initErr := InitializeNEURepository(); initErr != nil {
			return fmt.Errorf("failed to re-clone repository: %w", initErr)
		}

		fmt.Println("[NOT-ENOUGH-UPDATES] Repository re-cloned successfully")
		return nil
	}

	// Get current HEAD commit hash after update
	afterCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	afterCmd.Dir = "NotEnoughUpdates-REPO"
	afterHashBytes, err := afterCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	afterHash := strings.TrimSpace(string(afterHashBytes))

	if beforeHash != "" && beforeHash == afterHash {
		fmt.Println("[NOT-ENOUGH-UPDATES] Repository is already up to date")
		return nil
	}

	fmt.Printf("[NOT-ENOUGH-UPDATES] Updated to commit: %s\n", afterHash)

	return nil
}
