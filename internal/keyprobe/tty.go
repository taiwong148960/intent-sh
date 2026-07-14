package keyprobe

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

const (
	readPollInterval = 50 * time.Millisecond
	readIdleWindow   = 30 * time.Millisecond
)

type ttySession struct {
	file     *os.File
	state    *term.State
	mu       sync.Mutex
	restored bool
}

// OpenControllingTTY opens /dev/tty directly and enters temporary raw mode. It
// never falls back to stdin, so a pipeline cannot be consumed accidentally.
func OpenControllingTTY() (Session, error) {
	file, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	fd := int(file.Fd())
	if fd < 0 || fd >= fdSetCapacity() {
		_ = file.Close()
		return nil, errors.New("controlling terminal descriptor exceeds the bounded reader capacity")
	}
	if !term.IsTerminal(fd) {
		_ = file.Close()
		return nil, errors.New("controlling terminal is not a terminal")
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &ttySession{file: file, state: state}, nil
}

func fdSetCapacity() int {
	set := unix.FdSet{}
	return len(set.Bits) * int(unsafe.Sizeof(set.Bits[0])) * 8
}

func (session *ttySession) Prompt(value string) error {
	_, err := io.WriteString(session.file, value)
	return err
}

func (session *ttySession) ReadBounded(ctx context.Context, timeout time.Duration, maxBytes int) ([]byte, error) {
	if timeout <= 0 {
		return nil, ErrTimeout
	}
	if maxBytes <= 0 || maxBytes > DefaultMaxInputBytes {
		maxBytes = DefaultMaxInputBytes
	}
	overallDeadline := time.Now().Add(timeout)
	lastByteAt := time.Time{}
	received := make([]byte, 0, maxBytes+1)
	failed := true
	defer func() {
		if failed {
			wipeBytes(received)
		}
	}()
	fd := int(session.file.Fd())
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		now := time.Now()
		activeDeadline := overallDeadline
		if !lastByteAt.IsZero() {
			idleDeadline := lastByteAt.Add(readIdleWindow)
			if idleDeadline.Before(activeDeadline) {
				activeDeadline = idleDeadline
			}
		}
		if !now.Before(activeDeadline) {
			if len(received) > 0 {
				failed = false
				return received, nil
			}
			return nil, ErrTimeout
		}
		pollDuration := readPollInterval
		if remaining := activeDeadline.Sub(now); remaining < pollDuration {
			pollDuration = remaining
		}
		readSet := unix.FdSet{}
		if fd < 0 || fd >= fdSetCapacity() {
			return nil, os.ErrInvalid
		}
		readSet.Set(fd)
		timeval := unix.NsecToTimeval(pollDuration.Nanoseconds())
		ready, pollErr := unix.Select(fd+1, &readSet, nil, nil, &timeval)
		if pollErr != nil {
			if errors.Is(pollErr, unix.EINTR) {
				continue
			}
			return nil, pollErr
		}
		if ready == 0 {
			continue
		}
		if !readSet.IsSet(fd) {
			continue
		}
		remaining := maxBytes + 1 - len(received)
		if remaining <= 0 {
			return nil, ErrTooManyBytes
		}
		buffer := make([]byte, remaining)
		count, readErr := unix.Read(fd, buffer)
		if count > 0 {
			received = append(received, buffer[:count]...)
			lastByteAt = time.Now()
			if len(received) > maxBytes {
				wipeBytes(buffer)
				return nil, ErrTooManyBytes
			}
		}
		wipeBytes(buffer)
		if count == 0 && readErr == nil {
			return nil, io.EOF
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, unix.EINTR) || errors.Is(readErr, unix.EAGAIN) {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			return nil, io.EOF
		}
		return nil, readErr
	}
}

func wipeBytes(values []byte) {
	for index := range values {
		values[index] = 0
	}
}

func (session *ttySession) Restore() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.restored {
		return nil
	}
	if err := term.Restore(int(session.file.Fd()), session.state); err != nil {
		return err
	}
	session.restored = true
	return nil
}

func (session *ttySession) Close() error {
	return session.file.Close()
}
