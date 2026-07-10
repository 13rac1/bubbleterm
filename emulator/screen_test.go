package emulator

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

func TestEmulatorCreation(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	if e.ID() == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestEmulatorGetScreen(t *testing.T) {
	e, err := New(10, 5)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	// Initial screen should have damage (full redraw)
	frame := e.GetScreen()
	if len(frame.Rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(frame.Rows))
	}
	if len(frame.Damage) == 0 {
		t.Fatal("expected initial damage")
	}

	// After consuming, no more damage
	frame = e.GetScreen()
	if len(frame.Damage) != 0 {
		t.Fatalf("expected no damage after consumption, got %d", len(frame.Damage))
	}
}

func TestEmulatorResize(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	// Resize to new dimensions
	err = e.Resize(40, 12)
	if err != nil {
		t.Fatalf("failed to resize: %v", err)
	}

	// Get screen and verify new dimensions
	frame := e.GetScreen()
	if len(frame.Rows) != 12 {
		t.Fatalf("expected 12 rows after resize, got %d", len(frame.Rows))
	}
}

func TestEmulatorCursor(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	pos, visible := e.Cursor()
	// Cursor should be at origin initially
	if pos.X != 0 || pos.Y != 0 {
		t.Fatalf("expected cursor at (0,0), got (%d,%d)", pos.X, pos.Y)
	}
	if !visible {
		t.Fatal("expected cursor to be visible")
	}
}

func TestEmulatorPipeCreation(t *testing.T) {
	// Create a simple pipe pair
	r, w, err := createTestPipe()
	if err != nil {
		t.Skipf("could not create test pipe: %v", err)
	}

	e, err := NewFromPipes(80, 24, r, w)
	if err != nil {
		t.Fatalf("failed to create pipe-based emulator: %v", err)
	}
	defer e.Close()

	if e.ID() == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestEmulatorSendKey(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	// SendKey should not error on a valid emulator
	err = e.SendKey("hello")
	if err != nil {
		t.Fatalf("SendKey failed: %v", err)
	}
}

func TestEmulatorSendMouse(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	// Test mouse click
	err = e.SendMouse(0, 10, 5, true) // left click
	if err != nil {
		t.Fatalf("SendMouse (click) failed: %v", err)
	}

	// Test mouse release
	err = e.SendMouse(0, 10, 5, false) // left release
	if err != nil {
		t.Fatalf("SendMouse (release) failed: %v", err)
	}

	// Test mouse motion (button -1)
	err = e.SendMouse(-1, 15, 10, false)
	if err != nil {
		t.Fatalf("SendMouse (motion) failed: %v", err)
	}

	// Test middle and right buttons
	err = e.SendMouse(1, 5, 5, true) // middle click
	if err != nil {
		t.Fatalf("SendMouse (middle) failed: %v", err)
	}
	err = e.SendMouse(2, 5, 5, true) // right click
	if err != nil {
		t.Fatalf("SendMouse (right) failed: %v", err)
	}
}

func TestEmulatorSendMouseWheel(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	// Wheel up and wheel down should both be accepted. The button values
	// match vt.MouseWheelUp / vt.MouseWheelDown.
	if err := e.SendMouseWheel(int(vt.MouseWheelUp), 10, 5); err != nil {
		t.Fatalf("SendMouseWheel (up) failed: %v", err)
	}
	if err := e.SendMouseWheel(int(vt.MouseWheelDown), 10, 5); err != nil {
		t.Fatalf("SendMouseWheel (down) failed: %v", err)
	}
}

func TestEmulatorDoneClosesOnClose(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}

	done := e.Done()
	select {
	case <-done:
		t.Fatal("Done channel closed before Close")
	default:
	}

	if err := e.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	select {
	case <-done:
		// Expected: closed after Close.
	case <-time.After(time.Second):
		t.Fatal("Done channel not closed after Close")
	}
}

func TestEmulatorSetSize(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	err = e.SetSize(40, 12)
	if err != nil {
		t.Fatalf("SetSize failed: %v", err)
	}

	frame := e.GetScreen()
	if len(frame.Rows) != 12 {
		t.Fatalf("expected 12 rows after SetSize, got %d", len(frame.Rows))
	}
}

func TestEmulatorStartCommand(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	cmd := exec.Command("echo", "hello from bubbleterm")
	err = e.StartCommand(cmd)
	if err != nil {
		t.Fatalf("StartCommand failed: %v", err)
	}

	// Wait for the process to complete and output to be processed
	time.Sleep(200 * time.Millisecond)

	frame := e.GetScreen()
	combined := strings.Join(frame.Rows, "\n")
	if !strings.Contains(combined, "hello from bubbleterm") {
		t.Errorf("expected screen to contain 'hello from bubbleterm', got: %q", combined)
	}

	// Process should have exited
	if !e.IsProcessExited() {
		t.Error("expected process to have exited")
	}
}

func TestEmulatorStartCommandOnPipe(t *testing.T) {
	r, w, _ := createTestPipe()
	e, err := NewFromPipes(80, 24, r, w)
	if err != nil {
		t.Fatalf("failed to create pipe-based emulator: %v", err)
	}
	defer e.Close()

	cmd := exec.Command("echo", "test")
	err = e.StartCommand(cmd)
	if err == nil {
		t.Fatal("expected error when calling StartCommand on pipe-based emulator")
	}
}

func TestEmulatorPipeResponsesAreForwarded(t *testing.T) {
	reader := strings.NewReader("\x1b[c")
	writer := &captureWriteCloser{writes: make(chan []byte, 1)}

	e, err := NewFromPipes(80, 24, reader, writer)
	if err != nil {
		t.Fatalf("failed to create pipe-based emulator: %v", err)
	}
	defer e.Close()

	select {
	case got := <-writer.writes:
		if len(got) == 0 {
			t.Fatal("expected terminal response bytes")
		}
		if !bytes.HasPrefix(got, []byte("\x1b[?")) || !bytes.HasSuffix(got, []byte("c")) {
			t.Fatalf("expected device attributes response, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal response")
	}
}

func TestEmulatorOnExitCallback(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	exitCalled := make(chan string, 1)
	e.SetOnExit(func(id string) {
		exitCalled <- id
	})

	cmd := exec.Command("true")
	err = e.StartCommand(cmd)
	if err != nil {
		t.Fatalf("StartCommand failed: %v", err)
	}

	select {
	case id := <-exitCalled:
		if id != e.ID() {
			t.Errorf("exit callback received wrong ID: got %q, want %q", id, e.ID())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit callback")
	}
}

func TestEmulatorWriteAfterClose(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}

	e.Close()

	// Writing after close should error
	_, err = e.Write([]byte("test"))
	if err == nil {
		// Depending on OS, the PTY may or may not error immediately.
		// We just verify it doesn't panic.
		t.Log("Write after close did not error (OS-dependent behavior)")
	}
}

func TestEmulatorUniqueIDs(t *testing.T) {
	e1, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator 1: %v", err)
	}
	defer e1.Close()

	e2, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator 2: %v", err)
	}
	defer e2.Close()

	if e1.ID() == e2.ID() {
		t.Fatal("expected unique IDs for different emulators")
	}
}

func TestGetScreenReturnsCachedRowsWhenUndamaged(t *testing.T) {
	e, err := New(10, 5)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	// First call: consumes initial damage, renders cells.
	first := e.GetScreen()
	if len(first.Damage) == 0 {
		t.Fatal("expected initial damage")
	}

	// Second call without new data: should return cached rows, no damage,
	// and no renderCells work.
	second := e.GetScreen()
	if len(second.Damage) != 0 {
		t.Fatalf("expected no damage on cached call, got %d", len(second.Damage))
	}
	if len(second.Rows) != len(first.Rows) {
		t.Fatalf("cached rows length %d != first rows length %d", len(second.Rows), len(first.Rows))
	}
	for i := range first.Rows {
		if second.Rows[i] != first.Rows[i] {
			t.Errorf("cached row %d differs: %q vs %q", i, second.Rows[i], first.Rows[i])
		}
	}

	// Write data to trigger damage.
	e.mu.Lock()
	e.vt.Write([]byte("hello"))
	e.damaged = true
	e.mu.Unlock()

	// Third call: damaged, should re-render and return updated rows.
	third := e.GetScreen()
	if len(third.Damage) == 0 {
		t.Fatal("expected damage after write")
	}
	combined := strings.Join(third.Rows, "")
	if !strings.Contains(combined, "hello") {
		t.Errorf("expected 'hello' in re-rendered rows, got: %q", combined)
	}
}

func BenchmarkGetScreenUndamaged(b *testing.B) {
	e, err := New(80, 24)
	if err != nil {
		b.Fatal(err)
	}
	defer e.Close()

	// Consume initial damage.
	_ = e.GetScreen()

	for b.Loop() {
		e.GetScreen()
	}
}

func BenchmarkGetScreenDamaged(b *testing.B) {
	e, err := New(80, 24)
	if err != nil {
		b.Fatal(err)
	}
	defer e.Close()

	for b.Loop() {
		e.mu.Lock()
		e.damaged = true
		e.mu.Unlock()
		e.GetScreen()
	}
}

// BenchmarkGetScreenDamagedContent exercises the realistic damaged path where
// the screen is full of attributed (SGR) text, so renderCells does real
// work instead of returning blank rows.
func BenchmarkGetScreenDamagedContent(b *testing.B) {
	e, err := New(80, 24)
	if err != nil {
		b.Fatal(err)
	}
	defer e.Close()

	e.mu.Lock()
	for row := range 24 {
		// Move cursor, set a color, write a partial line so padding kicks in.
		e.vt.Write([]byte("\x1b[" + strconv.Itoa(row+1) + ";1H\x1b[1;32mrow content with attrs\x1b[0m"))
	}
	e.mu.Unlock()

	for b.Loop() {
		e.mu.Lock()
		e.damaged = true
		e.mu.Unlock()
		e.GetScreen()
	}
}

func TestEmulatorResizeMarksDamage(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	// Consume initial damage
	_ = e.GetScreen()

	// Resize should mark damage
	err = e.Resize(40, 12)
	if err != nil {
		t.Fatalf("Resize failed: %v", err)
	}

	frame := e.GetScreen()
	if len(frame.Damage) == 0 {
		t.Fatal("expected damage after resize")
	}
}

func TestEmulatorDirectWrite(t *testing.T) {
	e, err := New(40, 10)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	// Write ANSI content directly to the VT emulator
	e.mu.Lock()
	e.vt.Write([]byte("Hello World"))
	e.damaged = true
	e.mu.Unlock()

	frame := e.GetScreen()
	combined := strings.Join(frame.Rows, "")
	if !strings.Contains(combined, "Hello World") {
		t.Errorf("expected screen to contain 'Hello World', got: %q", combined)
	}
}

// createTestPipe creates a simple reader/writer pair for testing
func createTestPipe() (*testReader, *testWriter, error) {
	return &testReader{}, &testWriter{}, nil
}

type testReader struct{}

func (r *testReader) Read(p []byte) (n int, err error) {
	// Block forever - will be stopped by close
	select {}
}

type testWriter struct {
	closed bool
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (w *testWriter) Close() error {
	w.closed = true
	return nil
}

type captureWriteCloser struct {
	writes chan []byte
}

func (w *captureWriteCloser) Write(p []byte) (n int, err error) {
	buf := make([]byte, len(p))
	copy(buf, p)
	select {
	case w.writes <- buf:
	default:
	}
	return len(p), nil
}

func (w *captureWriteCloser) Close() error {
	close(w.writes)
	return nil
}

var _ io.WriteCloser = (*captureWriteCloser)(nil)

// TestMonitorProcessRace verifies that monitorProcess does not race on e.cmd.
// Before the fix, monitorProcess read e.cmd without the mutex while
// StartCommand wrote it under the lock — detectable by `go test -race`.
func TestMonitorProcessRace(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer e.Close()

	cmd := exec.Command("sleep", "0.01")
	if err := e.StartCommand(cmd); err != nil {
		t.Fatalf("StartCommand: %v", err)
	}

	// monitorProcess reads e.cmd without the mutex (lines 319, 324).
	// This write under the lock has no happens-before ordering with that
	// unsynchronized read, so the race detector flags it.
	e.mu.Lock()
	e.cmd = cmd
	e.mu.Unlock()

	time.Sleep(50 * time.Millisecond)
}

func TestCellAt(t *testing.T) {
	e, err := New(10, 5)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	e.mu.Lock()
	e.vt.Write([]byte("Hi"))
	e.mu.Unlock()

	h := e.CellAt(0, 0)
	if h == nil || h.Content != "H" {
		t.Fatalf("CellAt(0,0) = %v, want 'H'", h)
	}
	i := e.CellAt(1, 0)
	if i == nil || i.Content != "i" {
		t.Fatalf("CellAt(1,0) = %v, want 'i'", i)
	}

	// Empty cell after the text
	empty := e.CellAt(2, 0)
	if empty == nil {
		t.Fatal("CellAt(2,0) = nil, want empty cell")
	}

	// Out of bounds returns nil
	if c := e.CellAt(-1, 0); c != nil {
		t.Fatalf("CellAt(-1,0) = %v, want nil", c)
	}
	if c := e.CellAt(0, 999); c != nil {
		t.Fatalf("CellAt(0,999) = %v, want nil", c)
	}
}

func TestGetCells(t *testing.T) {
	e, err := New(10, 5)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	e.mu.Lock()
	e.vt.Write([]byte("AB"))
	e.mu.Unlock()

	cells := e.GetCells()
	if len(cells) != 5 {
		t.Fatalf("GetCells() returned %d rows, want 5", len(cells))
	}
	if len(cells[0]) != 10 {
		t.Fatalf("GetCells()[0] has %d columns, want 10", len(cells[0]))
	}

	if cells[0][0].Content != "A" {
		t.Errorf("cells[0][0].Content = %q, want 'A'", cells[0][0].Content)
	}
	if cells[0][1].Content != "B" {
		t.Errorf("cells[0][1].Content = %q, want 'B'", cells[0][1].Content)
	}

	// Remaining cells on row 0 should be empty
	for x := 2; x < 10; x++ {
		if c := cells[0][x].Content; c != "" && c != " " {
			t.Errorf("cells[0][%d].Content = %q, want empty", x, c)
		}
	}
}

func TestCellAtStyle(t *testing.T) {
	e, err := New(10, 5)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	// Write bold red "X"
	e.mu.Lock()
	e.vt.Write([]byte("\x1b[1;31mX\x1b[0m"))
	e.mu.Unlock()

	c := e.CellAt(0, 0)
	if c == nil {
		t.Fatal("CellAt(0,0) = nil")
	}
	if c.Content != "X" {
		t.Fatalf("Content = %q, want 'X'", c.Content)
	}
	if c.Style.Attrs&uv.AttrBold == 0 {
		t.Error("expected bold attribute set")
	}
	if c.Style.Fg == nil {
		t.Error("expected foreground color set")
	}
}

func TestRenderCellsRowCount(t *testing.T) {
	e, err := New(10, 3)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	frame := e.GetScreen()
	if len(frame.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(frame.Rows))
	}
	for i, row := range frame.Rows {
		visibleLen := ansi.StringWidth(row)
		if visibleLen != 10 {
			t.Errorf("row %d visible width = %d, want 10", i, visibleLen)
		}
	}
}

func TestRenderCellsBackgroundColor(t *testing.T) {
	e, err := New(10, 1)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	// Set a terminal background color. Empty cells should inherit it.
	e.vt.SetBackgroundColor(ansi.IndexedColor(236))

	// Write a short string so most cells remain empty.
	e.mu.Lock()
	e.vt.Write([]byte("hi"))
	e.damaged = true
	e.mu.Unlock()

	frame := e.GetScreen()
	if len(frame.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(frame.Rows))
	}
	row := frame.Rows[0]

	// The background SGR for color 236 should appear in the output,
	// applied to the empty cells that follow "hi".
	if !strings.Contains(row, "48;5;236") {
		t.Errorf("expected background color 236 in output, got: %q", row)
	}
}

func TestRenderCellsStyledContent(t *testing.T) {
	e, err := New(20, 1)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	// Write bold red text.
	e.mu.Lock()
	e.vt.Write([]byte("\x1b[1;31mhello\x1b[0m"))
	e.damaged = true
	e.mu.Unlock()

	frame := e.GetScreen()
	row := frame.Rows[0]

	if !strings.Contains(row, "hello") {
		t.Errorf("expected 'hello' in output, got: %q", row)
	}
	// SGR 1 = bold, SGR 31 = red foreground
	if !strings.Contains(row, "1") || !strings.Contains(row, "31") {
		t.Errorf("expected bold+red SGR in output, got: %q", row)
	}
}

func TestRenderCellsCachesUntouchedRows(t *testing.T) {
	e, err := New(40, 10)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer e.Close()

	// Write to row 0 only, leaving rows 1–9 untouched.
	e.mu.Lock()
	e.vt.Write([]byte("hello"))
	e.damaged = true
	e.mu.Unlock()

	frame1 := e.GetScreen()
	if !strings.Contains(frame1.Rows[0], "hello") {
		t.Fatalf("expected 'hello' in row 0, got: %q", frame1.Rows[0])
	}

	// Second render without new writes — rows should be cached.
	e.mu.Lock()
	e.damaged = true // force re-entry into renderCells
	e.mu.Unlock()

	frame2 := e.GetScreen()

	// Untouched rows must be identical across renders.
	for y := 1; y < 10; y++ {
		if frame2.Rows[y] != frame1.Rows[y] {
			t.Errorf("row %d changed unexpectedly: %q vs %q", y, frame1.Rows[y], frame2.Rows[y])
		}
	}

	// Row 0 was touched, so it's re-rendered (content should still match).
	if !strings.Contains(frame2.Rows[0], "hello") {
		t.Errorf("row 0 lost content after re-render: %q", frame2.Rows[0])
	}
}

func BenchmarkRenderCells(b *testing.B) {
	e, err := New(80, 24)
	if err != nil {
		b.Fatal(err)
	}
	defer e.Close()

	e.mu.Lock()
	for row := range 24 {
		e.vt.Write([]byte("\x1b[" + strconv.Itoa(row+1) + ";1H\x1b[1;32m" + strings.Repeat("A", 80) + "\x1b[0m"))
	}
	e.mu.Unlock()

	for b.Loop() {
		e.mu.Lock()
		e.renderCells()
		e.mu.Unlock()
	}
}

func TestCursorStateTracking(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	_, visible := e.Cursor()
	if !visible {
		t.Fatal("expected cursor visible initially")
	}
	ca := e.CursorAppearance()
	if ca.Style != CursorBlock {
		t.Errorf("initial cursor style = %d, want %d (block)", ca.Style, CursorBlock)
	}
	if !ca.Blink {
		t.Error("initial blink should be true (DEC default)")
	}
	if ca.Color != nil {
		t.Errorf("initial cursor color = %v, want nil", ca.Color)
	}

	e.mu.Lock()
	e.vt.Write([]byte("\x1b[?25l"))
	e.mu.Unlock()
	_, visible = e.Cursor()
	if visible {
		t.Fatal("expected cursor hidden after DECTCEM off")
	}

	e.mu.Lock()
	e.vt.Write([]byte("\x1b[5 q"))
	e.mu.Unlock()
	ca = e.CursorAppearance()
	if ca.Style != CursorBar {
		t.Errorf("cursor style = %d, want CursorBar", ca.Style)
	}
	if !ca.Blink {
		t.Error("expected blink=true for DECSCUSR 5")
	}

	e.mu.Lock()
	e.vt.Write([]byte("\x1b[6 q"))
	e.mu.Unlock()
	ca = e.CursorAppearance()
	if ca.Style != CursorBar {
		t.Errorf("cursor style = %d, want CursorBar", ca.Style)
	}
	if ca.Blink {
		t.Error("expected blink=false for DECSCUSR 6")
	}

	e.mu.Lock()
	e.vt.Write([]byte("\x1b]12;#ff6e63\x07"))
	e.mu.Unlock()
	ca = e.CursorAppearance()
	if ca.Color == nil {
		t.Fatal("expected non-nil cursor color after OSC 12")
	}
	r, g, b, _ := ca.Color.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	if r8 != 0xff || g8 != 0x6e || b8 != 0x63 {
		t.Errorf("cursor color = #%02x%02x%02x, want #ff6e63", r8, g8, b8)
	}

	e.mu.Lock()
	e.vt.Write([]byte("\x1b[?25h"))
	e.mu.Unlock()
	_, visible = e.Cursor()
	if !visible {
		t.Fatal("expected cursor visible after DECTCEM on")
	}
}

func TestCursorStateTrackingViaPipe(t *testing.T) {
	pr, pw := io.Pipe()
	writer := &testWriter{}

	e, err := NewFromPipes(80, 24, pr, writer)
	if err != nil {
		t.Fatalf("NewFromPipes failed: %v", err)
	}
	defer e.Close()

	batch := "\x1b[?25l\x1b[5 q\x1b]12;#ff6e63\x07Hello\x1b[1;1H\x1b[?25h"
	if _, err := pw.Write([]byte(batch)); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, visible := e.Cursor(); visible {
			if ca := e.CursorAppearance(); ca.Style == CursorBar && ca.Color != nil {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	_, visible := e.Cursor()
	if !visible {
		t.Fatal("expected cursor visible")
	}
	ca := e.CursorAppearance()
	if ca.Style != CursorBar {
		t.Errorf("cursor style = %d, want CursorBar", ca.Style)
	}
	if !ca.Blink {
		t.Error("expected blink=true")
	}
	if ca.Color == nil {
		t.Fatal("expected non-nil cursor color")
	}
	r, g, b, _ := ca.Color.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	if r8 != 0xff || g8 != 0x6e || b8 != 0x63 {
		t.Errorf("cursor color = #%02x%02x%02x, want #ff6e63", r8, g8, b8)
	}
}

func TestCursorDECSCUSRMapping(t *testing.T) {
	tests := []struct {
		param     int
		wantStyle CursorStyle
		wantBlink bool
	}{
		{0, CursorBlock, true}, {1, CursorBlock, true}, {2, CursorBlock, false},
		{3, CursorUnderline, true}, {4, CursorUnderline, false},
		{5, CursorBar, true}, {6, CursorBar, false},
	}

	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	for _, tt := range tests {
		e.mu.Lock()
		e.vt.Write([]byte(fmt.Sprintf("\x1b[%d q", tt.param)))
		e.mu.Unlock()

		ca := e.CursorAppearance()
		if ca.Style != tt.wantStyle {
			t.Errorf("DECSCUSR %d: style = %d, want %d", tt.param, ca.Style, tt.wantStyle)
		}
		if ca.Blink != tt.wantBlink {
			t.Errorf("DECSCUSR %d: blink = %v, want %v", tt.param, ca.Blink, tt.wantBlink)
		}
	}
}

func TestCursorColorReset(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	e.mu.Lock()
	e.vt.Write([]byte("\x1b]12;#ff0000\x07"))
	e.mu.Unlock()

	ca := e.CursorAppearance()
	if ca.Color == nil {
		t.Fatal("expected non-nil cursor color after OSC 12")
	}
	r, _, _, _ := ca.Color.RGBA()
	if r>>8 != 0xff {
		t.Errorf("expected red cursor, got red=%d", r>>8)
	}

	// OSC 112 resets to default (vt uses color.White, not nil).
	e.mu.Lock()
	e.vt.Write([]byte("\x1b]112\x07"))
	e.mu.Unlock()

	ca = e.CursorAppearance()
	if ca.Color == nil {
		t.Fatal("expected non-nil cursor color after OSC 112 (vt resets to default white)")
	}
	r, g, b, _ := ca.Color.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	if r8 != 0xff || g8 != 0xff || b8 != 0xff {
		t.Errorf("expected white (#ffffff) after reset, got #%02x%02x%02x", r8, g8, b8)
	}
}

func TestCursorStateAfterInterleavedHideShow(t *testing.T) {
	e, err := New(80, 24)
	if err != nil {
		t.Fatalf("failed to create emulator: %v", err)
	}
	defer e.Close()

	e.mu.Lock()
	e.vt.Write([]byte("\x1b[?25l\x1b[5 q\x1b]12;#00ff00\x07some content\x1b[?25h"))
	e.mu.Unlock()

	_, visible := e.Cursor()
	if !visible {
		t.Fatal("expected cursor visible after hide→show batch")
	}
	ca := e.CursorAppearance()
	if ca.Style != CursorBar {
		t.Errorf("style = %d, want CursorBar", ca.Style)
	}
	if !ca.Blink {
		t.Error("expected blink=true")
	}
	if ca.Color == nil {
		t.Fatal("expected non-nil color")
	}
	r, g, b, _ := ca.Color.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	if r8 != 0x00 || g8 != 0xff || b8 != 0x00 {
		t.Errorf("cursor color = #%02x%02x%02x, want #00ff00", r8, g8, b8)
	}
}

func TestCursorSequenceSplitAcrossWrites(t *testing.T) {
	pr, pw := io.Pipe()
	writer := &testWriter{}

	e, err := NewFromPipes(80, 24, pr, writer)
	if err != nil {
		t.Fatalf("NewFromPipes failed: %v", err)
	}
	defer e.Close()

	if _, err := pw.Write([]byte("\x1b[5")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := pw.Write([]byte(" q")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ca := e.CursorAppearance(); ca.Style == CursorBar {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	ca := e.CursorAppearance()
	if ca.Style != CursorBar {
		t.Errorf("split sequence: style = %d, want CursorBar", ca.Style)
	}

	if _, err := pw.Write([]byte("\x1b]12;#ab")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := pw.Write([]byte("cdef\x07")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ca := e.CursorAppearance(); ca.Color != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	ca = e.CursorAppearance()
	if ca.Color == nil {
		t.Fatal("split OSC 12: expected non-nil color")
	}
	r, g, b, _ := ca.Color.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	if r8 != 0xab || g8 != 0xcd || b8 != 0xef {
		t.Errorf("split OSC 12: color = #%02x%02x%02x, want #abcdef", r8, g8, b8)
	}
}

func TestDamageOnCursorMoveOnly(t *testing.T) {
	pr, pw := io.Pipe()
	writer := &testWriter{}

	e, err := NewFromPipes(80, 24, pr, writer)
	if err != nil {
		t.Fatalf("NewFromPipes failed: %v", err)
	}
	defer e.Close()

	if _, err := pw.Write([]byte("Hello")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(e.GetScreen().Rows, ""), "Hello") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.GetScreen() // drain remaining damage

	// Move cursor without changing content (CUP to row 1, col 10).
	if _, err := pw.Write([]byte("\x1b[1;10H")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	var frame EmittedFrame
	for time.Now().Before(deadline) {
		frame = e.GetScreen()
		if len(frame.Damage) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(frame.Damage) == 0 {
		t.Fatal("expected damage after cursor-only move")
	}
	if frame.Damage[0].Reason != CRCursor {
		t.Errorf("damage reason = %d, want CRCursor (%d)", frame.Damage[0].Reason, CRCursor)
	}

	frame = e.GetScreen()
	if len(frame.Damage) != 0 {
		t.Fatalf("expected no damage on second call, got %d", len(frame.Damage))
	}
}

func TestDamageOnCursorVisibilityChange(t *testing.T) {
	pr, pw := io.Pipe()
	writer := &testWriter{}

	e, err := NewFromPipes(80, 24, pr, writer)
	if err != nil {
		t.Fatalf("NewFromPipes failed: %v", err)
	}
	defer e.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(e.GetScreen().Damage) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := pw.Write([]byte("\x1b[?25l")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	var frame EmittedFrame
	for time.Now().Before(deadline) {
		frame = e.GetScreen()
		if len(frame.Damage) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(frame.Damage) == 0 {
		t.Fatal("expected damage after cursor visibility change")
	}
	if frame.Damage[0].Reason != CRCursor {
		t.Errorf("damage reason = %d, want CRCursor (%d)", frame.Damage[0].Reason, CRCursor)
	}
}

func TestDamageOnCursorColorChange(t *testing.T) {
	pr, pw := io.Pipe()
	writer := &testWriter{}

	e, err := NewFromPipes(80, 24, pr, writer)
	if err != nil {
		t.Fatalf("NewFromPipes failed: %v", err)
	}
	defer e.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(e.GetScreen().Damage) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := pw.Write([]byte("\x1b]12;#ff0000\x07")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	var frame EmittedFrame
	for time.Now().Before(deadline) {
		frame = e.GetScreen()
		if len(frame.Damage) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(frame.Damage) == 0 {
		t.Fatal("expected damage after cursor color change")
	}
	if frame.Damage[0].Reason != CRCursor {
		t.Errorf("damage reason = %d, want CRCursor (%d)", frame.Damage[0].Reason, CRCursor)
	}
}

func TestDamageReasonContentAndCursorChange(t *testing.T) {
	pr, pw := io.Pipe()
	writer := &testWriter{}

	e, err := NewFromPipes(80, 24, pr, writer)
	if err != nil {
		t.Fatalf("NewFromPipes failed: %v", err)
	}
	defer e.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(e.GetScreen().Damage) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := pw.Write([]byte("Hello")); err != nil {
		t.Fatalf("pipe write failed: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	var frame EmittedFrame
	for time.Now().Before(deadline) {
		frame = e.GetScreen()
		if len(frame.Damage) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(frame.Damage) == 0 {
		t.Fatal("expected damage after content+cursor change")
	}
	if frame.Damage[0].Reason != CRText {
		t.Errorf("damage reason = %d, want CRText (%d)", frame.Damage[0].Reason, CRText)
	}
}

func TestCursorStateTrackingViaPTY(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open failed: %v", err)
	}
	defer tty.Close()

	e, err := NewFromPipes(80, 24, ptmx, ptmx)
	if err != nil {
		ptmx.Close()
		t.Fatalf("NewFromPipes failed: %v", err)
	}
	defer e.Close()

	batch := "\x1b[?25l\x1b[H\x1b[2J\x1b[1;1HHello World\x1b]12;#ff6e63\x07\x1b[5 q\x1b[2;1H\x1b[?25h"
	if _, err := tty.Write([]byte(batch)); err != nil {
		t.Fatalf("tty write failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, visible := e.Cursor(); visible {
			if ca := e.CursorAppearance(); ca.Style == CursorBar && ca.Color != nil {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	_, visible := e.Cursor()
	if !visible {
		t.Fatal("expected cursor visible after DECTCEM show via PTY")
	}
	ca := e.CursorAppearance()
	if ca.Style != CursorBar {
		t.Errorf("cursor style = %d, want CursorBar", ca.Style)
	}
	if !ca.Blink {
		t.Error("expected blink=true for DECSCUSR 5")
	}
	if ca.Color == nil {
		t.Fatal("expected non-nil cursor color via PTY")
	}
	r, g, b, _ := ca.Color.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	if r8 != 0xff || g8 != 0x6e || b8 != 0x63 {
		t.Errorf("cursor color = #%02x%02x%02x, want #ff6e63", r8, g8, b8)
	}
}
