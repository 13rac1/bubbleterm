## WIP: Goroutine leak in pipe-based emulator Close()

### Problem

When a pipe-based emulator (created via `NewFromPipes`) is closed, two
goroutines leak:

1. **`ptyReadLoop`** blocks on `source.Read(buf)`. The `stopChan` check is
   non-blocking and only runs between iterations, so if `Read` is blocked
   waiting for data, the goroutine never exits. Closing `stopChan` does not
   unblock `io.Pipe.Read` or any other blocking reader.

2. **`pipeResponseLoop`** blocks on `e.vt.Read(buf)`. The vt emulator's
   internal pipe is never closed, so this goroutine also hangs indefinitely.

In PTY mode this is not an issue because closing the PTY fd causes the blocked
`Read` to return an error.

### Rejected approach: change `NewFromPipes` to require `io.ReadCloser`

The most direct fix is to change the `r` parameter from `io.Reader` to
`io.ReadCloser` and close it in `Close()`. This works but is a **breaking API
change** — any caller passing a plain `io.Reader` (e.g. `strings.NewReader`,
wrapped streams) would stop compiling.

### Alternative approaches to consider

1. **Type-assert to `io.Closer` at close time.** Keep the `io.Reader`
   signature. In `Close()`, check `if c, ok := e.reader.(io.Closer); ok {
   c.Close() }`. Callers that pass an `io.ReadCloser` (like `io.Pipe`) get
   clean shutdown automatically. Callers with non-closable readers get the
   same behavior as today (leak until GC). This is a non-breaking incremental
   improvement.

2. **Wrap the reader in an internal `io.Pipe`.** In `NewFromPipes`, spawn a
   goroutine that copies from the caller's reader into an internal `io.Pipe`.
   `Close()` closes the internal pipe writer, which unblocks `ptyReadLoop`.
   The copy goroutine exits when the caller's reader returns EOF or when the
   internal pipe is closed. This adds one goroutine but guarantees clean
   shutdown regardless of the reader type.

3. **Use a cancelable read wrapper.** Create a wrapper that selects between
   the reader and a done channel. This is tricky because `io.Reader.Read` is
   not select-friendly, but can be done with a goroutine-per-read pattern
   (which just moves the leak to the wrapper).

4. **Document that callers must close the reader.** Add to `NewFromPipes`
   docs: "The caller must close `r` after calling `Close()` to ensure all
   internal goroutines exit." This is the simplest change but puts the burden
   on callers and is easy to forget.

### Recommendation

Option 1 (type-assert) is the least disruptive and handles the common case
(`io.Pipe`, `os.File`, `exec.Cmd` pipes all implement `io.Closer`). Combined
with option 4 (documenting the requirement), it covers both the happy path
and the edge case without breaking the API.
