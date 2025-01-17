package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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
	fmt.Printf("Git history for '%s':\n", filePath)
	for i, line := range history {
		fmt.Printf("%d. %s\n", i+1, line)
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
	cmd = exec.Command("git", "commit", "-am", commitMessage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create commit: %v", err)
	}

	return nil
}

func main() {
	var count int

	// Create the root command
	var rootCmd = &cobra.Command{
		Use:   "cli",
		Short: "A CLI tool with subcommands",
		Long:  `This CLI tool provides two subcommands: hello and rollback.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Available subcommands:")
			fmt.Println("  hello    Print 'Hello, World!' multiple times")
			fmt.Println("  rollback Check if a file or directory exists at a given path")
		},
	}

	// Create the hello command
	var helloCmd = &cobra.Command{
		Use:   "hello",
		Short: "Print 'Hello, World!' multiple times",
		Run: func(cmd *cobra.Command, args []string) {
			for i := 1; i <= count; i++ {
				fmt.Printf("Hello, World! %d\n", i)
			}
		},
	}
	helloCmd.Flags().IntVarP(&count, "count", "c", 1, "Number of times to print 'Hello, World!'")

	// Create the rollback command
	var rollbackCmd = &cobra.Command{
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
				history, err := getFileGitHistory(inputPath)
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

					commit := strings.Split(history[index-1], ",")[0]
					if err := rollbackToCommit(inputPath, commit); err != nil {
						fmt.Println("Error rolling back:", err)
						os.Exit(1)
					}
					fmt.Printf("Successfully rolled back '%s' to commit %s.\n", inputPath, commit)
					break
				}
			} else {
				fmt.Println("The file is not named 'rollout.yaml'.")
			}
		},
	}

	// Add subcommands to the root command
	rootCmd.AddCommand(helloCmd)
	rootCmd.AddCommand(rollbackCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
