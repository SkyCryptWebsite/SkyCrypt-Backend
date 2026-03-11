package notenoughupdates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	}

	return nil
}

func UpdateNEURepository() error {
	if os.Getenv("FIBER_PREFORK_CHILD") != "" {
		return nil // Prefork children should not update the repository
	}

	cmd := exec.Command("git", "pull", "--depth", "1", "origin", "master")
	cmd.Dir = "NotEnoughUpdates-REPO"
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		fmt.Printf("[NOT-ENOUGH-UPDATES] Pull failed (%v: %s), removing and re-cloning repository...\n", err, outputStr)
		if removeErr := os.RemoveAll("NotEnoughUpdates-REPO"); removeErr != nil {
			return fmt.Errorf("failed to remove corrupted repository: %w", removeErr)
		}

		if initErr := InitializeNEURepository(); initErr != nil {
			return fmt.Errorf("failed to re-clone repository: %w", initErr)
		}

		fmt.Println("[NOT-ENOUGH-UPDATES] Repository re-cloned successfully")
		return nil
	}

	outputStr := string(output)
	if outputStr == "Already up to date.\n" {
		return nil
	}

	// Get current HEAD commit hash
	hashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	hashCmd.Dir = "NotEnoughUpdates-REPO"
	hashOutput, err := hashCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	fmt.Printf("[NOT-ENOUGH-UPDATES] Updated to commit: %s", string(hashOutput))

	return nil
}
