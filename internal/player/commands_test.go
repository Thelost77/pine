package player

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestLaunchCmdPropagatesGenerationToLaunchError verifies that LaunchCmd
// embeds the supplied generation into the resulting PlayerLaunchErrMsg so the
// root model can discard stale launch errors from superseded sessions.
func TestLaunchCmdPropagatesGenerationToLaunchError(t *testing.T) {
	mp := &launchFailPlayer{}
	cmd := LaunchCmd(mp, "http://test/audio.mp3", 0, 0, false, nil, 100, 7)
	msg := cmd()
	launchErr, ok := msg.(PlayerLaunchErrMsg)
	if !ok {
		t.Fatalf("expected PlayerLaunchErrMsg, got %T", msg)
	}
	if launchErr.Generation != 7 {
		t.Errorf("PlayerLaunchErrMsg.Generation = %d, want 7", launchErr.Generation)
	}
}

// TestPlayerReadyMsgAndLaunchErrMsgZeroValueGeneration verifies the zero value
// of Generation is accepted (used by tests that inject these messages directly).
func TestPlayerReadyMsgAndLaunchErrMsgZeroValueGeneration(t *testing.T) {
	if (PlayerReadyMsg{}).Generation != 0 {
		t.Error("PlayerReadyMsg zero-value Generation should be 0")
	}
	if (PlayerLaunchErrMsg{}).Generation != 0 {
		t.Error("PlayerLaunchErrMsg zero-value Generation should be 0")
	}
}

func TestFormatStartPosition(t *testing.T) {
	tests := []struct {
		name      string
		startTime float64
		duration  float64
		want      string
	}{
		{
			name:      "late resume uses percent",
			startTime: 93767,
			duration:  124076,
			want:      fmt.Sprintf("%f%%", 93767.0/124076.0*100),
		},
		{
			name:      "unknown duration uses seconds",
			startTime: 30,
			duration:  0,
			want:      fmt.Sprintf("%f", 30.0),
		},
		{
			name:      "start at zero uses seconds",
			startTime: 0,
			duration:  124076,
			want:      fmt.Sprintf("%f", 0.0),
		},
		{
			name:      "past end clamps to 100 percent",
			startTime: 200,
			duration:  100,
			want:      fmt.Sprintf("%f%%", 100.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStartPosition(tt.startTime, tt.duration)
			if got != tt.want {
				t.Fatalf("formatStartPosition(%v, %v) = %q, want %q", tt.startTime, tt.duration, got, tt.want)
			}
		})
	}
}

// launchFailPlayer implements Player and fails Launch, so that LaunchCmd
// returns PlayerLaunchErrMsg.
type launchFailPlayer struct{}

func (launchFailPlayer) Launch(url, startTime string, paused bool, httpHeaders []string, volume int) error {
	return errLaunchSentinel
}
func (launchFailPlayer) GetPosition() (float64, error)      { return 0, nil }
func (launchFailPlayer) GetDuration() (float64, error)      { return 0, nil }
func (launchFailPlayer) GetPaused() (bool, error)           { return false, nil }
func (launchFailPlayer) GetIdle() (bool, error)             { return false, nil }
func (launchFailPlayer) SetPause(paused bool) error         { return nil }
func (launchFailPlayer) Seek(seconds float64) error         { return nil }
func (launchFailPlayer) SeekRelative(seconds float64) error { return nil }
func (launchFailPlayer) SetSpeed(speed float64) error       { return nil }
func (launchFailPlayer) SetVolume(vol int) error            { return nil }
func (launchFailPlayer) GetVolume() (int, error)            { return 100, nil }
func (launchFailPlayer) Quit() error                        { return nil }

var errLaunchSentinel = &sentinelError{"launch failed"}

type sentinelError struct{ s string }

func (e *sentinelError) Error() string { return e.s }

// stateFailPlayer implements Player and fails SetPause, SetSpeed, and
// SetVolume.
type stateFailPlayer struct{ launchFailPlayer }

func (stateFailPlayer) SetPause(paused bool) error   { return errStateSentinel }
func (stateFailPlayer) SetSpeed(speed float64) error { return errStateSentinel }
func (stateFailPlayer) SetVolume(vol int) error      { return errStateSentinel }

var errStateSentinel = &sentinelError{"state not accepted"}

// TestStateCommandsReportFailure verifies that a command for the pause state,
// the speed, or the volume reports a failure of the player as CommandErrMsg.
// The failure is not a position tick: an app ignores a tick that has no
// generation.
func TestStateCommandsReportFailure(t *testing.T) {
	p := stateFailPlayer{}
	cmds := map[string]tea.Cmd{
		"pause":  SetPauseCmd(p, true),
		"speed":  SetSpeedCmd(p, 1.5),
		"volume": SetVolumeCmd(p, 50),
	}
	for name, cmd := range cmds {
		msg, ok := cmd().(CommandErrMsg)
		if !ok || msg.Err != error(errStateSentinel) {
			t.Errorf("%s: the command returned %#v, want CommandErrMsg with the error of the player", name, cmd())
		}
	}
}
