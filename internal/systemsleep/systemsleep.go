// Package systemsleep lets an app finish work before the system sleeps. It
// holds a logind sleep delay inhibitor, so that logind waits before a suspend
// or a hibernation until the app has done its work.
//
// The name keeps the package apart from the sleep timer of pine.
package systemsleep

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/Thelost77/pine/internal/logger"
)

const (
	logindName    = "org.freedesktop.login1"
	logindPath    = dbus.ObjectPath("/org/freedesktop/login1")
	managerIface  = "org.freedesktop.login1.Manager"
	prepareSignal = managerIface + ".PrepareForSleep"
	inhibitReason = "Stop playback before sleep"

	// releaseAfter is the longest time that the monitor holds logind back
	// before a sleep. It is the timeout of an abs.Client request. logind
	// itself waits at most InhibitDelayMaxSec, which can be shorter.
	releaseAfter = 10 * time.Second
)

// Monitor holds a logind sleep delay inhibitor. Before each sleep, it calls
// prepare, and it releases the inhibitor when prepare calls release or when
// the wait time ends. After the wake, it takes a new inhibitor.
type Monitor struct {
	conn    *dbus.Conn
	who     string
	wait    time.Duration
	prepare func(release func())

	mu        sync.Mutex
	inhibitor *os.File
	closed    bool
}

// Start connects to the system bus and starts a monitor. who is the name that
// logind shows for the inhibitor. prepare runs in its own goroutine before
// each sleep, and it must call release when its work has ended.
//
// Without a system bus, for example on macOS and Windows, Start logs the
// cause and returns nil. A nil monitor is valid.
func Start(who string, prepare func(release func())) *Monitor {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		logger.Info("system sleep monitor is off: no system bus", "err", err)
		return nil
	}
	m, err := start(conn, who, releaseAfter, prepare)
	if err != nil {
		logger.Warn("system sleep monitor is off", "err", err)
		_ = conn.Close()
		return nil
	}
	return m
}

func start(conn *dbus.Conn, who string, wait time.Duration, prepare func(release func())) (*Monitor, error) {
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender(logindName),
		dbus.WithMatchObjectPath(logindPath),
		dbus.WithMatchInterface(managerIface),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		return nil, fmt.Errorf("subscribe to PrepareForSleep: %w", err)
	}
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)

	m := &Monitor{conn: conn, who: who, wait: wait, prepare: prepare}
	m.inhibit()
	go m.loop(signals)
	return m, nil
}

// inhibit takes a sleep delay inhibitor. If logind refuses it, the monitor
// still calls prepare before a sleep, but logind does not wait.
func (m *Monitor) inhibit() {
	var fd dbus.UnixFD
	err := m.conn.Object(logindName, logindPath).
		Call(managerIface+".Inhibit", 0, "sleep", m.who, inhibitReason, "delay").
		Store(&fd)
	if err != nil {
		logger.Warn("system sleep: logind gave no inhibitor", "err", err)
		return
	}
	f := os.NewFile(uintptr(fd), "logind-inhibitor")

	m.mu.Lock()
	if m.closed || m.inhibitor != nil {
		m.mu.Unlock()
		_ = f.Close()
		return
	}
	m.inhibitor = f
	m.mu.Unlock()
	logger.Debug("system sleep: inhibitor taken")
}

func (m *Monitor) loop(signals <-chan *dbus.Signal) {
	for sig := range signals {
		if sig.Name != prepareSignal || sig.Path != logindPath || len(sig.Body) != 1 {
			continue
		}
		sleeping, ok := sig.Body[0].(bool)
		if !ok {
			continue
		}
		if sleeping {
			m.beforeSleep()
		} else {
			m.afterWake()
		}
	}
}

func (m *Monitor) beforeSleep() {
	m.mu.Lock()
	f := m.inhibitor
	m.inhibitor = nil
	m.mu.Unlock()
	logger.Info("system sleep: the system prepares to sleep")

	var once sync.Once
	release := func() {
		once.Do(func() {
			if f != nil {
				_ = f.Close()
			}
			logger.Debug("system sleep: inhibitor released")
		})
	}
	time.AfterFunc(m.wait, release)
	go m.prepare(release)
}

func (m *Monitor) afterWake() {
	logger.Info("system sleep: the system woke up")
	m.mu.Lock()
	skip := m.closed || m.inhibitor != nil
	m.mu.Unlock()
	if !skip {
		m.inhibit()
	}
}

// Close releases the inhibitor and closes the connection. It does nothing on
// a nil monitor.
func (m *Monitor) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	f := m.inhibitor
	m.inhibitor = nil
	m.closed = true
	m.mu.Unlock()
	if f != nil {
		_ = f.Close()
	}
	_ = m.conn.Close()
}
