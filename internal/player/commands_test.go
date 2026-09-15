package player

import (
	"fmt"
	"testing"
)

// TestLaunchCmdPropagatesGenerationToLaunchError verifies that LaunchCmd
// embeds the supplied generation into the resulting PlayerLaunchErrMsg so the
// root model can discard stale launch errors from superseded sessions.
func TestLaunchCmdPropagatesGenerationToLaunchError(t *testing.T) {
	mp := &launchFailPlayer{}
	cmd := LaunchCmd(mp, "http://test/audio.mp3", 0, 0, false, nil, 7)
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

// launchFailPlayer implements Player and fails Launch immediately so LaunchCmd
// returns PlayerLaunchErrMsg without waiting for the 3s socket-retry loop.
type launchFailPlayer struct{}

func (launchFailPlayer) Launch(url, startTime, socketPath string, paused bool, httpHeaders []string) error {
	return errLaunchSentinel
}
func (launchFailPlayer) Connect() error                       { return nil }
func (launchFailPlayer) GetPosition() (float64, error)       { return 0, nil }
func (launchFailPlayer) GetDuration() (float64, error)       { return 0, nil }
func (launchFailPlayer) GetPaused() (bool, error)             { return false, nil }
func (launchFailPlayer) SetPause(paused bool) error          { return nil }
func (launchFailPlayer) Seek(seconds float64) error           { return nil }
func (launchFailPlayer) SetSpeed(speed float64) error         { return nil }
func (launchFailPlayer) SetVolume(vol int) error             { return nil }
func (launchFailPlayer) GetVolume() (int, error)             { return 100, nil }
func (launchFailPlayer) Quit() error                         { return nil }

var errLaunchSentinel = &sentinelError{"launch failed"}

type sentinelError struct{ s string }

func (e *sentinelError) Error() string { return e.s }