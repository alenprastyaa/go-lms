package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const questionImageMarker = "[[QUESTION_IMAGE_URL]]"

type QuestionBankAIInput struct {
	SubjectName            string
	ClassName              string
	GradeLabel             string
	PhaseName              string
	CurriculumName         string
	Topic                  string
	QuestionType           string
	QuestionCount          int
	Difficulty             string
	IncludeIllustration    bool
	AdditionalInstructions string
}

type CurriculumTopicSuggestionInput struct {
	SubjectName    string
	ClassName      string
	GradeLabel     string
	PhaseName      string
	CurriculumName string
}

type QuestionBankAIItem struct {
	QuestionType       string   `json:"question_type"`
	QuestionText       string   `json:"question_text"`
	QuestionImageURL   string   `json:"question_image_url,omitempty"`
	IllustrationPrompt string   `json:"illustration_prompt,omitempty"`
	Options            []string `json:"options"`
	CorrectOption      *int     `json:"correct_option"`
	Rubric             *string  `json:"rubric"`
}

type curriculumTopicSuggestionResponse struct {
	Topics []string `json:"topics"`
}

func GenerateCurriculumTopicSuggestionsWithGemini(input CurriculumTopicSuggestionInput) ([]string, error) {
	fallbackTopics := buildFallbackCurriculumTopicSuggestions(input)
	prompt := buildCurriculumTopicSuggestionPrompt(input)
	text, err := callHuggingFace(prompt, "Anda adalah asisten kurikulum sekolah Indonesia yang hanya mengembalikan JSON valid tanpa markdown.", 0.3)
	if err != nil {
		return fallbackTopics, nil
	}

	extracted := extractJSONObject(text)
	if !json.Valid([]byte(extracted)) {
		return fallbackTopics, nil
	}

	var parsed curriculumTopicSuggestionResponse
	if err := json.Unmarshal([]byte(extracted), &parsed); err != nil {
		return fallbackTopics, nil
	}

	topics := normalizeTopicSuggestions(parsed.Topics)
	if len(topics) == 0 {
		return fallbackTopics, nil
	}
	return topics, nil
}

func GenerateCurriculumTopicSuggestionsWithHuggingFace(input CurriculumTopicSuggestionInput) ([]string, error) {
	return GenerateCurriculumTopicSuggestionsWithGemini(input)
}

func buildCurriculumTopicSuggestionPrompt(input CurriculumTopicSuggestionInput) string {
	curriculum := fallbackText(input.CurriculumName, "Kurikulum Merdeka")
	parts := []string{
		"Buat daftar materi/topik pembelajaran yang lazim dan relevan untuk guru membuat soal.",
		"Gunakan Bahasa Indonesia yang singkat dan mudah dipahami guru.",
		fmt.Sprintf("Kurikulum: %s.", curriculum),
		fmt.Sprintf("Mapel: %s.", fallbackText(input.SubjectName, "-")),
		fmt.Sprintf("Kelas: %s.", fallbackText(input.ClassName, "-")),
	}

	if strings.TrimSpace(input.GradeLabel) != "" {
		parts = append(parts, fmt.Sprintf("Jenjang/kelas: %s.", strings.TrimSpace(input.GradeLabel)))
	}
	if strings.TrimSpace(input.PhaseName) != "" {
		parts = append(parts, fmt.Sprintf("Fase: %s.", strings.TrimSpace(input.PhaseName)))
	}

	parts = append(parts,
		"Daftar harus berupa materi yang bisa langsung dipilih sebagai topik pembuatan soal.",
		"Jangan terlalu teknis, jangan terlalu panjang, dan jangan membuat capaian pembelajaran berbentuk paragraf.",
		"Hasilkan 8 sampai 12 topik.",
		"Kembalikan JSON saja tanpa markdown.",
		`Struktur JSON wajib: {"topics":["Materi 1","Materi 2"]}`,
	)

	return strings.Join(parts, "\n")
}

func normalizeTopicSuggestions(items []string) []string {
	topics := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		topic := strings.TrimSpace(item)
		if topic == "" {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(topic), " "))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		topics = append(topics, topic)
		if len(topics) >= 12 {
			break
		}
	}
	return topics
}

func buildFallbackCurriculumTopicSuggestions(input CurriculumTopicSuggestionInput) []string {
	subjectName := fallbackText(input.SubjectName, "Mata Pelajaran")
	category := inferTeachingModuleSubjectCategory(subjectName)

	categoryTopics := map[string][]string{
		"Numerasi": {
			"Bilangan dan operasi hitung",
			"Persamaan dan pertidaksamaan",
			"Fungsi dan grafik",
			"Geometri dan pengukuran",
			"Statistika dasar",
			"Peluang sederhana",
			"Pemecahan masalah kontekstual",
			"Perbandingan dan skala",
		},
		"Sains": {
			"Pengukuran dan besaran",
			"Zat dan perubahannya",
			"Gaya, gerak, dan energi",
			"Sistem makhluk hidup",
			"Ekosistem dan lingkungan",
			"Gelombang dan bunyi",
			"Listrik dan rangkaian sederhana",
			"Metode ilmiah dan eksperimen",
		},
		"Bahasa": {
			"Teks deskripsi dan observasi",
			"Teks narasi dan cerita",
			"Teks argumentasi",
			"Gagasan pokok dan simpulan",
			"Ejaan dan kalimat efektif",
			"Berbicara dan presentasi",
			"Membaca kritis",
			"Menulis terstruktur",
		},
		"Sosial Humaniora": {
			"Interaksi sosial",
			"Kegiatan ekonomi",
			"Perubahan sosial budaya",
			"Sejarah peristiwa penting",
			"Peta, wilayah, dan keruangan",
			"Lembaga sosial dan pemerintahan",
			"Hak dan kewajiban warga negara",
			"Masalah sosial di lingkungan sekitar",
		},
		"Teknologi": {
			"Algoritma dan logika dasar",
			"Pemrograman dasar",
			"Data dan representasi informasi",
			"Jaringan komputer dasar",
			"Sistem komputer",
			"Keamanan digital",
			"Antarmuka dan pengalaman pengguna",
			"Proyek teknologi sederhana",
		},
		"Keagamaan": {
			"Nilai keimanan dan akhlak",
			"Ibadah dalam kehidupan sehari-hari",
			"Keteladanan tokoh",
			"Adab dan etika pergaulan",
			"Pemahaman ayat atau hadis pilihan",
			"Toleransi dan moderasi",
			"Tanggung jawab sebagai pelajar",
			"Refleksi nilai keagamaan",
		},
		"Seni": {
			"Unsur seni dan estetika",
			"Apresiasi karya seni",
			"Karya dua dimensi",
			"Karya tiga dimensi",
			"Musik dan ritme dasar",
			"Ekspresi kreatif",
			"Proses berkarya",
			"Presentasi hasil karya",
		},
		"Olahraga": {
			"Kebugaran jasmani",
			"Gerak dasar",
			"Permainan bola besar",
			"Permainan bola kecil",
			"Atletik dasar",
			"Senam dan kelentukan",
			"Pola hidup sehat",
			"Sportivitas dan kerja sama",
		},
		"Kejuruan": {
			"Keselamatan dan kesehatan kerja",
			"Dasar proses kerja kejuruan",
			"Penggunaan alat dan bahan",
			"Standar prosedur operasional",
			"Dokumentasi hasil kerja",
			"Pelayanan atau kualitas produk",
			"Troubleshooting dasar",
			"Proyek praktik sederhana",
		},
		"Umum": {
			fmt.Sprintf("Konsep dasar %s", subjectName),
			fmt.Sprintf("Penerapan %s dalam konteks sehari-hari", subjectName),
			fmt.Sprintf("Latihan dan penguatan %s", subjectName),
			fmt.Sprintf("Analisis materi inti %s", subjectName),
			fmt.Sprintf("Evaluasi pemahaman %s", subjectName),
			fmt.Sprintf("Proyek sederhana %s", subjectName),
			fmt.Sprintf("Diskusi topik penting %s", subjectName),
			fmt.Sprintf("Refleksi pembelajaran %s", subjectName),
		},
	}

	topics := categoryTopics[category]
	if len(topics) == 0 {
		topics = categoryTopics["Umum"]
	}

	grade := strings.TrimSpace(input.GradeLabel)
	if grade != "" {
		adapted := make([]string, 0, len(topics))
		for _, topic := range topics {
			adapted = append(adapted, fmt.Sprintf("%s (%s)", topic, grade))
		}
		topics = adapted
	}

	return normalizeTopicSuggestions(topics)
}

func buildQuestionBankPrompt(input QuestionBankAIInput) string {
	normalizedType := normalizeQuestionType(input.QuestionType)
	parts := []string{
		"Anda adalah penyusun bank soal untuk LMS sekolah.",
		"Buat soal dalam Bahasa Indonesia yang jelas, natural, dan sesuai konteks sekolah.",
		"Kalimat soal harus terdengar seperti soal buku latihan atau ujian yang ditulis guru, bukan terjemahan literal mesin.",
		"Hindari frasa kaku, berulang, atau janggal seperti 'untuk x ... per hari dimodelkan oleh' jika ada bentuk yang lebih natural.",
		"Untuk soal matematika atau kontekstual, pilih narasi yang mengalir alami, misalnya 'Jika x ...' atau 'Biaya ... untuk memproduksi x ... adalah ...'.",
		"Jangan mengulang satuan atau keterangan waktu/tempat secara berlebihan dalam satu kalimat.",
		"Jangan gunakan format LaTeX atau tanda dolar. Tulis notasi matematika dengan teks biasa yang tetap mudah dibaca, misalnya 'pangkat', 'akar', atau 'dibagi'.",
		"Gunakan gaya ringkas. Batasi panjang: question_text <= 220 karakter, setiap opsi <= 70 karakter, rubric (jika essay) <= 200 karakter.",
		fmt.Sprintf("Mapel: %s.", fallbackText(input.SubjectName, "-")),
		fmt.Sprintf("Kelas: %s.", fallbackText(input.ClassName, "-")),
	}

	if strings.TrimSpace(input.GradeLabel) != "" {
		parts = append(parts, fmt.Sprintf("Jenjang/kelas target tambahan: %s.", strings.TrimSpace(input.GradeLabel)))
	}
	if strings.TrimSpace(input.PhaseName) != "" {
		parts = append(parts, fmt.Sprintf("Fase belajar: %s.", strings.TrimSpace(input.PhaseName)))
	}
	if strings.TrimSpace(input.CurriculumName) != "" {
		parts = append(parts, fmt.Sprintf("Kurikulum: %s.", strings.TrimSpace(input.CurriculumName)))
	}

	parts = append(parts,
		fmt.Sprintf("Topik: %s.", strings.TrimSpace(input.Topic)),
		fmt.Sprintf("Tipe soal: %s.", normalizedType),
		fmt.Sprintf("Jumlah soal: %d.", input.QuestionCount),
		fmt.Sprintf("Wajib hasilkan tepat %d soal, tidak boleh kurang dan tidak boleh lebih.", input.QuestionCount),
		fmt.Sprintf("Tingkat kesulitan: %s.", fallbackText(strings.TrimSpace(input.Difficulty), "MENENGAH")),
	)

	if strings.TrimSpace(input.AdditionalInstructions) != "" {
		parts = append(parts, fmt.Sprintf("Instruksi tambahan: %s.", strings.TrimSpace(input.AdditionalInstructions)))
	}

	if input.IncludeIllustration {
		parts = append(parts,
			"Walaupun ilustrasi dibuat terpisah, question_text harus tetap bahasa Indonesia yang natural, rapi, dan tidak berubah kualitasnya.",
			"Jangan menulis instruksi ilustrasi, caption gambar, atau prompt visual ke dalam question_text.",
			"Gunakan notasi matematika yang bersih dan mudah dibaca. Jangan menulis LaTeX mentah seperti \\frac, \\theta, atau bentuk pecahan yang tidak dirender.",
		)
	}

	if normalizedType == "MCQ" {
		parts = append(parts, "Setiap soal MCQ harus punya tepat 5 opsi jawaban dari A sampai E dan satu correct_option berbasis indeks 0 sampai 4.")
	} else {
		parts = append(parts, "Setiap soal essay harus memiliki rubric singkat untuk membantu penilaian guru.")
	}

	parts = append(parts,
		"Kembalikan JSON saja tanpa markdown, tanpa penjelasan tambahan.",
		"Struktur JSON wajib:",
		fmt.Sprintf(`{"items":[{"question_type":"%s","question_text":"...","options":["..."],"correct_option":0,"rubric":null}]}`, normalizedType),
		"Untuk ESSAY, isi options dengan null dan correct_option dengan null.",
	)

	return strings.Join(parts, "\n")
}

func normalizeQuestionType(value string) string {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if upper == "ESSAY" {
		return "ESSAY"
	}
	return "MCQ"
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func GenerateQuestionBankItemsWithGemini(input QuestionBankAIInput) ([]QuestionBankAIItem, error) {
	if input.QuestionCount <= 0 {
		input.QuestionCount = 5
	}

	systemMessage := "Anda adalah penyusun bank soal sekolah. Kembalikan JSON valid saja tanpa markdown, tanpa penjelasan tambahan."

	// Default: 1 request untuk N soal (lebih hemat prompt_tokens dibanding batch).
	// Jika JSON invalid/terpotong, fallback ke batch yang lebih kecil.
	target := input.QuestionCount
	for _, batch := range []int{target, minInt(5, target), minInt(3, target), 1} {
		if batch <= 0 {
			continue
		}
		batchInput := input
		batchInput.QuestionCount = batch
		prompt := buildQuestionBankPrompt(batchInput)

		items, err := generateQuestionBankItemsWithGemini(prompt, systemMessage, input.QuestionType, batch)
		if err != nil {
			// Try smaller batch when JSON is invalid or truncated.
			if strings.Contains(strings.ToLower(err.Error()), "json") {
				continue
			}
			return nil, err
		}

		seen := make(map[string]bool, len(items))
		unique := appendUniqueQuestionBankItems(nil, items, seen, batch, 0)
		if len(unique) < batch {
			// Incomplete; try smaller batch.
			continue
		}

		unique = populateQuestionBankIllustrations(batchInput, unique)
		return unique, nil
	}

	return nil, fmt.Errorf("AI gagal menghasilkan JSON bank soal yang valid. Coba generate ulang.")
}

func GenerateQuestionBankItemsWithHuggingFace(input QuestionBankAIInput) ([]QuestionBankAIItem, error) {
	return GenerateQuestionBankItemsWithGemini(input)
}

func generateQuestionBankItemsWithGemini(prompt, systemMessage, questionType string, questionCount int) ([]QuestionBankAIItem, error) {
	_ = questionCount
	text, err := callHuggingFace(prompt, systemMessage, 0.7)
	if err != nil {
		return nil, err
	}

	extracted := extractJSONObject(text)
	if !json.Valid([]byte(extracted)) {
		return nil, fmt.Errorf("hasil Gemini tidak bisa diparsing sebagai JSON bank soal: JSON tidak valid")
	}

	items, err := parseQuestionBankItemsFromJSON(extracted, questionType)
	if err != nil {
		return nil, fmt.Errorf("hasil Gemini tidak bisa diparsing sebagai JSON bank soal: %w", err)
	}

	return items, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func appendUniqueQuestionBankItems(dst, src []QuestionBankAIItem, seen map[string]bool, limit int, maxPerBatch int) []QuestionBankAIItem {
	appended := 0
	for _, item := range src {
		if len(dst) >= limit {
			break
		}
		if maxPerBatch > 0 && appended >= maxPerBatch {
			break
		}
		key := normalizeQuestionIdentity(item.QuestionText)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		dst = append(dst, item)
		appended++
	}
	return dst
}

func normalizeQuestionIdentity(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func extractJSONObject(raw string) string {
	direct := strings.TrimSpace(raw)
	if direct == "" {
		return ""
	}

	if candidate := firstBalancedJSONObject(direct); candidate != "" {
		return candidate
	}

	if fenced := fencedJSON(direct); fenced != "" {
		if candidate := firstBalancedJSONObject(fenced); candidate != "" {
			return candidate
		}
		return fenced
	}

	return direct
}

func firstBalancedJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for index := start; index < len(raw); index++ {
		char := raw[index]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inString = false
			}
			continue
		}

		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(raw[start : index+1])
			}
		}
	}

	return ""
}

func fencedJSON(raw string) string {
	start := strings.Index(strings.ToLower(raw), "```json")
	if start < 0 {
		start = strings.Index(raw, "```")
		if start < 0 {
			return ""
		}
	}

	candidate := raw[start:]
	candidate = strings.TrimPrefix(candidate, "```json")
	candidate = strings.TrimPrefix(candidate, "```")
	candidate = strings.TrimSpace(candidate)
	if end := strings.LastIndex(candidate, "```"); end >= 0 {
		candidate = candidate[:end]
	}
	return strings.TrimSpace(candidate)
}

func normalizeQuestionBankItems(items []map[string]interface{}, questionType string) []QuestionBankAIItem {
	normalizedType := normalizeQuestionType(questionType)
	normalized := make([]QuestionBankAIItem, 0, len(items))

	for _, item := range items {
		questionText := coalesceString(
			item["question_text"],
			item["question"],
			item["text"],
			item["soal"],
			item["prompt"],
		)
		if questionText == "" {
			continue
		}

		if normalizedType == "MCQ" {
			options := normalizeOptions(5, item["options"], item["choices"], item["answers"])
			correctOption, ok := normalizeCorrectOption(
				item["correct_option"],
				item["correctAnswer"],
				item["correct_answer"],
				item["answer"],
				item["jawaban_benar"],
				options,
			)
			if len(options) < 2 || !ok || correctOption < 0 || correctOption >= len(options) {
				continue
			}

			correct := correctOption
			rubric := (*string)(nil)
			illustrationPrompt := coalesceString(item["illustration_prompt"], item["image_prompt"], item["visual_prompt"])

			normalized = append(normalized, QuestionBankAIItem{
				QuestionType:       "MCQ",
				QuestionText:       normalizeQuestionText(questionText),
				QuestionImageURL:   coalesceString(item["question_image_url"], item["image_url"]),
				IllustrationPrompt: illustrationPrompt,
				Options:            options,
				CorrectOption:      &correct,
				Rubric:             rubric,
			})
			continue
		}

		rubric := coalesceString(item["rubric"], item["scoring_rubric"], item["answer_guideline"], item["guideline"])
		if rubric == "" {
			rubric = "Jawaban dinilai berdasarkan ketepatan konsep, kelengkapan penjelasan, dan kejelasan alasan."
		}
		illustrationPrompt := coalesceString(item["illustration_prompt"], item["image_prompt"], item["visual_prompt"])

		normalized = append(normalized, QuestionBankAIItem{
			QuestionType:       "ESSAY",
			QuestionText:       normalizeQuestionText(questionText),
			QuestionImageURL:   coalesceString(item["question_image_url"], item["image_url"]),
			IllustrationPrompt: illustrationPrompt,
			Options:            nil,
			CorrectOption:      nil,
			Rubric:             &rubric,
		})
	}

	return normalized
}

func normalizeQuestionText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}

	text = strings.Join(strings.Fields(text), " ")
	text = strings.ReplaceAll(text, "$", "")
	replacements := []struct {
		pattern *regexp.Regexp
		repl    string
	}{
		{
			pattern: regexp.MustCompile(`(?i)\b([a-z0-9)\]])\^\{([^}]+)\}`),
			repl:    `$1 pangkat ($2)`,
		},
		{
			pattern: regexp.MustCompile(`(?i)\b([a-z0-9)\]])\^([a-z0-9+\-().]+)`),
			repl:    `$1 pangkat $2`,
		},
		{
			pattern: regexp.MustCompile(`(?i)\buntuk x ([^,.!?;:]+?) per hari dimodelkan oleh fungsi\b`),
			repl:    "per hari untuk memproduksi x $1 dinyatakan oleh fungsi",
		},
		{
			pattern: regexp.MustCompile(`(?i)\buntuk x ([^,.!?;:]+?) per hari dimodelkan oleh\b`),
			repl:    "per hari untuk memproduksi x $1 dinyatakan oleh",
		},
		{
			pattern: regexp.MustCompile(`(?i)\bdalam rupiah\) untuk x ([^,.!?;:]+?) per hari\b`),
			repl:    "dalam rupiah) per hari untuk memproduksi x $1",
		},
	}

	for _, item := range replacements {
		text = item.pattern.ReplaceAllString(text, item.repl)
	}

	text = strings.NewReplacer(
		"  ", " ",
		" ,", ",",
		" .", ".",
		" :", ":",
		" ;", ";",
		" ?", "?",
		" !", "!",
	).Replace(text)

	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))

	return strings.TrimSpace(text)
}

func parseQuestionBankItemsFromJSON(raw string, questionType string) ([]QuestionBankAIItem, error) {
	var payload interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	items := extractQuestionBankItemMaps(payload)
	return normalizeQuestionBankItems(items, questionType), nil
}

func extractQuestionBankItemMaps(payload interface{}) []map[string]interface{} {
	switch value := payload.(type) {
	case []interface{}:
		return interfaceSliceToMaps(value)
	case map[string]interface{}:
		for _, key := range []string{"items", "questions", "data", "results"} {
			if nested, ok := value[key]; ok {
				if maps := extractQuestionBankItemMaps(nested); len(maps) > 0 {
					return maps
				}
			}
		}
		return []map[string]interface{}{value}
	default:
		return nil
	}
}

func interfaceSliceToMaps(items []interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if row, ok := item.(map[string]interface{}); ok {
			out = append(out, row)
		}
	}
	return out
}

func coalesceString(values ...interface{}) string {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				return text
			}
		case fmt.Stringer:
			if text := strings.TrimSpace(typed.String()); text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizeOptions(limit int, values ...interface{}) []string {
	for _, value := range values {
		if options := optionsFromAny(value, limit); len(options) > 0 {
			return options
		}
	}
	return nil
}

func optionsFromAny(value interface{}, limit int) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case []interface{}:
		options := make([]string, 0, len(typed))
		for _, item := range typed {
			text := optionText(item)
			if text != "" {
				options = append(options, text)
			}
		}
		return trimAndLimitStrings(options, limit)
	case []string:
		return trimAndLimitStrings(typed, limit)
	case map[string]interface{}:
		orderedKeys := []string{"A", "B", "C", "D", "E", "a", "b", "c", "d", "e", "1", "2", "3", "4", "5"}
		options := make([]string, 0, len(typed))
		used := make(map[string]bool, len(typed))
		for _, key := range orderedKeys {
			if raw, ok := typed[key]; ok {
				text := cleanOptionLabel(fmt.Sprint(raw))
				if text != "" {
					options = append(options, text)
					used[key] = true
				}
			}
		}
		if len(options) == 0 {
			for key, raw := range typed {
				if used[key] {
					continue
				}
				text := cleanOptionLabel(fmt.Sprint(raw))
				if text != "" {
					options = append(options, text)
				}
			}
		}
		return trimAndLimitStrings(options, limit)
	case string:
		lines := splitOptionLines(typed)
		return trimAndLimitStrings(lines, limit)
	default:
		return nil
	}
}

func optionText(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return cleanOptionLabel(typed)
	case map[string]interface{}:
		return coalesceString(
			typed["text"],
			typed["option_text"],
			typed["label"],
			typed["value"],
			typed["content"],
		)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func cleanOptionLabel(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	if len(text) >= 3 {
		prefix := strings.ToUpper(strings.TrimSpace(text[:2]))
		if (prefix[0] >= 'A' && prefix[0] <= 'E') && prefix[1] == '.' {
			return strings.TrimSpace(text[2:])
		}
	}
	if len(text) >= 2 {
		prefix := strings.ToUpper(strings.TrimSpace(text[:1]))
		if prefix[0] >= 'A' && prefix[0] <= 'E' {
			next := text[1]
			if next == ')' || next == ':' {
				return strings.TrimSpace(text[2:])
			}
		}
	}
	return text
}

func splitOptionLines(value string) []string {
	rawLines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		text := cleanOptionLabel(line)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func normalizeCorrectOption(values ...interface{}) (int, bool) {
	if len(values) == 0 {
		return 0, false
	}
	options, _ := values[len(values)-1].([]string)
	for _, value := range values[:len(values)-1] {
		if index, ok := correctOptionFromAny(value, options); ok {
			return index, true
		}
	}
	return 0, false
}

func correctOptionFromAny(value interface{}, options []string) (int, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
			return parsed, true
		}
		upper := strings.ToUpper(text)
		if len(upper) == 1 && upper[0] >= 'A' && upper[0] <= 'E' {
			return int(upper[0] - 'A'), true
		}
		cleaned := cleanOptionLabel(text)
		for index, option := range options {
			if strings.EqualFold(strings.TrimSpace(option), strings.TrimSpace(cleaned)) {
				return index, true
			}
		}
		return 0, false
	case map[string]interface{}:
		return correctOptionFromAny(
			coalesceString(typed["text"], typed["label"], typed["value"], typed["answer"]),
			options,
		)
	default:
		return correctOptionFromAny(fmt.Sprint(value), options)
	}
}

func normalizeStringArray(raw json.RawMessage, limit int) []string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || strings.EqualFold(string(raw), "null") {
		return nil
	}

	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		return trimAndLimitStrings(values, limit)
	}

	var interfaces []interface{}
	if err := json.Unmarshal(raw, &interfaces); err == nil {
		values = make([]string, 0, len(interfaces))
		for _, item := range interfaces {
			values = append(values, fmt.Sprint(item))
		}
		return trimAndLimitStrings(values, limit)
	}

	return nil
}

func trimAndLimitStrings(values []string, limit int) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func parseIntegerFromRawMessage(raw json.RawMessage) (int, bool) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || strings.EqualFold(string(raw), "null") {
		return 0, false
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return int(number), true
	}

	var integer int
	if err := json.Unmarshal(raw, &integer); err == nil {
		return integer, true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		var parsed int
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(text), "%d", &parsed); scanErr == nil {
			return parsed, true
		}
	}

	return 0, false
}

func bytesTrimSpace(raw json.RawMessage) json.RawMessage {
	return json.RawMessage(strings.TrimSpace(string(raw)))
}
