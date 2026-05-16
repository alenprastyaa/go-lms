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
	if strings.Contains(sheetXML, "Username (otomatis)") || strings.Contains(sheetXML, "Email") || strings.Contains(sheetXML, "No. HP") {
		t.Fatal("teacher template must not include old manual columns")
	}
	if !strings.Contains(sheetXML, "Template ini hanya membutuhkan Nama Lengkap.") {
		t.Fatal("teacher template must explain that only full name is required")
	}
	if !strings.Contains(sheetXML, `<c r="A11" s="4" t="inlineStr"><is><t xml:space="preserve">No.</t></is></c>`) {
		t.Fatal("teacher template must include number header")
	}
	if !strings.Contains(sheetXML, `<c r="A12" s="4" t="inlineStr"><is><t xml:space="preserve">1</t></is></c>`) || !strings.Contains(sheetXML, `<c r="B12" s="5"/>`) {
		t.Fatal("teacher template must prefill numbered rows")
	}

	rows, err := parseTeacherImportXLSXRows(payload)
	if err != nil {
		t.Fatalf("parse generated template: %v", err)
	}
	expectedHeader := []string{"No.", "Nama Lengkap"}
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
	if strings.Contains(sheetXML, "Username (otomatis)") || strings.Contains(sheetXML, "Email Orang Tua") || strings.Contains(sheetXML, "No. HP") {
		t.Fatal("student template must not include old manual columns")
	}
	if !strings.Contains(sheetXML, "Template ini khusus untuk kelas X IPA 1") {
		t.Fatal("student template must mention selected class")
	}
	if !strings.Contains(sheetXML, "Segera minta siswa mengganti password setelah login pertama.") {
		t.Fatal("student template must remind first-login password change")
	}
	if !strings.Contains(sheetXML, `<c r="A11" s="4" t="inlineStr"><is><t xml:space="preserve">No.</t></is></c>`) {
		t.Fatal("student template must include number header")
	}
	if !strings.Contains(sheetXML, `<c r="A12" s="4" t="inlineStr"><is><t xml:space="preserve">1</t></is></c>`) || !strings.Contains(sheetXML, `<c r="B12" s="5"/>`) {
		t.Fatal("student template must prefill numbered student rows")
	}

	rows, err := parseTeacherImportXLSXRows(payload)
	if err != nil {
		t.Fatalf("parse generated template: %v", err)
	}
	expectedHeader := []string{"No.", "Nama Lengkap"}
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

func TestStudentAccountsExportXLSXIsReadable(t *testing.T) {
	payload, err := buildStudentAccountsXLSX("X IPA 1", []studentAccountExportRow{
		{FullName: "Ahmad Fajri", Username: "ahmad_fajri", Password: "Ab3kL9mN2Q"},
	})
	if err != nil {
		t.Fatalf("build accounts export: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("read xlsx zip: %v", err)
	}

	sheetXML := string(readTemplateZipFile(t, reader, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheetXML, "Daftar Akun Siswa - X IPA 1") {
		t.Fatal("student account export must mention selected class")
	}
	if !strings.Contains(sheetXML, "Ahmad Fajri") || !strings.Contains(sheetXML, "ahmad_fajri") || !strings.Contains(sheetXML, "Ab3kL9mN2Q") {
		t.Fatal("student account export must include name, username, and password")
	}
}

func TestNormalizeUsernameSeedAndUniqueness(t *testing.T) {
	existing := map[string]struct{}{
		"budi_santoso":   {},
		"budi_santoso_1": {},
	}

	if got := normalizeUsernameSeed("  Budi Santoso  "); got != "budi_santoso" {
		t.Fatalf("normalizeUsernameSeed = %q, want %q", got, "budi_santoso")
	}

	if got := nextAvailableUsername("Budi Santoso", "", existing); got != "budi_santoso_2" {
		t.Fatalf("nextAvailableUsername = %q, want %q", got, "budi_santoso_2")
	}

	if got := nextAvailableUsername("", "Ari Wibowo", map[string]struct{}{}); got != "ari_wibowo" {
		t.Fatalf("fallback username = %q, want %q", got, "ari_wibowo")
	}
}

func TestStudentImportEmptyTemplateRowIsIgnored(t *testing.T) {
	columnIndex := map[string]int{
		"full_name":    0,
		"username":     1,
		"class_name":   3,
		"parent_email": 4,
		"phone_number": 5,
	}
	row := []string{"", "", "", "X IPA 1", "", ""}

	if !isStudentImportRowEmpty(row, columnIndex) {
		t.Fatal("row with only prefilled class should be ignored by importer")
	}
}

func TestStudentImportTemplateClassValidation(t *testing.T) {
	selectedClass := models.Class{ID: 1, ClassName: "X IPA 1"}
	otherClass := models.Class{ID: 2, ClassName: "XI IPA 1"}
	payload, err := buildStudentTemplateXLSX([]models.Class{
		selectedClass,
		otherClass,
	}, &otherClass)
	if err != nil {
		t.Fatalf("build template: %v", err)
	}

	rows, err := parseTeacherImportXLSXRows(payload)
	if err != nil {
		t.Fatalf("parse template rows: %v", err)
	}

	if got := studentTemplateClassNameFromRows(rows); got != "XI IPA 1" {
		t.Fatalf("studentTemplateClassNameFromRows = %q, want %q", got, "XI IPA 1")
	}

	if err := validateStudentTemplateClass(rows, &selectedClass); err == nil {
		t.Fatal("expected mismatch validation to fail")
	} else if !strings.Contains(strings.ToLower(err.Error()), "tidak cocok") {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
}

func TestAccountPasswordEncryptDecryptRoundTrip(t *testing.T) {
	encoded, err := encryptAccountPassword("Rahasia123")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}

	decoded, err := decryptAccountPassword(encoded)
	if err != nil {
		t.Fatalf("decrypt password: %v", err)
	}

	if decoded != "Rahasia123" {
		t.Fatalf("decryptAccountPassword = %q, want %q", decoded, "Rahasia123")
	}
}

func TestAttachDecodedInitialPassword(t *testing.T) {
	encoded, err := encryptAccountPassword("Reset12345")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}

	row := map[string]interface{}{
		"initial_password_ciphertext": encoded,
	}
	attachDecodedInitialPassword(row)

	if got := row["initial_password"]; got != "Reset12345" {
		t.Fatalf("attachDecodedInitialPassword = %v, want %q", got, "Reset12345")
	}
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
