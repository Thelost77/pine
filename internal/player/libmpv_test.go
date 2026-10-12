package player

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testAuthValue  = "Bearer secret, with a comma"
	testAuthHeader = "Authorization: " + testAuthValue
)

// requireLibmpv loads libmpv. Without the library the test skips. With
// PINE_REQUIRE_LIBMPV=1 it fails, so that a release cannot skip these tests.
func requireLibmpv(t *testing.T) {
	t.Helper()
	if err := loadLibmpv(); err != nil {
		if os.Getenv("PINE_REQUIRE_LIBMPV") == "1" {
			t.Fatalf("libmpv is required: %v", err)
		}
		t.Skipf("libmpv is not available: %v", err)
	}
}

// silentWAV returns a silent 8-bit mono WAV file.
func silentWAV(seconds int) []byte {
	const rate = 8000
	data := bytes.Repeat([]byte{0x80}, rate*seconds)
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	for _, field := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate), uint16(1), uint16(8)} {
		_ = binary.Write(&b, binary.LittleEndian, field)
	}
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	return b.Bytes()
}

// audioServer serves /long.wav (120 seconds) and /short.wav (1 second). It
// answers 401 to a request without the test Authorization header.
type audioServer struct {
	*httptest.Server
	authorized atomic.Bool
}

func newAudioServer(t *testing.T) *audioServer {
	t.Helper()
	long, short := silentWAV(120), silentWAV(1)
	s := &audioServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != testAuthValue {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		s.authorized.Store(true)
		body := long
		if r.URL.Path == "/short.wav" {
			body = short
		}
		http.ServeContent(w, r, "audio.wav", time.Time{}, bytes.NewReader(body))
	}))
	t.Cleanup(s.Close)
	return s
}

// eventually polls check until it returns nil.
func eventually(t *testing.T, what string, check func() error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		if err = check(); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s: %v", what, err)
}

// newTestHandle returns an initialized handle with no audio device.
func newTestHandle(t *testing.T, options ...[2]string) mpvHandle {
	t.Helper()
	requireLibmpv(t)
	h := mpvHandle(mpvCreate())
	t.Cleanup(func() { mpvTerminateDestroy(uintptr(h)) })
	for _, option := range append([][2]string{{"vid", "no"}, {"ao", "null"}, {"ytdl", "no"}}, options...) {
		if err := h.setOption(option[0], option[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := mpvError(mpvInitialize(uintptr(h)), "initialize"); err != nil {
		t.Fatal(err)
	}
	return h
}

// waitEndFile returns the first end-file event of the handle.
func waitEndFile(t *testing.T, h mpvHandle) mpvEndFile {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if event := mpvWaitEvent(uintptr(h), 0.1); event.id == mpvEventEndFile {
			return *(*mpvEndFile)(event.data)
		}
	}
	t.Fatal("no end-file event")
	return mpvEndFile{}
}

func TestOpenLibmpvMissingLibraryNamesThePackage(t *testing.T) {
	_, err := openLibmpv([]string{filepath.Join(t.TempDir(), "libmpv-absent")})
	if err == nil {
		t.Fatal("openLibmpv() opened a library that does not exist")
	}
	if !strings.Contains(err.Error(), libmpvInstallHint()) {
		t.Fatalf("error %q does not contain the install hint %q", err, libmpvInstallHint())
	}
}

func TestMpvErrorMapsPropertyUnavailable(t *testing.T) {
	if err := mpvError(0, "get time-pos"); err != nil {
		t.Fatalf("mpvError(0) = %v, want nil", err)
	}
	err := mpvError(mpvErrorPropertyUnavailable, "get time-pos")
	if !errors.Is(err, ErrPropertyUnavailable) {
		t.Fatalf("mpvError() = %v, want ErrPropertyUnavailable", err)
	}
}

func TestLibmpvHandlePlaysStreamWithHeaderAndOptions(t *testing.T) {
	srv := newAudioServer(t)
	h := newTestHandle(t, [2]string{"start", "50%"}, [2]string{"pause", "yes"}, [2]string{"volume", "35"})

	if err := h.command("change-list", "http-header-fields", "append", testAuthHeader); err != nil {
		t.Fatal(err)
	}
	if err := h.command("loadfile", srv.URL+"/long.wav"); err != nil {
		t.Fatal(err)
	}

	var position float64
	eventually(t, "time-pos after load", func() (err error) {
		position, err = h.getDouble("time-pos")
		return err
	})
	if position < 59 || position > 61 {
		t.Errorf("time-pos = %.1f, want 60 (50%% of 120 seconds)", position)
	}
	if !srv.authorized.Load() {
		t.Error("the server did not get the Authorization header")
	}
	if volume, err := h.getDouble("volume"); err != nil || volume != 35 {
		t.Errorf("volume = %v, %v, want 35", volume, err)
	}
	if paused, err := h.getFlag("pause"); err != nil || !paused {
		t.Errorf("pause = %v, %v, want true", paused, err)
	}

	if err := h.setDouble("speed", 1.5); err != nil {
		t.Fatal(err)
	}
	if speed, err := h.getDouble("speed"); err != nil || speed != 1.5 {
		t.Errorf("speed = %v, %v, want 1.5", speed, err)
	}
	if err := h.setFlag("pause", false); err != nil {
		t.Fatal(err)
	}
	if paused, err := h.getFlag("pause"); err != nil || paused {
		t.Errorf("pause = %v, %v, want false", paused, err)
	}
}

func TestLibmpvHandleReportsEndOfFile(t *testing.T) {
	srv := newAudioServer(t)
	h := newTestHandle(t)
	if err := h.command("change-list", "http-header-fields", "append", testAuthHeader); err != nil {
		t.Fatal(err)
	}
	if err := h.command("loadfile", srv.URL+"/short.wav"); err != nil {
		t.Fatal(err)
	}

	if end := waitEndFile(t, h); endFileReason(end.reason) != "eof" {
		t.Fatalf("end-file reason = %q, want eof", endFileReason(end.reason))
	}
}

// collectLogMessages loads url with the test Authorization header and returns
// the log texts of the mpv messages for that load.
func collectLogMessages(t *testing.T, level, url string) []string {
	t.Helper()
	h := newTestHandle(t)
	if err := mpvError(mpvRequestLogMessages(uintptr(h), level), "request log messages"); err != nil {
		t.Fatal(err)
	}
	if err := h.command("change-list", "http-header-fields", "append", testAuthHeader); err != nil {
		t.Fatal(err)
	}
	if err := h.command("loadfile", url); err != nil {
		t.Fatal(err)
	}
	var texts []string
	ended := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		switch event := mpvWaitEvent(uintptr(h), 0.2); event.id {
		case mpvEventLogMessage:
			texts = append(texts, (*mpvLogMessage)(event.data).logText())
		case mpvEventEndFile:
			// mpv gives other events before log messages, so messages of
			// this load can still follow.
			ended = true
		case 0:
			if ended {
				return texts
			}
		}
	}
	t.Fatal("no end-file event")
	return nil
}

func TestLibmpvHandleLogMessagesGiveTheCauseWithoutSecrets(t *testing.T) {
	// The server rejects each request, so mpv sends the header and then
	// reports an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	texts := collectLogMessages(t, mpvLogLevel, srv.URL+"/book.wav?token=query-secret")

	all := strings.Join(texts, "\n")
	if !strings.Contains(all, "404") {
		t.Errorf("the log messages do not give the HTTP status: %q", texts)
	}
	if !strings.Contains(all, "/book.wav") {
		t.Errorf("the log messages do not name the stream: %q", texts)
	}
	for _, secret := range []string{testAuthValue, "query-secret"} {
		if strings.Contains(all, secret) {
			t.Errorf("the log messages contain the secret %q: %q", secret, texts)
		}
	}
}

func TestLibmpvHandleReportsLoadError(t *testing.T) {
	srv := newAudioServer(t)
	h := newTestHandle(t)
	if err := h.command("loadfile", srv.URL+"/long.wav"); err != nil {
		t.Fatal(err)
	}

	end := waitEndFile(t, h)
	if endFileReason(end.reason) != "error" {
		t.Fatalf("end-file reason = %q, want error", endFileReason(end.reason))
	}
	if mpvErrorString(end.err) == "" {
		t.Error("end-file error has no text")
	}
}
