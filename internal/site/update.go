package site

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// UpdateFromGitHub clones or pulls bb-sites from GitHub into bbSitesDir.
func UpdateFromGitHub(ctx context.Context, bbSitesDir string) error {
	// Check git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found. Install git and try again")
	}

	if _, err := os.Stat(bbSitesDir); os.IsNotExist(err) {
		// Clone fresh
		fmt.Printf("Cloning bb-sites from GitHub...\n")
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1",
			"https://github.com/epiral/bb-sites.git", bbSitesDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	} else {
		// Pull latest
		fmt.Printf("Pulling latest bb-sites...\n")
		cmd := exec.CommandContext(ctx, "git", "-C", bbSitesDir, "pull", "--ff-only")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git pull failed: %w", err)
		}
	}

	return nil
}
