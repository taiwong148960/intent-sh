package keyprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestTTYReadBoundedReturnsEOFWithoutSpinning(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	session := &ttySession{file: reader}
	started := time.Now()
	_, err = session.ReadBounded(context.Background(), time.Second, DefaultMaxInputBytes)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("closed descriptor error = %v, want EOF", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("closed descriptor took %s; poll likely spun until the deadline", elapsed)
	}
}

func TestKeyProbePTYHelper(t *testing.T) {
	if os.Getenv("INTENT_SH_KEYPROBE_PTY_HELPER") != "1" {
		return
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tty.Close()
	before, err := term.GetState(int(tty.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	result := (Probe{PerKeyTimeout: 250 * time.Millisecond}).Run(t.Context(), "alt+g", "alt+u")
	after, err := term.GetState(int(tty.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	checks := checksByID(result)
	fmt.Printf("\r\nPTY_RESULT|READY=%t|RESTORED=%t|REWRITE=%s|UNDO=%s|ENTER=%s|CANCEL=%s|RESTORE=%s|DETAIL=%s|\r\n",
		result.Ready, terminalStatesEquivalent(before, after), checks[CheckRewrite].Status, checks[CheckUndo].Status,
		checks[CheckEnter].Status, checks[CheckCancel].Status, checks[CheckRestore].Status, checks[CheckRewrite].Detail)
}

func terminalStatesEquivalent(before, after *term.State) bool {
	return equalTerminalValue(reflect.ValueOf(before), reflect.ValueOf(after), "")
}

func equalTerminalValue(before, after reflect.Value, fieldName string) bool {
	if before.Kind() != after.Kind() {
		return false
	}
	switch before.Kind() {
	case reflect.Pointer:
		if before.IsNil() || after.IsNil() {
			return before.IsNil() == after.IsNil()
		}
		return equalTerminalValue(before.Elem(), after.Elem(), fieldName)
	case reflect.Struct:
		if before.NumField() != after.NumField() {
			return false
		}
		for index := 0; index < before.NumField(); index++ {
			if !equalTerminalValue(before.Field(index), after.Field(index), before.Type().Field(index).Name) {
				return false
			}
		}
		return true
	case reflect.Array:
		if before.Len() != after.Len() {
			return false
		}
		for index := 0; index < before.Len(); index++ {
			if !equalTerminalValue(before.Index(index), after.Index(index), fieldName) {
				return false
			}
		}
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		beforeValue, afterValue := before.Uint(), after.Uint()
		if runtime.GOOS == "darwin" && fieldName == "Lflag" {
			// PENDIN is a transient kernel status bit set while queued input is
			// reprocessed after canonical mode returns; it is not a mode change.
			beforeValue &^= 0x20000000
			afterValue &^= 0x20000000
		}
		return beforeValue == afterValue
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return before.Int() == after.Int()
	case reflect.Bool:
		return before.Bool() == after.Bool()
	default:
		return fmt.Sprint(before) == fmt.Sprint(after)
	}
}

func TestProbeThroughPTYRestoresState(t *testing.T) {
	tests := []struct {
		name        string
		rewrite     []byte
		signal      bool
		wantReady   bool
		wantRewrite Status
	}{
		{name: "success", rewrite: []byte{0x1b, 'g'}, wantReady: true, wantRewrite: StatusPass},
		{name: "transformed", rewrite: []byte{0x1b, 'x'}, wantRewrite: StatusFail},
		{name: "excessive", rewrite: []byte("123456789"), wantRewrite: StatusFail},
		{name: "timeout", wantRewrite: StatusFail},
		{name: "termination signal", signal: true, wantRewrite: StatusFail},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestKeyProbePTYHelper$")
			command.Env = append(os.Environ(), "INTENT_SH_KEYPROBE_PTY_HELPER=1")
			terminal, err := pty.Start(command)
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			reader := newProbePTYReader(terminal)
			reader.readUntil(t, "press Alt+G now", 3*time.Second)
			if test.signal {
				if err := command.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			} else {
				if len(test.rewrite) > 0 {
					if _, err := terminal.Write(test.rewrite); err != nil {
						t.Fatal(err)
					}
				}
				reader.readUntil(t, "press Alt+U now", 3*time.Second)
				if _, err := terminal.Write([]byte{0x1b, 'u'}); err != nil {
					t.Fatal(err)
				}
				reader.readUntil(t, "press Enter now", 3*time.Second)
				if _, err := terminal.Write([]byte{'\n'}); err != nil {
					t.Fatal(err)
				}
				reader.readUntil(t, "press Ctrl+C now", 3*time.Second)
				if _, err := terminal.Write([]byte{0x03}); err != nil {
					t.Fatal(err)
				}
			}
			output := reader.readUntil(t, "|RESTORE=PASS|", 3*time.Second)
			if err := command.Wait(); err != nil {
				t.Fatalf("helper failed: %v\n%s", err, output)
			}
			for _, want := range []string{"PTY_RESULT|", "RESTORED=true", "RESTORE=PASS", "REWRITE=" + string(test.wantRewrite)} {
				if !strings.Contains(output, want) {
					t.Fatalf("PTY result omitted %q:\n%s", want, output)
				}
			}
			if strings.Contains(output, fmt.Sprintf("READY=%t", !test.wantReady)) {
				t.Fatalf("PTY readiness mismatch:\n%s", output)
			}
		})
	}
}

type probePTYReader struct {
	file   *os.File
	buffer strings.Builder
	chunks chan probePTYChunk
}

type probePTYChunk struct {
	data []byte
	err  error
}

func newProbePTYReader(file *os.File) *probePTYReader {
	reader := &probePTYReader{file: file, chunks: make(chan probePTYChunk, 1)}
	go func() {
		for {
			buffer := make([]byte, 1024)
			count, err := file.Read(buffer)
			if count > 0 {
				reader.chunks <- probePTYChunk{data: append([]byte(nil), buffer[:count]...)}
			}
			if err != nil {
				reader.chunks <- probePTYChunk{err: err}
				return
			}
		}
	}()
	return reader
}

func (reader *probePTYReader) readUntil(t *testing.T, needle string, timeout time.Duration) string {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for !strings.Contains(reader.buffer.String(), needle) {
		select {
		case chunk := <-reader.chunks:
			if len(chunk.data) > 0 {
				reader.buffer.Write(chunk.data)
			}
			if chunk.err != nil {
				t.Fatalf("read PTY waiting for %q: %v\n%s", needle, chunk.err, reader.buffer.String())
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for %q\n%s", needle, reader.buffer.String())
		}
	}
	return reader.buffer.String()
}
