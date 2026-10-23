// Package artifact describes binary results that agents can consume directly
// when the protocol supports it.
package artifact

// Image is binary image data with its MIME type.
type Image struct {
	MIMEType string
	Data     []byte
}

// ImageSource is implemented by action results that carry images. The MCP
// server attaches each image as protocol image content while keeping the
// structured result free of raw bytes.
type ImageSource interface {
	Images() []Image
}
