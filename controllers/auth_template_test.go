package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"lms/models"
)

func TestTeacherImportTemplateXLSXIsReadable(t *testing.T) {
	payload, err := buildTeacherTemplateXLSX()
	if err != nil {
		t.Fatalf("build template: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("read xlsx zip: %v", err)
	}

	for _, path := range []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"xl/workbook.xml",
		"xl/_rels/workbook.xml.rels",
		"xl/styles.xml",
		"xl/worksheets/sheet1.xml",
	} {
		data := readTemplateZipFile(t, reader, path)
		if err := xml.Unmarshal(data, new(interface{})); err != nil {
			t.Fatalf("%s is not valid XML: %v", path, err)
		}
	}

	sheetXML := string(readTemplateZipFile(t, reader, "xl/worksheets/sheet1.xml"))
	autoFilterIndex := strings.Index(sheetXML, "<autoFilter")
	mergeCellsIndex := strings.Index(sheetXML, "<mergeCells")
	if autoFilterIndex < 0 || mergeCellsIndex < 0 || autoFilterIndex > mergeCellsIndex {
		t.Fatal("worksheet elements must place autoFilter before mergeCells for Excel compatibility")
	}
	if !strings.Contains(sheetXML, `<col min="5" max="5" width="22" style="8" customWidth="1"/>`) {
		t.Fatal("phone number column must use text style to preserve leading zeroes")
	}

	stylesXML := string(readTemplateZipFile(t, reader, "xl/styles.xml"))
	if !strings.Contains(stylesXML, `<xf numFmtId="49"`) {
		t.Fatal("template styles must include text number format for phone numbers")
	}

	rows, err := parseTeacherImportXLSXRows(payload)
	if err != nil {
		t.Fatalf("parse generated template: %v", err)
	}
	expectedHeader := []string{"Nama Lengkap", "Username", "Password", "Email", "No. HP"}
	for _, row := range rows {
		if len(row) < len(expectedHeader) {
			continue
		}
		matches := true
		for index, expected := range expectedHeader {
			if cellValue(row, index) != expected {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	t.Fatalf("expected import header %v in generated template rows", expectedHeader)
}

func TestStudentImportTemplateXLSXIsReadable(t *testing.T) {
	selectedClass := models.Class{ID: 1, ClassName: "X IPA 1"}
	payload, err := buildStudentTemplateXLSX([]models.Class{
		selectedClass,
		{ID: 2, ClassName: "XI IPA 1"},
	}, &selectedClass)
	if err != nil {
		t.Fatalf("build template: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("read xlsx zip: %v", err)
	}

	sheetXML := string(readTemplateZipFile(t, reader, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheetXML, `<col min="6" max="6" width="22" style="8" customWidth="1"/>`) {
		t.Fatal("student phone number column must use text style to preserve leading zeroes")
	}
	if !strings.Contains(sheetXML, "X IPA 1") || !strings.Contains(sheetXML, "XI IPA 1") {
		t.Fatal("student template must include available class names")
	}
	if !strings.Contains(sheetXML, `<c r="D12" s="5" t="inlineStr"><is><t xml:space="preserve">X IPA 1</t></is></c>`) {
		t.Fatal("student template must prefill selected class in input rows")
	}

	rows, err := parseTeacherImportXLSXRows(payload)
	if err != nil {
		t.Fatalf("parse generated template: %v", err)
	}
	expectedHeader := []string{"Nama Lengkap", "Username", "Password", "Kelas", "Email Orang Tua", "No. HP"}
	for _, row := range rows {
		if len(row) < len(expectedHeader) {
			continue
		}
		matches := true
		for index, expected := range expectedHeader {
			if cellValue(row, index) != expected {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	t.Fatalf("expected import header %v in generated student template rows", expectedHeader)
}

func readTemplateZipFile(t *testing.T, reader *zip.Reader, path string) []byte {
	t.Helper()
	for _, file := range reader.File {
		if file.Name != path {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		defer handle.Close()
		data, err := io.ReadAll(handle)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return data
	}
	t.Fatalf("zip file %s not found", path)
	return nil
}
