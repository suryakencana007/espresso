---
title: File Upload
description: Handle file uploads with Multipart extractor
---

# File Upload Example

This example shows how to handle file uploads using Espresso's Multipart extractor.

## Basic File Upload

### Single File Upload

```go
package main

import (
    "context"
    "net/http"
    
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
)

func main() {
    router := espresso.Portafilter()
    
    // Single file upload
    router.Post("/upload", espresso.Doppio(uploadFile))
    
    router.Brew(espresso.WithAddr(":8080"))
}

func uploadFile(ctx context.Context, req *extractor.File) (espresso.JSON[UploadResponse], error) {
    return espresso.JSON[UploadResponse]{
        Data: UploadResponse{
            Filename: req.File.Filename,
            Size:     req.File.Size,
        },
    }, nil
}

type UploadResponse struct {
    Filename string `json:"filename"`
    Size     int64  `json:"size"`
}
```

### Multiple File Upload

```go
func uploadFiles(ctx context.Context, req *extractor.Files) (espresso.JSON[MultiUploadResponse], error) {
    filenames := make([]string, 0, len(req.Files))
    for _, f := range req.Files {
        filenames = append(filenames, f.Filename)
    }
    
    return espresso.JSON[MultiUploadResponse]{
        Data: MultiUploadResponse{
            Count:     len(req.Files),
            Filenames: filenames,
        },
    }, nil
}

type MultiUploadResponse struct {
    Count     int      `json:"count"`
    Filenames []string `json:"filenames"`
}

// Route
router.Post("/upload/multiple", espresso.Doppio(uploadFiles))
```

## File Upload with Form Data

### Multipart Form

```go
type UploadForm struct {
    Title       string          `form:"title"`
    Description string          `form:"description"`
    Filename    string          `file:"document"`
}

func uploadWithMetadata(ctx context.Context, req *extractor.Multipart[UploadForm]) (espresso.JSON[DocumentResponse], error) {
    return espresso.JSON[DocumentResponse]{
        Data: DocumentResponse{
            Title:      req.Data.Title,
            Filename:   req.Data.Filename,
            CreatedAt:  time.Now(),
        },
    }, nil
}

type DocumentResponse struct {
    Title     string    `json:"title"`
    Filename  string    `json:"filename"`
    CreatedAt time.Time `json:"created_at"`
}

// Route
router.Post("/documents", espresso.Doppio(uploadWithMetadata))
```

### Access File Content

```go
func uploadWithContent(ctx context.Context, req *espresso.JSON[FileRequest]) (espresso.JSON[FileResponse], error) {
    // File content is available after extraction
    // Note: extractor.File only provides metadata
    // For content, use r.FormFile("file") in handler
    
    return espresso.JSON[FileResponse]{
        Data: FileResponse{
            Status: "uploaded",
        },
    }, nil
}
```

## Client Examples

### curl

```bash
# Single file
curl -X POST http://localhost:8080/upload \
    -F "file=@/path/to/document.pdf"

# Multiple files
curl -X POST http://localhost:8080/upload/multiple \
    -F "files=@/path/to/file1.pdf" \
    -F "files=@/path/to/file2.pdf"

# Multipart with metadata
curl -X POST http://localhost:8080/documents \
    -F "title=My Document" \
    -F "description=Important file" \
    -F "document=@/path/to/document.pdf"
```

### JavaScript (Fetch)

```javascript
// Single file upload
async function uploadFile(file) {
    const formData = new FormData();
    formData.append('file', file);
    
    const response = await fetch('http://localhost:8080/upload', {
        method: 'POST',
        body: formData,
    });
    
    return response.json();
}

// Multiple files
async function uploadFiles(files) {
    const formData = new FormData();
    for (const file of files) {
        formData.append('files', file);
    }
    
    const response = await fetch('http://localhost:8080/upload/multiple', {
        method: 'POST',
        body: formData,
    });
    
    return response.json();
}

// Multipart with metadata
async function uploadDocument(file, metadata) {
    const formData = new FormData();
    formData.append('title', metadata.title);
    formData.append('description', metadata.description);
    formData.append('document', file);
    
    const response = await fetch('http://localhost:8080/documents', {
        method: 'POST',
        body: formData,
    });
    
    return response.json();
}
```

## Error Handling

### File Size Limit

```go
func uploadWithLimit(ctx context.Context, req *extractor.File) (espresso.JSON[UploadResponse], error) {
    // Check file size (example: max 10MB)
    const maxSize = 10 * 1024 * 1024
    if req.File.Size > maxSize {
        return espresso.JSON[UploadResponse]{}, fmt.Errorf("file too large: max %d bytes", maxSize)
    }
    
    return espresso.JSON[UploadResponse]{
        Data: UploadResponse{
            Filename: req.File.Filename,
            Size:     req.File.Size,
        },
    }, nil
}
```

### File Type Validation

```go
func uploadWithValidation(ctx context.Context, req *extractor.File) (espresso.JSON[UploadResponse], error) {
    // Validate file extension
    allowedTypes := map[string]bool{
        ".pdf":  true,
        ".doc":  true,
        ".docx": true,
        ".txt":  true,
    }
    
    ext := filepath.Ext(req.File.Filename)
    if !allowedTypes[ext] {
        return espresso.JSON[UploadResponse]{}, fmt.Errorf("file type %s not allowed", ext)
    }
    
    return espresso.JSON[UploadResponse]{
        Data: UploadResponse{
            Filename: req.File.Filename,
            Size:     req.File.Size,
        },
    }, nil
}
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "path/filepath"
    
    "github.com/suryakencana007/espresso"
    "github.com/suryakencana007/espresso/extractor"
)

type UploadResponse struct {
    Filename string `json:"filename"`
    Size     int64  `json:"size"`
    Type     string `json:"type"`
}

func main() {
    router := espresso.Portafilter()
    
    // Single file upload
    router.Post("/upload", espresso.Doppio(uploadSingle))
    
    // Multiple files
    router.Post("/upload/multiple", espresso.Doppio(uploadMultiple))
    
    // Multipart with metadata
    router.Post("/documents", espresso.Doppio(uploadDocument))
    
    fmt.Println("Server starting on :8080")
    router.Brew(espresso.WithAddr(":8080"))
}

func uploadSingle(ctx context.Context, req *extractor.File) (espresso.JSON[UploadResponse], error) {
    return espresso.JSON[UploadResponse]{
        Data: UploadResponse{
            Filename: req.File.Filename,
            Size:     req.File.Size,
            Type:     filepath.Ext(req.File.Filename),
        },
    }, nil
}

func uploadMultiple(ctx context.Context, req *extractor.Files) (espresso.JSON[map[string]any], error) {
    files := make([]map[string]any, 0, len(req.Files))
    for _, f := range req.Files {
        files = append(files, map[string]any{
            "filename": f.Filename,
            "size":      f.Size,
        })
    }
    
    return espresso.JSON[map[string]any]{
        Data: map[string]any{
            "count": len(req.Files),
            "files": files,
        },
    }, nil
}

type DocumentForm struct {
    Title       string `form:"title"`
    Description string `form:"description"`
    Filename    string `file:"document"`
}

func uploadDocument(ctx context.Context, req *extractor.Multipart[DocumentForm]) (espresso.JSON[map[string]any], error) {
    return espresso.JSON[map[string]any]{
        Data: map[string]any{
            "title":      req.Data.Title,
            "description": req.Data.Description,
            "filename":   req.Data.Filename,
        },
    }, nil
}
```

## See Also

- [Extractors Guide](/guide/extractors) - All extractor types
- [Response Types Guide](/guide/response) - Response handling
- [Production Example](/examples/production) - Production setup