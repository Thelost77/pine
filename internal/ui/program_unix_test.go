//go:build unix

package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// hangupChildEnv marks the process that TestRunProgramReturnsAtHangup starts.
const hangupChildEnv = "PINE_TEST_HANGUP_CHILD"

// waitModel is a program that waits. Its Init prints a line when the program
// runs.
type waitModel struct{}

func (waitModel) Init() tea.Cmd {
	return func() tea.Msg {
		fmt.Println("running")
		return nil
	}
}

func (m waitModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (waitModel) View() string { return "" }

// runHangupChild runs a program with RunProgram, as the main of an app does,
// and prints a line at the place where that main calls Cleanup.
func runHangupChild() {
	p := tea.NewProgram(waitModel{}, tea.WithInput(nil), tea.WithOutput(io.Discard))
	_, _ = RunProgram(p)
	fmt.Println("cleanup")
	os.Exit(0)
}

// TestRunProgramReturnsAtHangup verifies that the process gets to the place
// after the program when it gets SIGHUP, which is the close of the terminal.
// The main of an app calls Cleanup there. The Cleanup of pine sends the last
// position and the listening time.
func TestRunProgramReturnsAtHangup(t *testing.T) {
	if os.Getenv(hangupChildEnv) == "1" {
		runHangupChild()
	}
	child := exec.Command(os.Args[0], "-test.run=^TestRunProgramReturnsAtHangup$")
	child.Env = append(os.Environ(), hangupChildEnv+"=1")
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	// A child that does not end fails the test, and does not block it.
	kill := time.AfterFunc(10*time.Second, func() { _ = child.Process.Kill() })
	defer kill.Stop()

	lines := bufio.NewScanner(stdout)
	if !lines.Scan() || lines.Text() != "running" {
		t.Fatalf("first line of the child = %q, want running", lines.Text())
	}
	if err := child.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	var after []string
	for lines.Scan() {
		after = append(after, lines.Text())
	}
	err = child.Wait()
	if err != nil || len(after) != 1 || after[0] != "cleanup" {
		t.Errorf("after SIGHUP, the process printed %q and ended with the error %v, want the line cleanup and no error", after, err)
	}
}
