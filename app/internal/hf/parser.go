package hf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf16"
)

// artifact label -> pom property mapping per spec
var labelToPomProperty = map[string]string{
	"CMN-DOP Chart":             "cmn_dop.version",
	"Core artifacts version":    "core.version",
	"tmf622 version":            "tmf622.version",
	"CMN Common Version":        "cmn.common.version",
	"CMN Customization Version": "cmn.version",
	"CMN Snow Version":          "cmn.snow.version",
}

// ParseEML parses an .eml file content, extracts HTML and returns mapped versions
func ParseEML(filename string, r io.Reader, opts ParseOptions) (*ParsedEmail, error) {
	if opts.MaxHTMLBytes <= 0 {
		opts = DefaultParseOptions()
	}

	// Read entire message into memory (typical email sizes are small)
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read eml: %w", err)
	}

	// Some Outlook-generated EMLs (via MSG SaveAs) are UTF-16. Convert to UTF-8 if needed.
	if isUTF16(raw) {
		if utf8Bytes, convErr := utf16ToUTF8(raw); convErr == nil {
			raw = utf8Bytes
		} else {
			return nil, fmt.Errorf("decode utf16: %w", convErr)
		}
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse eml headers: %w", err)
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		// Fallback: if no content-type, try to treat whole body as text/html
		mediaType = "text/plain"
	}

	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return nil, fmt.Errorf("read eml body: %w", err)
	}

	var html string
	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			p, perr := mr.NextPart()
			if perr != nil {
				break
			}
			ct := p.Header.Get("Content-Type")
			if strings.Contains(ct, "text/html") {
				b, _ := io.ReadAll(p)
				html = string(b)
				break
			}
		}
	case strings.Contains(mediaType, "text/html"):
		html = string(body)
	default:
		// Try best-effort: sometimes html is inline in plain content
		html = string(body)
	}

	// Extract versions from HTML using relaxed regex targeting common "label:value" or table cells
	rawMappings := extractLabelVersionPairs(html)
	versions := map[string]string{}
	for label, value := range rawMappings {
		if prop, ok := labelToPomProperty[label]; ok {
			versions[prop] = value
		}
	}

	result := &ParsedEmail{
		Filename:    filename,
		Subject:     msg.Header.Get("Subject"),
		From:        msg.Header.Get("From"),
		To:          msg.Header.Get("To"),
		HTMLSnippet: snippet(html, opts.MaxHTMLBytes),
		Versions:    versions,
		RawMappings: rawMappings,
	}
	return result, nil
}

// extractLabelVersionPairs tries to find rows like "Label ... Version" in HTML
func extractLabelVersionPairs(html string) map[string]string {
	res := map[string]string{}
	if html == "" {
		return res
	}

	clean := html
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = stripTags(clean)

	// Patterns for lines like: "CMN-DOP Chart: 10.4.826-hf2503.41" or with hyphen/space separators
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?mi)^(CMN-DOP Chart)\s*[:\-]\s*([\w\.-]+)`),
		regexp.MustCompile(`(?mi)^(Core artifacts version)\s*[:\-]\s*([\w\.-]+)`),
		regexp.MustCompile(`(?mi)^(tmf622 version)\s*[:\-]\s*([\w\.-]+)`),
		regexp.MustCompile(`(?mi)^(CMN Common Version)\s*[:\-]\s*([\w\.-]+)`),
		regexp.MustCompile(`(?mi)^(CMN Customization Version)\s*[:\-]\s*([\w\.-]+)`),
		regexp.MustCompile(`(?mi)^(CMN Snow Version)\s*[:\-]\s*([\w\.-]+)`),
	}

	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatch(clean, -1) {
			label := strings.TrimSpace(m[1])
			value := strings.TrimSpace(m[2])
			if label != "" && value != "" {
				res[label] = value
			}
		}
	}

	// Also try table-like lines: Label [whitespace]+ Version
	tableRE := regexp.MustCompile(`(?mi)^(CMN\-DOP Chart|Core artifacts version|tmf622 version|CMN Common Version|CMN Customization Version|CMN Snow Version)\s+([\w\.-]+)$`)
	for _, m := range tableRE.FindAllStringSubmatch(clean, -1) {
		label := strings.TrimSpace(m[1])
		value := strings.TrimSpace(m[2])
		res[label] = value
	}

	return res
}

func stripTags(s string) string {
	// Very lightweight tag stripper; sufficient for regex scanning
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "\n")
}

// --- Encoding helpers ---
func isUTF16(b []byte) bool {
	if len(b) < 2 {
		return false
	}
	// BOM check
	if (b[0] == 0xFF && b[1] == 0xFE) || (b[0] == 0xFE && b[1] == 0xFF) {
		return true
	}
	// Heuristic: lots of NUL bytes suggests UTF-16
	nul := 0
	for i := 0; i < len(b) && i < 512; i++ {
		if b[i] == 0 {
			nul++
		}
	}
	return nul > 8
}

func utf16ToUTF8(b []byte) ([]byte, error) {
	if len(b) < 2 {
		return b, nil
	}
	bigEndian := false
	offset := 0
	if b[0] == 0xFE && b[1] == 0xFF {
		bigEndian = true
		offset = 2
	}
	if b[0] == 0xFF && b[1] == 0xFE {
		bigEndian = false
		offset = 2
	}
	// Build uint16 slice
	u16 := make([]uint16, 0, (len(b)-offset)/2)
	for i := offset; i+1 < len(b); i += 2 {
		var v uint16
		if bigEndian {
			v = binary.BigEndian.Uint16(b[i : i+2])
		} else {
			v = binary.LittleEndian.Uint16(b[i : i+2])
		}
		u16 = append(u16, v)
	}
	runes := utf16.Decode(u16)
	return []byte(string(runes)), nil
}

func snippet(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
