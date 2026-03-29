package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fastclaw-ai/anyclaw/internal/adapter"
	"github.com/spf13/cobra"
)

var browserCmd = &cobra.Command{
	Use:   "browser <subcommand>",
	Short: "Raw browser control via the AnyClaw extension",
	Long: `Control the browser directly through the AnyClaw daemon and extension.

Subcommands:
  open <url>        Navigate the browser to a URL
  eval "<js>"       Evaluate JavaScript in the current tab
  screenshot        Capture a screenshot of the current tab
  status            Show daemon and extension connection status`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		sub := args[0]
		rest := args[1:]

		switch sub {
		case "open":
			return browserOpen(rest)
		case "eval":
			return browserEval(rest)
		case "screenshot":
			return browserScreenshot()
		case "status":
			return browserStatus()
		case "--help", "-h":
			return cmd.Help()
		default:
			return fmt.Errorf("unknown browser subcommand: %s", sub)
		}
	},
}

func browserOpen(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: anyclaw browser open <url>")
	}
	url := args[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	adapter.EnsureDaemon()

	if err := adapter.BridgeNavigate(url); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Navigated to %s\n", url)
	return nil
}

func browserEval(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: anyclaw browser eval \"<js>\"")
	}
	script := strings.Join(args, " ")

	adapter.EnsureDaemon()

	result, err := adapter.BridgeEvaluate(script)
	if err != nil {
		return err
	}
	fmt.Println(result)
	return nil
}

func browserScreenshot() error {
	adapter.EnsureDaemon()

	b64, err := adapter.BridgeScreenshot()
	if err != nil {
		return err
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("decode screenshot: %w", err)
	}

	filename := fmt.Sprintf("screenshot-%s.png", time.Now().Format("20060102-150405"))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("save screenshot: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Saved %s (%d bytes)\n", filename, len(data))
	return nil
}

func browserStatus() error {
	pid := adapter.ReadDaemonPID()
	if pid == 0 {
		fmt.Println("Daemon: not running")
	} else {
		fmt.Printf("Daemon: running (PID %d)\n", pid)
	}

	connected, version := adapter.BridgeStatus()
	if connected {
		fmt.Printf("Extension: connected (v%s)\n", version)
	} else {
		fmt.Println("Extension: not connected")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(browserCmd)
}
