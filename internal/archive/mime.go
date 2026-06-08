package archive

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message/mail"
)

type ParseResult struct {
	BodyText    string
	BodyHTML    string
	Attachments []AttachmentMeta
}

type AttachmentMeta struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	PartPath    string `json:"part_path"`
}

func ParseEmail(raw []byte) (*ParseResult, error) {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return &ParseResult{}, fmt.Errorf("create mail reader: %w", err)
	}

	res := &ParseResult{}
	partIdx := 0

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		partIdx++
		partPath := fmt.Sprintf("%d", partIdx)

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			body, err := io.ReadAll(p.Body)
			if err != nil {
				continue
			}
			switch strings.ToLower(ct) {
			case "text/plain":
				if res.BodyText == "" {
					res.BodyText = string(body)
				}
			case "text/html":
				if res.BodyHTML == "" {
					res.BodyHTML = string(body)
				}
			}

		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			ct, params, _ := h.ContentType()
			if filename == "" {
				filename = params["name"]
			}
			body, err := io.ReadAll(p.Body)
			if err != nil {
				continue
			}
			res.Attachments = append(res.Attachments, AttachmentMeta{
				Filename:    filename,
				ContentType: ct,
				SizeBytes:   int64(len(body)),
				PartPath:    partPath,
			})
		}
	}

	return res, nil
}

// ExtractAttachment re-parses raw RFC822 bytes and returns the decoded body of
// the attachment at the given partPath (e.g. "3"). Returns an error when the
// part is not found.
func ExtractAttachment(raw []byte, partPath string) ([]byte, string, error) {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return nil, "", err
	}

	partIdx := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		partIdx++
		if fmt.Sprintf("%d", partIdx) != partPath {
			_, _ = io.Copy(io.Discard, p.Body)
			continue
		}

		body, err := io.ReadAll(p.Body)
		if err != nil {
			return nil, "", err
		}

		ct := "application/octet-stream"
		if ah, ok := p.Header.(*mail.AttachmentHeader); ok {
			ct, _, _ = ah.ContentType()
		} else if ih, ok := p.Header.(*mail.InlineHeader); ok {
			ct, _, _ = ih.ContentType()
		}
		return body, ct, nil
	}
	return nil, "", fmt.Errorf("part %s not found", partPath)
}

func BodyPreview(text string, n int) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n])
}
