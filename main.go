package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func isGitRepo() (bool, string, error) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return false, "", fmt.Errorf("not a git repository")
	}

	isRepo := strings.TrimSpace(string(out)) == "true"
	if !isRepo {
		return false, "", fmt.Errorf("not a git repository")
	}

	cmd = exec.Command("git", "branch", "--show-current")
	branchOut, err := cmd.Output()
	if err != nil {
		return false, "", fmt.Errorf("failed to get the current branch")
	}

	currentBranch := strings.TrimSpace(string(branchOut))
	protectedBranches := []string{"master", "develop", "main"}
	for _, branch := range protectedBranches {
		if currentBranch == branch {
			return true, currentBranch, fmt.Errorf("current branch '%s' is a protected branch", currentBranch)
		}
	}

	return true, currentBranch, nil
}

func getFileGitHistory(filePath string) ([]string, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%h, %an, %ad, %s", "--date=format:%Y-%m-%d %H:%M:%S", "-n", "10", "--", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve git history: %v", err)
	}

	history := strings.Split(strings.TrimSpace(string(output)), "\n")
	fmt.Printf("\nGit history for '%s':\n", filePath)
	for i, line := range history {
		if i+1 < 10 {
			fmt.Printf(" %d. %s\n", i+1, line)
		} else {
			fmt.Printf("%d. %s\n", i+1, line)
		}
	}

	return history, nil
}

func rollbackToCommit(filePath string, commit string) error {
	cmd := exec.Command("git", "checkout", commit, "--", filePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout commit: %v", err)
	}

	commitMessage := fmt.Sprintf("Successfully rolled back '%s' to commit %s", filePath, commit)
	cmd = exec.Command("git", "commit", "-m", commitMessage, filePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create commit: %v", err)
	}

	return nil
}

func countRolloutFiles(dirPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(info.Name(), "rollout.yaml") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func handleSingleRolloutFile(filePath string) {
	history, err := getFileGitHistory(filePath)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	for {
		defaultIndex := 2
		if len(history) < defaultIndex {
			defaultIndex = len(history)
		}
		fmt.Printf("Enter the number of the commit to rollback to [%d]: ", defaultIndex)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input := scanner.Text()
		if input == "" {
			input = strconv.Itoa(defaultIndex)
		}
		index, err := strconv.Atoi(input)
		if err != nil || index < 1 || index > len(history) {
			fmt.Println("Invalid number. Please try again.")
			continue
		}

		if index == 1 {
			fmt.Printf("No rollback has been done for '%s' because it is already at commit number 1.\n", filePath)
			break
		}

		commit := strings.Split(history[index-1], ",")[0]
		if err := rollbackToCommit(filePath, commit); err != nil {
			fmt.Println("Error rolling back:", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully rolled back '%s' to commit %s.\n", filePath, commit)
		break
	}
}

func handleDirectoryRolloutFiles(dirPath string) {
	files, err := countRolloutFiles(dirPath)
	if err != nil {
		fmt.Println("Error walking the directory:", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d rollout.yaml files:\n", len(files))
	for _, file := range files {
		fmt.Println(file)
	}

	fmt.Printf("Would you like to continue with rolling back all %d of these files? (yes/no): ", len(files))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := strings.ToLower(scanner.Text())
	if response != "yes" {
		fmt.Println("Operation aborted by the user.")
		os.Exit(0)
	}

	fmt.Println("Proceeding with rollback for all rollout.yaml files...")
	for _, file := range files {
		handleSingleRolloutFile(file)
	}
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "rollback [path]",
		Short: "Check if a file or directory exists at the given path",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			inputPath := args[0]
			if _, err := os.Stat(inputPath); os.IsNotExist(err) {
				fmt.Printf("The path '%s' does not exist.\n", inputPath)
				os.Exit(1)
			}

			// check we are in a git repo
			_, _, err := isGitRepo()
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}

			if strings.HasSuffix(inputPath, "rollout.yaml") {
				handleSingleRolloutFile(inputPath)
			} else {
				handleDirectoryRolloutFiles(inputPath)
			}
		},
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
