package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

func extractText(filePath, filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return extractPDF(filePath)
	case ".docx":
		return extractDOCX(filePath)
	default:
		b, err := os.ReadFile(filePath)
		return string(b), err
	}
}

func extractPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}

func extractDOCX(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open DOCX: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return xmlText(data), nil
	}
	return "", fmt.Errorf("word/document.xml not found in DOCX")
}

// xmlText strips XML tags and returns plain text, adding newlines at paragraph boundaries.
func xmlText(data []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.StartElement:
			if t.Name.Local == "p" {
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}
