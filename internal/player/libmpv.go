package player

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/ebitengine/purego"
)

// Values from mpv/client.h.
const (
	mpvFormatFlag   = 3
	mpvFormatDouble = 5

	mpvEventShutdown   = 1
	mpvEventLogMessage = 2
	mpvEventEndFile    = 7

	mpvErrorPropertyUnavailable = -10

	// mpvLogLevel is the lowest level of the mpv messages that go to the app
	// log. The debug levels contain the HTTP request with its headers.
	mpvLogLevel = "warn"
)

// mpvEvent is mpv_event of client.h.
type mpvEvent struct {
	id   int32
	_    int32  // error
	_    uint64 // reply_userdata
	data unsafe.Pointer
}

// mpvEndFile is the start of mpv_event_end_file of client.h.
type mpvEndFile struct {
	reason int32
	err    int32
}

// mpvLogMessage is the start of mpv_event_log_message of client.h.
type mpvLogMessage struct {
	prefix *byte
	level  *byte
	text   *byte
}

// urlQuery matches the query of a URL in an mpv message.
var urlQuery = regexp.MustCompile(`\?[^\s"']+`)

// logText returns the text of the message for the app log. It removes the
// query of each URL, because a query can hold a token.
func (m *mpvLogMessage) logText() string {
	return urlQuery.ReplaceAllString(strings.TrimSpace(cString(m.text)), "?<removed>")
}

// cString copies a NUL-terminated C string.
func cString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// endFileReason names the reason of an end-file event as the mpv IPC does.
func endFileReason(code int32) string {
	switch code {
	case 0:
		return "eof"
	case 2:
		return "stop"
	case 3:
		return "quit"
	case 4:
		return "error"
	case 5:
		return "redirect"
	}
	return "unknown"
}

// The libmpv functions that the player uses. loadLibmpv sets them.
var (
	libmpvMu     sync.Mutex
	libmpvLoaded bool

	mpvCreate             func() uintptr
	mpvInitialize         func(handle uintptr) int32
	mpvTerminateDestroy   func(handle uintptr)
	mpvSetOptionString    func(handle uintptr, name, value string) int32
	mpvCommand            func(handle uintptr, args unsafe.Pointer) int32
	mpvSetProperty        func(handle uintptr, name string, format int32, data unsafe.Pointer) int32
	mpvGetProperty        func(handle uintptr, name string, format int32, data unsafe.Pointer) int32
	mpvWaitEvent          func(handle uintptr, timeout float64) *mpvEvent
	mpvRequestLogMessages func(handle uintptr, minLevel string) int32
	mpvErrorString        func(code int32) string
)

// loadLibmpv opens libmpv on first use, so that a machine without libmpv can
// start the app. A failed load is tried again on the next call.
func loadLibmpv() error {
	libmpvMu.Lock()
	defer libmpvMu.Unlock()
	if libmpvLoaded {
		return nil
	}
	library, err := openLibmpv(libmpvCandidates())
	if err != nil {
		return err
	}
	if err := registerLibmpv(library); err != nil {
		return err
	}
	libmpvLoaded = true
	return nil
}

// openLibmpv opens the first candidate that loads. The error names the package
// to install, because the app shows it to the user.
func openLibmpv(candidates []string) (uintptr, error) {
	var errs []error
	for _, name := range candidates {
		library, err := openLibrary(name)
		if err == nil {
			logger.Info("libmpv loaded", "library", name)
			return library, nil
		}
		errs = append(errs, err)
	}
	logger.Error("libmpv not found", "tried", candidates, "err", errors.Join(errs...))
	return 0, fmt.Errorf("libmpv not found: %s", libmpvInstallHint())
}

func registerLibmpv(library uintptr) (err error) {
	// RegisterLibFunc panics if the library does not have the symbol.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("libmpv is not usable: %v", r)
		}
	}()
	purego.RegisterLibFunc(&mpvCreate, library, "mpv_create")
	purego.RegisterLibFunc(&mpvInitialize, library, "mpv_initialize")
	purego.RegisterLibFunc(&mpvTerminateDestroy, library, "mpv_terminate_destroy")
	purego.RegisterLibFunc(&mpvSetOptionString, library, "mpv_set_option_string")
	purego.RegisterLibFunc(&mpvCommand, library, "mpv_command")
	purego.RegisterLibFunc(&mpvSetProperty, library, "mpv_set_property")
	purego.RegisterLibFunc(&mpvGetProperty, library, "mpv_get_property")
	purego.RegisterLibFunc(&mpvWaitEvent, library, "mpv_wait_event")
	purego.RegisterLibFunc(&mpvRequestLogMessages, library, "mpv_request_log_messages")
	purego.RegisterLibFunc(&mpvErrorString, library, "mpv_error_string")
	return nil
}

// mpvError converts a libmpv status code to an error.
func mpvError(code int32, action string) error {
	if code >= 0 {
		return nil
	}
	if code == mpvErrorPropertyUnavailable {
		return fmt.Errorf("%s: %w", action, ErrPropertyUnavailable)
	}
	return fmt.Errorf("%s: %s", action, mpvErrorString(code))
}

// mpvHandle is one mpv_handle of libmpv.
type mpvHandle uintptr

func (h mpvHandle) setOption(name, value string) error {
	return mpvError(mpvSetOptionString(uintptr(h), name, value), "set option "+name)
}

func (h mpvHandle) command(args ...string) error {
	// mpv_command takes a NULL-terminated array of C strings. The array holds
	// Go pointers, which the cgo pointer rules do not permit. This is safe
	// here: mpv_command copies the strings before it returns, the Go heap does
	// not move, and KeepAlive holds the buffers for the time of the call.
	buffers := make([][]byte, len(args))
	pointers := make([]*byte, len(args)+1)
	for i, arg := range args {
		buffers[i] = append([]byte(arg), 0)
		pointers[i] = &buffers[i][0]
	}
	code := mpvCommand(uintptr(h), unsafe.Pointer(&pointers[0]))
	runtime.KeepAlive(pointers)
	return mpvError(code, "command "+args[0])
}

func (h mpvHandle) getDouble(name string) (float64, error) {
	var value float64
	code := mpvGetProperty(uintptr(h), name, mpvFormatDouble, unsafe.Pointer(&value))
	return value, mpvError(code, "get "+name)
}

func (h mpvHandle) getFlag(name string) (bool, error) {
	var value int32
	code := mpvGetProperty(uintptr(h), name, mpvFormatFlag, unsafe.Pointer(&value))
	return value != 0, mpvError(code, "get "+name)
}

func (h mpvHandle) setDouble(name string, value float64) error {
	code := mpvSetProperty(uintptr(h), name, mpvFormatDouble, unsafe.Pointer(&value))
	return mpvError(code, "set "+name)
}

func (h mpvHandle) setFlag(name string, on bool) error {
	var value int32
	if on {
		value = 1
	}
	code := mpvSetProperty(uintptr(h), name, mpvFormatFlag, unsafe.Pointer(&value))
	return mpvError(code, "set "+name)
}
