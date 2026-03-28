package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/fastclaw-ai/anyclaw/internal/adapter"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the browser bridge daemon",
	Long: `Manage the daemon that bridges browser extensions and anyclaw commands.

The daemon accepts WebSocket connections from browser extensions and
HTTP commands from the CLI, forwarding between them.

Examples:
  anyclaw daemon start     # start daemon (foreground)
  anyclaw daemon status    # check daemon status
  anyclaw daemon stop      # stop daemon`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the browser bridge daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		d := adapter.NewDaemon(port)

		// Save PID for management
		adapter.WriteDaemonPID(os.Getpid())
		defer adapter.RemoveDaemonPID()

		return d.Start()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check daemon and extension status",
	Run: func(cmd *cobra.Command, args []string) {
		pid := adapter.ReadDaemonPID()
		if pid <= 0 || !isAlive(pid) {
			fmt.Println("Daemon: not running")
			fmt.Println("\nStart with: anyclaw daemon start &")
			return
		}

		fmt.Printf("Daemon: running (pid=%d)\n", pid)

		connected, version := adapter.BridgeStatus()
		if connected {
			fmt.Printf("Extension: connected (v%s)\n", version)
		} else {
			fmt.Println("Extension: not connected")
			fmt.Println("\nInstall the AnyClaw browser extension:")
			fmt.Println("  1. chrome://extensions → Enable Developer Mode")
			fmt.Println("  2. Load unpacked → select 'extension' directory from anyclaw repo")
		}
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		pid := adapter.ReadDaemonPID()
		if pid <= 0 {
			return fmt.Errorf("daemon is not running")
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		proc.Kill()
		adapter.RemoveDaemonPID()
		fmt.Println("Daemon stopped.")
		return nil
	},
}

func isAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func init() {
	daemonStartCmd.Flags().Int("port", 19825, "Daemon port")
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	rootCmd.AddCommand(daemonCmd)
}
