package hf

import "time"

// ParsedEmail represents the result of parsing an HF adoption .eml file
type ParsedEmail struct {
	// Original file metadata
	Filename   string            `json:"filename,omitempty"`
	ReceivedAt *time.Time        `json:"receivedAt,omitempty"`
	Subject    string            `json:"subject,omitempty"`
	From       string            `json:"from,omitempty"`
	To         string            `json:"to,omitempty"`

	// HTML extracted for debugging/inspection
	HTMLSnippet string `json:"htmlSnippet,omitempty"`

	// Extracted versions mapped to pom.xml property names
	Versions map[string]string `json:"versions"`

	// Raw label->version pairs as found in the email table
	RawMappings map[string]string `json:"rawMappings,omitempty"`
}

// ParseOptions provides optional controls for the parser behavior
type ParseOptions struct {
	MaxHTMLBytes int
}

// DefaultParseOptions returns sane defaults
func DefaultParseOptions() ParseOptions {
	return ParseOptions{MaxHTMLBytes: 64 * 1024}
}


