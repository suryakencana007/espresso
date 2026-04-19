# Task 7: Streaming Upload Memory Test

**Priority:** 🔵 Verification
**Estimated Effort:** 1 day
**Dependencies:** None (uses existing multipart/upload support)

## Context

Barista will support uploading backup archives for restore operations. These archives can be **large** (1 GB or more). Espresso's `Multipart[T]`, `File`, and `Files` extractors need to handle these uploads without buffering the entire file into memory.

This task verifies that Espresso's upload handling streams data incrementally, keeping memory usage constant regardless of file size.

## Acceptance Criteria

- [ ] Upload a 1 GB file with memory usage staying under 100 MB peak
- [ ] Upload handling processes chunks incrementally (not all at once)
- [ ] Behavior documented in `docs/performance.md`
- [ ] If test fails, workaround or fix documented as separate issue

## Technical Approach

### Step 7.1: Test Setup

Create a test that:

1. Generates a large temporary file (1 GB) with random data
2. Uploads it via HTTP POST multipart/form-data
3. The handler streams/processes the upload without buffering
4. Monitors memory usage during upload
5. Asserts peak memory stays under threshold

Create `tests/integration/upload_memory_test.go`:

```go
//go:build integration

package integration

import (
    "bytes"
    "context"
    "crypto/rand"
    "io"
    "mime/multipart"
    "net/http"
    "net/http/httptest"
    "os"
    "runtime"
    "sync/atomic"
    "testing"
    "time"

    "github.com/suryakencana007/espresso"
)

// TestUpload_StreamingMemory verifies that a 1 GB upload doesn't consume
// excessive memory. Memory peak should stay under 100 MB.
func TestUpload_StreamingMemory(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping memory test in short mode")
    }

    const fileSize = 1 << 30 // 1 GiB
    const memoryLimit = 100 << 20 // 100 MiB

    // Generate a temp file of random bytes
    tmpFile, err := os.CreateTemp("", "upload-test-*.bin")
    if err != nil {
        t.Fatalf("create temp file: %v", err)
    }
    defer os.Remove(tmpFile.Name())
    defer tmpFile.Close()

    t.Logf("generating %d MB test file at %s", fileSize>>20, tmpFile.Name())
    if _, err := io.CopyN(tmpFile, rand.Reader, fileSize); err != nil {
        t.Fatalf("write test file: %v", err)
    }
    if _, err := tmpFile.Seek(0, 0); err != nil {
        t.Fatalf("seek test file: %v", err)
    }

    // Track bytes processed to verify streaming works
    var bytesProcessed atomic.Int64

    router := espresso.Portafilter().
        Post("/upload", espresso.Doppio(
            func(ctx context.Context, req *espresso.Multipart[UploadReq]) (espresso.JSON[UploadRes], error) {
                // Stream-process the uploaded file
                file := req.Data.File
                buf := make([]byte, 64*1024) // 64 KB buffer
                for {
                    n, err := file.Read(buf)
                    if n > 0 {
                        bytesProcessed.Add(int64(n))
                        // Simulate some processing (e.g., checksum)
                    }
                    if err == io.EOF {
                        break
                    }
                    if err != nil {
                        return espresso.JSON[UploadRes]{}, err
                    }
                }
                return espresso.JSON[UploadRes]{
                    Data: UploadRes{BytesReceived: bytesProcessed.Load()},
                }, nil
            }))

    server := httptest.NewServer(router.HTTPHandler())
    defer server.Close()

    // Capture baseline memory
    runtime.GC()
    var baselineMem runtime.MemStats
    runtime.ReadMemStats(&baselineMem)

    // Start memory monitoring goroutine
    var peakMem atomic.Uint64
    peakMem.Store(baselineMem.Alloc)
    done := make(chan struct{})
    go func() {
        ticker := time.NewTicker(100 * time.Millisecond)
        defer ticker.Stop()
        for {
            select {
            case <-done:
                return
            case <-ticker.C:
                var m runtime.MemStats
                runtime.ReadMemStats(&m)
                for {
                    current := peakMem.Load()
                    if m.Alloc <= current {
                        break
                    }
                    if peakMem.CompareAndSwap(current, m.Alloc) {
                        break
                    }
                }
            }
        }
    }()

    // Build multipart form
    bodyReader, bodyWriter := io.Pipe()
    multipartWriter := multipart.NewWriter(bodyWriter)

    go func() {
        defer bodyWriter.Close()
        defer multipartWriter.Close()

        formFile, err := multipartWriter.CreateFormFile("file", "test.bin")
        if err != nil {
            _ = bodyWriter.CloseWithError(err)
            return
        }
        if _, err := io.Copy(formFile, tmpFile); err != nil {
            _ = bodyWriter.CloseWithError(err)
            return
        }
    }()

    // Send the upload
    req, _ := http.NewRequest("POST", server.URL+"/upload", bodyReader)
    req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

    start := time.Now()
    resp, err := http.DefaultClient.Do(req)
    uploadDuration := time.Since(start)

    close(done) // stop memory monitoring

    if err != nil {
        t.Fatalf("upload failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        t.Fatalf("unexpected status: %d", resp.StatusCode)
    }

    // Verify all bytes were processed
    if processed := bytesProcessed.Load(); processed != fileSize {
        t.Errorf("bytes processed = %d, expected %d", processed, fileSize)
    }

    peak := peakMem.Load()
    growth := peak - baselineMem.Alloc

    t.Logf("upload completed in %v", uploadDuration)
    t.Logf("memory baseline: %d MB", baselineMem.Alloc>>20)
    t.Logf("memory peak: %d MB (growth: %d MB)", peak>>20, growth>>20)

    if growth > memoryLimit {
        t.Errorf("memory peak %d MB exceeds limit %d MB",
            growth>>20, memoryLimit>>20)
    }
}

type UploadReq struct {
    File espresso.File `file:"file"`
}

type UploadRes struct {
    BytesReceived int64 `json:"bytes_received"`
}
```

### Step 7.2: Alternate Test with Raw Body

If `Multipart[T]` buffers internally (possible, depending on implementation), add a test using raw body streaming to confirm streaming *is* possible:

```go
// TestUpload_RawBodyStreaming tests streaming via RawBody extractor
// to establish a baseline for what Espresso can do.
func TestUpload_RawBodyStreaming(t *testing.T) {
    // Similar to above but using POST /upload-raw with raw body
    // and streaming read from req.Body
}
```

## Tests Required

- `TestUpload_StreamingMemory` — 1 GB multipart upload, peak memory <100 MB
- `TestUpload_RawBodyStreaming` — 1 GB raw body upload, peak memory <100 MB (baseline)

Both in integration tests, tagged `//go:build integration`.

## Documentation

Add to `docs/performance.md`:

````markdown
## Large File Uploads

Espresso handles large multipart uploads by streaming data incrementally.
File contents are not buffered fully in memory.

### Tested Behavior

- 1 GB file upload: peak memory <100 MB (measured)
- Stream-processed via `File.Read()` or `io.Copy`
- No disk spillover in the tested scenario

### Recommendations for Very Large Uploads

- Use `espresso.File` extractor and read via `Read()` in chunks
- Process data incrementally (don't `io.ReadAll` the file)
- Consider chunked upload protocols (resumable.js, tus) for >5 GB

### Example

```go
type BackupUploadReq struct {
    Archive espresso.File `file:"archive"`
    Name    string        `form:"name"`
}

func uploadBackup(ctx context.Context, req *espresso.Multipart[BackupUploadReq]) (espresso.JSON[Res], error) {
    file := req.Data.Archive
    buf := make([]byte, 64*1024)
    for {
        n, err := file.Read(buf)
        if n > 0 {
            // process chunk
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return espresso.JSON[Res]{}, err
        }
    }
    return espresso.JSON[Res]{Data: Res{Status: "ok"}}, nil
}
```
````

## Definition of Done

- [ ] Test created and passes (if Espresso supports streaming)
- [ ] OR: Test fails and failure is documented with issue filed
- [ ] Memory behavior documented in `docs/performance.md`
- [ ] Recommendations for large upload handling provided
- [ ] `CHANGELOG.md` entry

## Expected Outcomes

There are three possible outcomes for this task:

### Outcome A: Test Passes
Espresso already streams uploads correctly. Document this as a feature and close the task.

### Outcome B: Test Fails (Multipart Buffers)
If multipart buffers entire files in memory (a common implementation flaw), this is a bug. Actions:

1. File a P1 bug issue with reproduction
2. Document in `docs/performance.md` that large uploads should use `RawBody` or chunked protocols
3. Target a fix for v1.3.1 or v1.4

### Outcome C: Test Needs Workaround
If streaming works but has caveats (e.g., must use `io.Reader` directly, not buffered read):

1. Document the caveats in `docs/performance.md`
2. Add example showing the correct pattern
3. Consider if API improvements are warranted for a future release

## Notes

- This test is resource-heavy (generates 1 GB file). Don't run in short mode.
- CI runners may have limited memory — consider reducing file size if needed for CI, keeping 1 GB for local/nightly only.
- `runtime.ReadMemStats` is somewhat expensive; keep sampling interval ≥100ms.
- The `io.Pipe` pattern in the client is important: it avoids buffering the multipart body in memory before sending.
