// Package mimetype is a stub implementation satisfying the go-playground/validator dependency.
package mimetype

import "io"

// MIME represents a detected MIME type.
type MIME struct {
	mime string
}

func (m *MIME) String() string {
	return m.mime
}

// DetectReader detects the MIME type from r.
func DetectReader(r io.Reader) (*MIME, error) {
	return &MIME{mime: "application/octet-stream"}, nil
}
