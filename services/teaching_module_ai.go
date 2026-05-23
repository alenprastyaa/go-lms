package services

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TeachingModuleAIInput struct {
	SubjectName            string
	ClassName              string
	GradeLabel             string
	PhaseName              string
	CurriculumName         string
	Title                  string
	Topic                  string
	CPReference            string
	LearningObjectives     string
	MaterialScope          string
	TimeAllocation         string
	Meetings               int
	StudentCharacteristics string
	Facilities             string
	PancasilaProfile       string
	LearningModel          string
	AdditionalInstructions string
}

type TeachingModuleAIDraft struct {
	Title              string                           `json:"title"`
	Identity           TeachingModuleIdentity           `json:"identity"`
	GeneralInformation TeachingModuleGeneralInformation `json:"general_information"`
	CoreComponents     TeachingModuleCoreComponents     `json:"core_components"`
	Attachments        TeachingModuleAttachments        `json:"attachments"`
	Notes              string                           `json:"notes"`
}

type TeachingModuleSuggestionInput struct {
	SubjectName    string
	ClassName      string
	GradeLabel     string
	PhaseName      string
	CurriculumName string
	Topic          string
}

type TeachingModuleSuggestionCatalog struct {
	SubjectCategory           string   `json:"subject_category"`
	Titles                    []string `json:"titles"`
	Topics                    []string `json:"topics"`
	CPReferences              []string `json:"cp_references"`
	LearningObjectives        []string `json:"learning_objectives"`
	MaterialScopes            []string `json:"material_scopes"`
	StudentCharacteristics    []string `json:"student_characteristics"`
	Facilities                []string `json:"facilities"`
	AdditionalInstructionTips []string `json:"additional_instruction_tips"`
}

type TeachingModuleIdentity struct {
	SubjectName    string `json:"subject_name"`
	ClassName      string `json:"class_name"`
	GradeLabel     string `json:"grade_label"`
	PhaseName      string `json:"phase_name"`
	CurriculumName string `json:"curriculum_name"`
	Topic          string `json:"topic"`
	Title          string `json:"title"`
	TimeAllocation string `json:"time_allocation"`
	Meetings       int    `json:"meetings"`
}

type TeachingModuleGeneralInformation struct {
	CompetencyPrerequisites []string `json:"competency_prerequisites"`
	PancasilaProfile        []string `json:"pancasila_profile"`
	Facilities              []string `json:"facilities"`
	TargetLearners          string   `json:"target_learners"`
	LearningModel           string   `json:"learning_model"`
	LearningApproach        string   `json:"learning_approach"`
}

type TeachingModuleCoreComponents struct {
	CPReference             string                           `json:"cp_reference"`
	LearningObjectives      []string                         `json:"learning_objectives"`
	AchievementIndicators   []string                         `json:"achievement_indicators"`
	ProjectOutputs          []string                         `json:"project_outputs"`
	MeaningfulUnderstanding []string                         `json:"meaningful_understanding"`
	TriggerQuestions        []string                         `json:"trigger_questions"`
	LearningActivities      TeachingModuleLearningActivities `json:"learning_activities"`
	Assessments             TeachingModuleAssessments        `json:"assessments"`
	Differentiation         TeachingModuleDifferentiation    `json:"differentiation"`
	Remedial                []string                         `json:"remedial"`
	Enrichment              []string                         `json:"enrichment"`
	StudentReflection       []string                         `json:"student_reflection"`
	TeacherReflection       []string                         `json:"teacher_reflection"`
}

type TeachingModuleLearningActivities struct {
	Introduction []string `json:"introduction"`
	Core         []string `json:"core"`
	Closing      []string `json:"closing"`
}

type TeachingModuleAssessments struct {
	Diagnostic []string `json:"diagnostic"`
	Formative  []string `json:"formative"`
	Summative  []string `json:"summative"`
	Rubric     []string `json:"rubric"`
}

type TeachingModuleDifferentiation struct {
	Content []string `json:"content"`
	Process []string `json:"process"`
	Product []string `json:"product"`
}

type TeachingModuleAttachments struct {
	StudentWorksheet []string `json:"student_worksheet"`
	ReadingMaterials []string `json:"reading_materials"`
	Glossary         []string `json:"glossary"`
	Bibliography     []string `json:"bibliography"`
}

func buildTeachingModulePrompt(input TeachingModuleAIInput) string {
	curriculum := fallbackText(input.CurriculumName, "Kurikulum Merdeka")
	gradeLabel := fallbackText(input.GradeLabel, input.ClassName)
	phaseName := fallbackText(input.PhaseName, "Fase belum ditentukan")
	title := fallbackText(input.Title, input.Topic)

	parts := []string{
		"Anda adalah asisten guru Indonesia yang menyusun draft Modul Ajar sesuai prinsip Kurikulum Merdeka.",
		"Gunakan Bahasa Indonesia formal, praktis, dan langsung bisa dipakai guru untuk mengajar.",
		"Jangan menulis format RPP lama, jangan membuat narasi promosi, dan jangan keluar dari struktur modul ajar.",
		fmt.Sprintf("Kurikulum: %s.", curriculum),
		fmt.Sprintf("Mata pelajaran: %s.", fallbackText(input.SubjectName, "-")),
		fmt.Sprintf("Kelas/rombel: %s.", fallbackText(input.ClassName, "-")),
		fmt.Sprintf("Jenjang/kelas target: %s.", gradeLabel),
		fmt.Sprintf("Fase: %s.", phaseName),
		fmt.Sprintf("Judul modul: %s.", title),
		fmt.Sprintf("Topik utama: %s.", strings.TrimSpace(input.Topic)),
		fmt.Sprintf("Alokasi waktu: %s.", fallbackText(input.TimeAllocation, "-")),
		fmt.Sprintf("Jumlah pertemuan: %d.", input.Meetings),
		fmt.Sprintf("Acuan CP/ATP dari guru: %s.", strings.TrimSpace(input.CPReference)),
		fmt.Sprintf("Tujuan pembelajaran dari guru: %s.", strings.TrimSpace(input.LearningObjectives)),
		fmt.Sprintf("Cakupan materi: %s.", strings.TrimSpace(input.MaterialScope)),
	}

	if strings.TrimSpace(input.StudentCharacteristics) != "" {
		parts = append(parts, fmt.Sprintf("Karakteristik peserta didik: %s.", strings.TrimSpace(input.StudentCharacteristics)))
	}
	if strings.TrimSpace(input.Facilities) != "" {
		parts = append(parts, fmt.Sprintf("Sarana dan prasarana yang tersedia: %s.", strings.TrimSpace(input.Facilities)))
	}
	if strings.TrimSpace(input.PancasilaProfile) != "" {
		parts = append(parts, fmt.Sprintf("Fokus Profil Pelajar Pancasila: %s.", strings.TrimSpace(input.PancasilaProfile)))
	}
	if strings.TrimSpace(input.LearningModel) != "" {
		parts = append(parts, fmt.Sprintf("Model pembelajaran yang diprioritaskan: %s.", strings.TrimSpace(input.LearningModel)))
	}
	if strings.TrimSpace(input.AdditionalInstructions) != "" {
		parts = append(parts, fmt.Sprintf("Instruksi tambahan guru: %s.", strings.TrimSpace(input.AdditionalInstructions)))
	}

	parts = append(parts,
		"Wajib susun output sebagai draft modul ajar yang memuat identitas, informasi umum, komponen inti, dan lampiran.",
		"CP / ATP tidak boleh generik. Tulis spesifik sesuai mapel, topik, dan konteks kelas, seolah merujuk capaian/topik resmi yang operasional.",
		"Tujuan pembelajaran wajib terukur, dapat diamati, dan memakai kata kerja operasional seperti mengidentifikasi, mendemonstrasikan, menyusun, mempresentasikan, atau mengevaluasi.",
		"Setiap tujuan pembelajaran harus menunjukkan output atau performa yang jelas, bukan hanya 'memahami' atau 'mengetahui'.",
		"Profil Pelajar Pancasila hanya boleh memakai dimensi resmi: Beriman, bertakwa kepada Tuhan YME, dan berakhlak mulia; Berkebinekaan global; Gotong royong; Mandiri; Bernalar kritis; Kreatif.",
		"Untuk mapel berbasis praktik atau performa, kegiatan inti wajib memuat demonstrasi, simulasi, praktik, roleplay, presentasi, umpan balik, dan perbaikan performa.",
		"Untuk DKV, multimedia, desain, produksi konten digital, editing video/audio, atau industri kreatif, wajib gunakan konteks workflow produksi, brief, storyboard, branding visual, aset digital, editing, publikasi, portofolio, dan rubrik produk kreatif.",
		"Jika topik memuat 'konsep dasar', 'pengertian', 'unsur', 'fungsi', atau pengenalan komunikasi industri kreatif, jangan membuat proyek produksi penuh. Fokuskan pada pemahaman konsep, analisis contoh, fungsi komunikasi visual, unsur komunikasi, media, audiens, dan contoh brand sederhana.",
		"Jika aktivitas berupa produksi/editing/proyek portofolio penuh, alokasi waktu harus realistis minimal 3 pertemuan. Untuk topik konsep dasar, aktivitas cukup analisis, diskusi, LKPD, peta konsep, dan presentasi singkat.",
		"Asesmen wajib konkret. Sertakan indikator atau rubrik penilaian yang relevan terhadap performa atau produk belajar, bukan narasi umum saja.",
		"Wajib isi achievement_indicators dengan indikator ketercapaian tujuan yang dapat diamati dan dinilai.",
		"Wajib isi project_outputs dengan output proyek yang spesifik sesuai topik, format produk, dan bukti portofolio.",
		"Wajib isi assessments.rubric dengan kriteria rubrik detail skala 1 sampai 4 atau deskripsi tingkat capaian.",
		"Setiap daftar wajib cukup kaya isi, bukan satu kalimat pendek saja.",
		"Jumlah minimal item yang diharapkan: competency_prerequisites 3 item, learning_objectives 4 item, meaningful_understanding 3 item, trigger_questions 3 item.",
		"learning_activities.introduction minimal 4 langkah, core minimal 6 langkah, closing minimal 3 langkah.",
		"assessments.diagnostic minimal 3 item, formative minimal 4 item, summative minimal 3 item.",
		"differentiation.content, process, product masing-masing minimal 3 item.",
		"remedial minimal 3 item, enrichment minimal 3 item, student_reflection minimal 3 item, teacher_reflection minimal 3 item.",
		"student_worksheet, reading_materials, glossary, bibliography masing-masing minimal 3 item.",
		"Asesmen harus dibagi ke diagnostik, formatif, dan sumatif.",
		"Kegiatan pembelajaran harus dibagi ke pendahuluan, inti, dan penutup.",
		"Pembelajaran berdiferensiasi harus konkret pada aspek konten, proses, dan produk.",
		"Jaga agar isi tetap realistis untuk kelas dan waktu yang diberikan guru.",
		"Jangan menyalin ulang instruksi ini. Jangan gunakan markdown. Jangan gunakan tabel.",
		"Kembalikan JSON valid saja dengan struktur berikut:",
		`{"title":"...","identity":{"subject_name":"...","class_name":"...","grade_label":"...","phase_name":"...","curriculum_name":"Kurikulum Merdeka","topic":"...","title":"...","time_allocation":"...","meetings":1},"general_information":{"competency_prerequisites":["..."],"pancasila_profile":["..."],"facilities":["..."],"target_learners":"...","learning_model":"...","learning_approach":"..."},"core_components":{"cp_reference":"...","learning_objectives":["..."],"achievement_indicators":["..."],"project_outputs":["..."],"meaningful_understanding":["..."],"trigger_questions":["..."],"learning_activities":{"introduction":["..."],"core":["..."],"closing":["..."]},"assessments":{"diagnostic":["..."],"formative":["..."],"summative":["..."],"rubric":["..."]},"differentiation":{"content":["..."],"process":["..."],"product":["..."]},"remedial":["..."],"enrichment":["..."],"student_reflection":["..."],"teacher_reflection":["..."]},"attachments":{"student_worksheet":["..."],"reading_materials":["..."],"glossary":["..."],"bibliography":["..."]},"notes":"..."}`,
	)

	return strings.Join(parts, "\n")
}

func GenerateTeachingModuleDraftWithHuggingFace(input TeachingModuleAIInput) (*TeachingModuleAIDraft, error) {
	if input.Meetings <= 0 {
		input.Meetings = 1
	}
	if strings.TrimSpace(input.CurriculumName) == "" {
		input.CurriculumName = "Kurikulum Merdeka"
	}

	fallbackDraft := buildFallbackTeachingModuleDraft(input)

	text, err := callOpenRouterText(
		"teaching-module-draft",
		buildTeachingModulePrompt(input),
		"Anda adalah penyusun modul ajar Kurikulum Merdeka dan wajib mengembalikan JSON valid tanpa markdown.",
		0.5,
		8000,
	)
	if err != nil {
		return &fallbackDraft, nil
	}

	var parsed TeachingModuleAIDraft
	if err := json.Unmarshal([]byte(extractJSONObject(text)), &parsed); err != nil {
		return &fallbackDraft, nil
	}

	normalized := normalizeTeachingModuleDraft(parsed, input)
	return &normalized, nil
}

func buildTeachingModuleSuggestionPrompt(input TeachingModuleSuggestionInput) string {
	curriculum := fallbackText(input.CurriculumName, "Kurikulum Merdeka")
	gradeLabel := fallbackText(input.GradeLabel, input.ClassName)
	phaseName := fallbackText(input.PhaseName, "Fase belum ditentukan")
	subjectCategory := inferTeachingModuleSubjectCategory(input.SubjectName)

	parts := []string{
		"Anda adalah asisten guru Indonesia yang membuat katalog pilihan dropdown untuk menyusun Modul Ajar Kurikulum Merdeka.",
		"Semua hasil harus ringkas, realistis, dan relevan dengan mapel.",
		fmt.Sprintf("Mapel: %s.", fallbackText(input.SubjectName, "-")),
		fmt.Sprintf("Kategori mapel: %s.", subjectCategory),
		fmt.Sprintf("Kelas/rombel: %s.", fallbackText(input.ClassName, "-")),
		fmt.Sprintf("Jenjang/kelas target: %s.", gradeLabel),
		fmt.Sprintf("Fase: %s.", phaseName),
		fmt.Sprintf("Kurikulum: %s.", curriculum),
	}

	if strings.TrimSpace(input.Topic) != "" {
		parts = append(parts, fmt.Sprintf("Topik yang sedang dipilih guru: %s.", strings.TrimSpace(input.Topic)))
	}

	parts = append(parts,
		"Buat katalog saran untuk dropdown yang bisa langsung dipilih guru.",
		"Setiap daftar berisi 5 sampai 8 item, singkat, natural, dan sesuai konteks mapel.",
		"cp_references harus berupa ringkasan acuan CP/ATP yang masuk akal untuk topik/mapel terkait, bukan kode resmi palsu.",
		"learning_objectives harus berupa kalimat tujuan pembelajaran yang bisa langsung dipakai guru.",
		"material_scopes harus berupa cakupan materi singkat yang konkret.",
		"student_characteristics harus berupa karakteristik kelas yang realistis.",
		"facilities harus berupa sarana prasarana yang lazim tersedia atau wajar dibutuhkan.",
		"additional_instruction_tips harus berupa instruksi singkat yang dapat dipilih guru untuk mengarahkan gaya modul.",
		"Jangan gunakan markdown. Jangan gunakan tabel. Kembalikan JSON valid saja.",
		`{"subject_category":"...","titles":["..."],"topics":["..."],"cp_references":["..."],"learning_objectives":["..."],"material_scopes":["..."],"student_characteristics":["..."],"facilities":["..."],"additional_instruction_tips":["..."]}`,
	)

	return strings.Join(parts, "\n")
}

func GenerateTeachingModuleSuggestionsWithHuggingFace(input TeachingModuleSuggestionInput) (*TeachingModuleSuggestionCatalog, error) {
	if strings.TrimSpace(input.CurriculumName) == "" {
		input.CurriculumName = "Kurikulum Merdeka"
	}

	fallbackCatalog := buildFallbackTeachingModuleSuggestions(input)

	text, err := callOpenRouterText(
		"teaching-module-suggestions",
		buildTeachingModuleSuggestionPrompt(input),
		"Anda adalah asisten guru yang membuat katalog dropdown modul ajar dan wajib mengembalikan JSON valid tanpa markdown.",
		0.4,
		2500,
	)
	if err != nil {
		return &fallbackCatalog, nil
	}

	var parsed TeachingModuleSuggestionCatalog
	if err := json.Unmarshal([]byte(extractJSONObject(text)), &parsed); err != nil {
		return &fallbackCatalog, nil
	}

	normalized := normalizeTeachingModuleSuggestions(parsed, input)
	return &normalized, nil
}

func normalizeTeachingModuleDraft(parsed TeachingModuleAIDraft, input TeachingModuleAIInput) TeachingModuleAIDraft {
	title := fallbackText(parsed.Title, fallbackText(parsed.Identity.Title, fallbackText(input.Title, input.Topic)))
	category := inferTeachingModuleSubjectCategory(input.SubjectName)
	instructionMode := inferTeachingModuleInstructionMode(input.SubjectName, input.Topic)

	identity := parsed.Identity
	identity.SubjectName = fallbackText(identity.SubjectName, input.SubjectName)
	identity.ClassName = fallbackText(identity.ClassName, input.ClassName)
	identity.GradeLabel = fallbackText(identity.GradeLabel, input.GradeLabel)
	identity.PhaseName = fallbackText(identity.PhaseName, input.PhaseName)
	identity.CurriculumName = fallbackText(identity.CurriculumName, input.CurriculumName)
	identity.Topic = fallbackText(identity.Topic, input.Topic)
	identity.Title = fallbackText(identity.Title, title)
	identity.TimeAllocation = fallbackText(identity.TimeAllocation, input.TimeAllocation)
	if identity.Meetings <= 0 {
		identity.Meetings = input.Meetings
	}
	identity = normalizeTeachingModuleSchedule(identity, instructionMode)

	general := parsed.GeneralInformation
	general.CompetencyPrerequisites = ensureMinTeachingModuleList(general.CompetencyPrerequisites, buildCompetencyPrerequisites(input.Topic, instructionMode), 3, 6)
	general.PancasilaProfile = normalizePancasilaProfile(general.PancasilaProfile, splitTeachingModuleInput(input.PancasilaProfile, defaultPancasilaProfile(instructionMode)), instructionMode)
	general.Facilities = normalizeTeachingModuleFacilities(general.Facilities, splitTeachingModuleInput(input.Facilities, buildFacilities(input.Topic, instructionMode)), instructionMode)
	general.TargetLearners = fallbackText(general.TargetLearners, fallbackText(input.StudentCharacteristics, "Peserta didik reguler dengan kebutuhan belajar yang beragam."))
	general.LearningModel = fallbackText(general.LearningModel, fallbackText(input.LearningModel, "Pembelajaran berdiferensiasi"))
	general.LearningApproach = fallbackText(general.LearningApproach, "Pembelajaran aktif, kontekstual, dan berpusat pada peserta didik.")

	core := parsed.CoreComponents
	core.CPReference = fallbackText(core.CPReference, buildSpecificCPReference(input, category, instructionMode))
	core.LearningObjectives = ensureMinTeachingModuleList(core.LearningObjectives, splitTeachingModuleInput(input.LearningObjectives, buildLearningObjectives(input, category, instructionMode)), 4, 8)
	core.AchievementIndicators = ensureMinTeachingModuleList(core.AchievementIndicators, buildAchievementIndicators(input.Topic, instructionMode), 4, 8)
	core.ProjectOutputs = ensureMinTeachingModuleList(core.ProjectOutputs, buildProjectOutputs(input.Topic, instructionMode), 3, 8)
	core.MeaningfulUnderstanding = ensureMinTeachingModuleList(core.MeaningfulUnderstanding, buildMeaningfulUnderstanding(input.Topic, category, instructionMode), 3, 6)
	core.TriggerQuestions = ensureMinTeachingModuleList(core.TriggerQuestions, buildTriggerQuestions(input.Topic, category, instructionMode), 3, 6)
	core.LearningActivities.Introduction = ensureMinTeachingModuleList(core.LearningActivities.Introduction, []string{
		"Guru membuka pembelajaran, menyampaikan tujuan, dan mengaitkan topik dengan pengalaman peserta didik.",
		"Guru memeriksa kesiapan belajar dan pengetahuan awal peserta didik melalui pertanyaan pemantik.",
		"Guru menyampaikan alur kegiatan dan hasil belajar yang diharapkan.",
		"Peserta didik diarahkan menyiapkan alat, bahan, atau catatan belajar yang diperlukan.",
	}, 4, 6)
	core.LearningActivities.Core = ensureSpecificTeachingModuleList(core.LearningActivities.Core, buildCoreActivities(input.Topic, category, instructionMode), input.Topic, instructionMode, 6, 10)
	core.LearningActivities.Closing = ensureMinTeachingModuleList(core.LearningActivities.Closing, []string{
		"Guru bersama peserta didik menyimpulkan pembelajaran dan menegaskan tindak lanjut.",
		"Peserta didik merefleksikan bagian yang sudah dipahami dan yang masih perlu dilatih.",
		"Guru menyampaikan tindak lanjut berupa pengayaan, remedial, atau tugas lanjutan.",
	}, 3, 6)
	core.Assessments.Diagnostic = ensureMinTeachingModuleList(core.Assessments.Diagnostic, buildDiagnosticAssessments(input.Topic, instructionMode), 3, 5)
	core.Assessments.Formative = ensureMinTeachingModuleList(core.Assessments.Formative, buildFormativeAssessments(input.Topic, instructionMode), 4, 6)
	core.Assessments.Summative = ensureMinTeachingModuleList(core.Assessments.Summative, buildSummativeAssessments(input.Topic, instructionMode), 3, 5)
	core.Assessments.Rubric = ensureMinTeachingModuleList(core.Assessments.Rubric, buildRubricCriteria(input.Topic, instructionMode), 4, 8)
	core.Differentiation.Content = ensureMinTeachingModuleList(core.Differentiation.Content, buildDifferentiationContent(input.Topic, instructionMode), 3, 5)
	core.Differentiation.Process = ensureMinTeachingModuleList(core.Differentiation.Process, buildDifferentiationProcess(input.Topic, instructionMode), 3, 5)
	core.Differentiation.Product = ensureMinTeachingModuleList(core.Differentiation.Product, buildDifferentiationProduct(input.Topic, instructionMode), 3, 5)
	core.Remedial = ensureMinTeachingModuleList(core.Remedial, []string{
		"Berikan bimbingan ulang pada konsep yang belum tuntas, disertai contoh bertahap dan latihan tambahan.",
		"Gunakan soal atau tugas dengan tingkat kesulitan lebih rendah untuk penguatan dasar.",
		"Pendampingan dilakukan melalui penjelasan ulang, latihan terarah, dan refleksi singkat.",
	}, 3, 5)
	core.Enrichment = ensureMinTeachingModuleList(core.Enrichment, []string{
		"Berikan tantangan lanjutan atau penerapan konsep pada konteks yang lebih luas bagi peserta didik yang sudah tuntas.",
		"Peserta didik diberi tugas lanjutan yang menuntut analisis atau penerapan lebih dalam.",
		"Guru menyediakan aktivitas eksplorasi tambahan untuk memperluas wawasan peserta didik.",
	}, 3, 5)
	core.StudentReflection = ensureMinTeachingModuleList(core.StudentReflection, []string{
		"Konsep apa yang paling saya pahami hari ini?",
		"Bagian mana yang masih perlu saya latih lagi?",
		"Strategi belajar apa yang paling membantu saya memahami materi?",
	}, 3, 6)
	core.TeacherReflection = ensureMinTeachingModuleList(core.TeacherReflection, []string{
		"Bagian pembelajaran mana yang paling efektif dan mana yang perlu diperbaiki pada pertemuan berikutnya?",
		"Apakah aktivitas belajar sudah sesuai dengan tujuan pembelajaran dan karakteristik peserta didik?",
		"Aspek apa yang perlu diperkuat pada media, asesmen, atau pendampingan guru?",
	}, 3, 6)

	attachments := parsed.Attachments
	attachments.StudentWorksheet = ensureMinTeachingModuleList(attachments.StudentWorksheet, []string{
		"Lembar kerja singkat yang memandu peserta didik mengerjakan latihan inti sesuai topik.",
		"Tugas terstruktur yang meminta peserta didik menerapkan konsep utama secara bertahap.",
		"Latihan reflektif atau pertanyaan penguatan untuk mengecek pemahaman akhir.",
	}, 3, 6)
	attachments.ReadingMaterials = ensureMinTeachingModuleList(attachments.ReadingMaterials, []string{
		"Ringkasan materi, buku teks, atau sumber bacaan relevan sesuai topik.",
		"Bahan bacaan singkat yang membantu peserta didik memahami konsep kunci sebelum latihan.",
		"Sumber belajar tambahan yang dapat dipakai guru untuk pengayaan atau remedial.",
	}, 3, 6)
	attachments.Glossary = ensureMinTeachingModuleList(attachments.Glossary, []string{
		"Istilah penting pada topik pembelajaran beserta makna singkatnya.",
		"Kosakata kunci yang sering muncul selama pembelajaran.",
		"Definisi singkat untuk membantu peserta didik memahami istilah teknis utama.",
	}, 3, 8)
	attachments.Bibliography = ensureMinTeachingModuleList(attachments.Bibliography, []string{
		"Buku teks resmi, modul sekolah, atau sumber belajar tepercaya yang relevan dengan topik.",
		"Referensi pendukung yang dapat dipakai guru untuk menyiapkan materi lanjutan.",
		"Sumber belajar daring atau cetak yang sesuai dengan jenjang dan topik pembelajaran.",
	}, 3, 8)

	notes := strings.TrimSpace(parsed.Notes)
	if notes == "" {
		notes = "Silakan sesuaikan detail akhir modul dengan ATP sekolah, karakteristik kelas, dan perangkat asesmen yang digunakan guru."
	}
	notes = sanitizeTeachingModuleNotes(notes)

	return TeachingModuleAIDraft{
		Title:              title,
		Identity:           identity,
		GeneralInformation: general,
		CoreComponents:     core,
		Attachments:        attachments,
		Notes:              notes,
	}
}

func sanitizeTeachingModuleNotes(notes string) string {
	lower := strings.ToLower(strings.TrimSpace(notes))
	if lower == "" || strings.Contains(lower, "template cadangan") || strings.Contains(lower, "respons ai") || strings.Contains(lower, "hasil ai") || strings.Contains(lower, "hugging") || strings.Contains(lower, "qwen") {
		return "Silakan sesuaikan detail akhir modul dengan ATP sekolah, karakteristik kelas, dan perangkat asesmen yang digunakan guru."
	}
	return strings.TrimSpace(notes)
}

func normalizeTeachingModuleSchedule(identity TeachingModuleIdentity, mode string) TeachingModuleIdentity {
	if mode == "digital_content" && identity.Meetings < 3 {
		identity.Meetings = 3
		identity.TimeAllocation = "6 x 45 menit (3 pertemuan)"
	}
	if mode == "digital_concept" && identity.Meetings <= 0 {
		identity.Meetings = 1
		identity.TimeAllocation = fallbackText(identity.TimeAllocation, "2 x 45 menit")
	}
	return identity
}

func fallbackTeachingModuleList(values, fallback []string, limit int) []string {
	normalized := trimAndLimitStrings(values, limit)
	if len(normalized) > 0 {
		return normalized
	}
	return trimAndLimitStrings(fallback, limit)
}

func ensureMinTeachingModuleList(values, fallback []string, minCount, limit int) []string {
	normalized := trimAndLimitStrings(values, limit)
	if len(normalized) >= minCount {
		return normalized
	}

	seen := map[string]bool{}
	for _, item := range normalized {
		seen[strings.ToLower(strings.TrimSpace(item))] = true
	}
	for _, item := range fallback {
		cleaned := strings.TrimSpace(item)
		if cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if seen[key] {
			continue
		}
		normalized = append(normalized, cleaned)
		seen[key] = true
		if limit > 0 && len(normalized) >= limit {
			break
		}
	}
	return normalized
}

func ensureSpecificTeachingModuleList(values, fallback []string, topic, mode string, minCount, limit int) []string {
	normalized := ensureMinTeachingModuleList(values, fallback, minCount, limit)
	if mode == "general" {
		return normalized
	}

	joined := strings.ToLower(strings.Join(normalized, " "))
	topicWords := strings.Fields(strings.ToLower(topic))
	hasTopicSignal := false
	for _, word := range topicWords {
		if len(word) >= 5 && strings.Contains(joined, word) {
			hasTopicSignal = true
			break
		}
	}

	modeSignals := map[string][]string{
		"digital_content": {"brief", "storyboard", "moodboard", "aset", "editing", "branding", "portofolio", "publikasi", "workflow"},
		"public_speaking": {"presentasi", "artikulasi", "intonasi", "kontak mata", "gesture", "simulasi", "tampil"},
		"practice":        {"praktik", "demonstrasi", "prosedur", "produk", "performa"},
	}
	hasModeSignal := false
	for _, signal := range modeSignals[mode] {
		if strings.Contains(joined, signal) {
			hasModeSignal = true
			break
		}
	}
	if hasTopicSignal && hasModeSignal {
		return normalized
	}
	return fallbackTeachingModuleList(fallback, fallback, limit)
}

func splitTeachingModuleInput(raw string, fallback []string) []string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return fallback
	}

	replacer := strings.NewReplacer("\r\n", "\n", "\r", "\n", ";", "\n", ",", "\n")
	parts := strings.Split(replacer.Replace(text), "\n")
	return fallbackTeachingModuleList(parts, fallback, 8)
}

func normalizeTeachingModuleSuggestions(parsed TeachingModuleSuggestionCatalog, input TeachingModuleSuggestionInput) TeachingModuleSuggestionCatalog {
	category := strings.TrimSpace(parsed.SubjectCategory)
	if category == "" {
		category = inferTeachingModuleSubjectCategory(input.SubjectName)
	}
	mode := inferTeachingModuleInstructionMode(input.SubjectName, input.Topic)

	defaultTopics := []string{
		fallbackText(input.Topic, fmt.Sprintf("Topik inti %s", fallbackText(input.SubjectName, "pembelajaran"))),
	}

	return TeachingModuleSuggestionCatalog{
		SubjectCategory:           category,
		Titles:                    fallbackTeachingModuleList(parsed.Titles, []string{fallbackText(input.Topic, fmt.Sprintf("Modul Ajar %s", fallbackText(input.SubjectName, "Mapel")))}, 8),
		Topics:                    fallbackTeachingModuleList(parsed.Topics, defaultTopics, 8),
		CPReferences:              fallbackTeachingModuleList(parsed.CPReferences, []string{buildSpecificCPReference(TeachingModuleAIInput{SubjectName: input.SubjectName, Topic: input.Topic, GradeLabel: input.GradeLabel, ClassName: input.ClassName}, category, mode)}, 8),
		LearningObjectives:        fallbackTeachingModuleList(parsed.LearningObjectives, buildLearningObjectives(TeachingModuleAIInput{SubjectName: input.SubjectName, Topic: input.Topic, GradeLabel: input.GradeLabel, ClassName: input.ClassName}, category, mode), 8),
		MaterialScopes:            fallbackTeachingModuleList(parsed.MaterialScopes, []string{"Konsep inti, contoh penerapan, latihan terarah, dan evaluasi singkat."}, 8),
		StudentCharacteristics:    fallbackTeachingModuleList(parsed.StudentCharacteristics, []string{"Kelas heterogen dengan variasi kesiapan belajar dan membutuhkan contoh kontekstual."}, 8),
		Facilities:                fallbackTeachingModuleList(parsed.Facilities, []string{"Buku ajar, LKPD, papan tulis, dan media presentasi sederhana."}, 8),
		AdditionalInstructionTips: fallbackTeachingModuleList(parsed.AdditionalInstructionTips, []string{"Gunakan contoh yang dekat dengan kehidupan sehari-hari peserta didik."}, 8),
	}
}

func inferTeachingModuleSubjectCategory(subjectName string) string {
	name := strings.ToLower(strings.TrimSpace(subjectName))
	switch {
	case strings.Contains(name, "matematika"):
		return "Numerasi"
	case strings.Contains(name, "fisika"), strings.Contains(name, "kimia"), strings.Contains(name, "biologi"), strings.Contains(name, "ipa"), strings.Contains(name, "ipas"):
		return "Sains"
	case strings.Contains(name, "bahasa indonesia"), strings.Contains(name, "bahasa inggris"), strings.Contains(name, "bahasa jawa"), strings.Contains(name, "bahasa"):
		return "Bahasa"
	case strings.Contains(name, "sejarah"), strings.Contains(name, "geografi"), strings.Contains(name, "ekonomi"), strings.Contains(name, "sosiologi"), strings.Contains(name, "ips"), strings.Contains(name, "ppkn"), strings.Contains(name, "pkn"):
		return "Sosial Humaniora"
	case strings.Contains(name, "informatika"), strings.Contains(name, "komputer"), strings.Contains(name, "tkj"), strings.Contains(name, "rpl"):
		return "Teknologi"
	case strings.Contains(name, "agama"), strings.Contains(name, "fikih"), strings.Contains(name, "aqidah"), strings.Contains(name, "akidah"), strings.Contains(name, "quran"), strings.Contains(name, "hadis"):
		return "Keagamaan"
	case strings.Contains(name, "dkv"), strings.Contains(name, "desain komunikasi visual"), strings.Contains(name, "multimedia"), strings.Contains(name, "broadcast"), strings.Contains(name, "konten digital"), strings.Contains(name, "produksi konten"), strings.Contains(name, "industri kreatif"):
		return "Kejuruan Kreatif"
	case strings.Contains(name, "seni"), strings.Contains(name, "musik"), strings.Contains(name, "rupa"), strings.Contains(name, "teater"):
		return "Seni"
	case strings.Contains(name, "pjok"), strings.Contains(name, "olahraga"), strings.Contains(name, "penjaskes"):
		return "Olahraga"
	case strings.Contains(name, "produktif"), strings.Contains(name, "kejuruan"), strings.Contains(name, "akuntansi"), strings.Contains(name, "pemasaran"), strings.Contains(name, "perhotelan"), strings.Contains(name, "kuliner"):
		return "Kejuruan"
	default:
		return "Umum"
	}
}

func buildFallbackTeachingModuleSuggestions(input TeachingModuleSuggestionInput) TeachingModuleSuggestionCatalog {
	subjectName := fallbackText(input.SubjectName, "Mata Pelajaran")
	topic := fallbackText(input.Topic, fmt.Sprintf("Topik inti %s", subjectName))
	category := inferTeachingModuleSubjectCategory(input.SubjectName)
	mode := inferTeachingModuleInstructionMode(input.SubjectName, input.Topic)

	cpTemplate := map[string]string{
		"Numerasi":         "Peserta didik memahami konsep %s, menggunakan representasi yang tepat, dan menyelesaikan masalah kontekstual secara bertahap.",
		"Sains":            "Peserta didik menjelaskan konsep %s, menghubungkan dengan fenomena ilmiah, dan menyajikan hasil pengamatan atau analisis sederhana.",
		"Bahasa":           "Peserta didik memahami, menafsirkan, dan mengomunikasikan gagasan pada topik %s secara lisan maupun tulis.",
		"Sosial Humaniora": "Peserta didik menganalisis topik %s, menghubungkannya dengan konteks sosial, dan menyampaikan alasan secara logis.",
		"Teknologi":        "Peserta didik memahami konsep %s, menerapkan langkah kerja yang tepat, dan menghasilkan solusi atau produk sederhana.",
		"Keagamaan":        "Peserta didik memahami nilai dan konsep pada topik %s serta menunjukkan sikap yang sesuai dalam keseharian.",
		"Seni":             "Peserta didik memahami unsur pada topik %s, mengeksplorasi ide, dan menghasilkan karya atau apresiasi sederhana.",
		"Olahraga":         "Peserta didik memahami konsep gerak pada topik %s, mempraktikkan teknik yang benar, dan menunjukkan sikap sportivitas.",
		"Kejuruan":         "Peserta didik memahami dasar %s, menerapkan prosedur kerja, dan menghasilkan produk atau layanan sesuai standar sederhana.",
		"Umum":             "Peserta didik memahami konsep utama pada topik %s dan menerapkannya pada situasi yang relevan.",
	}

	objectiveTemplate := map[string][]string{
		"Numerasi": {
			fmt.Sprintf("Peserta didik mampu menjelaskan konsep dasar pada %s.", topic),
			fmt.Sprintf("Peserta didik mampu menyelesaikan latihan %s dengan langkah yang runtut.", topic),
		},
		"Sains": {
			fmt.Sprintf("Peserta didik mampu menjelaskan konsep inti pada %s.", topic),
			fmt.Sprintf("Peserta didik mampu mengaitkan %s dengan fenomena di sekitar.", topic),
		},
		"Bahasa": {
			fmt.Sprintf("Peserta didik mampu mengidentifikasi ide pokok pada topik %s.", topic),
			fmt.Sprintf("Peserta didik mampu menyampaikan gagasan terkait %s secara runtut.", topic),
		},
		"Kejuruan": {
			fmt.Sprintf("Peserta didik mampu menjelaskan prosedur dasar pada %s.", topic),
			fmt.Sprintf("Peserta didik mampu mempraktikkan langkah kerja %s secara aman dan sistematis.", topic),
		},
	}

	materialTemplate := map[string][]string{
		"Numerasi": {fmt.Sprintf("Definisi dan konsep inti %s", topic), fmt.Sprintf("Contoh soal dan penyelesaian %s", topic), fmt.Sprintf("Penerapan %s pada masalah kontekstual", topic)},
		"Sains":    {fmt.Sprintf("Konsep dasar %s", topic), fmt.Sprintf("Fenomena atau eksperimen sederhana tentang %s", topic), fmt.Sprintf("Analisis hasil belajar %s", topic)},
		"Bahasa":   {fmt.Sprintf("Pemahaman konsep kebahasaan pada %s", topic), fmt.Sprintf("Contoh teks atau dialog terkait %s", topic), fmt.Sprintf("Latihan menyusun respons pada %s", topic)},
		"Kejuruan": {fmt.Sprintf("Dasar teori %s", topic), fmt.Sprintf("Langkah kerja atau prosedur %s", topic), fmt.Sprintf("Standar hasil kerja pada %s", topic)},
		"Umum":     {fmt.Sprintf("Konsep inti %s", topic), fmt.Sprintf("Contoh penerapan %s", topic), fmt.Sprintf("Latihan dan evaluasi %s", topic)},
	}

	getObjectives := objectiveTemplate[category]
	if len(getObjectives) == 0 {
		getObjectives = []string{
			fmt.Sprintf("Peserta didik mampu menjelaskan konsep utama pada %s.", topic),
			fmt.Sprintf("Peserta didik mampu menerapkan %s pada tugas atau latihan yang relevan.", topic),
		}
	}

	getMaterials := materialTemplate[category]
	if len(getMaterials) == 0 {
		getMaterials = materialTemplate["Umum"]
	}

	cpText := cpTemplate[category]
	if cpText == "" {
		cpText = cpTemplate["Umum"]
	}
	cpReference := buildSpecificCPReference(TeachingModuleAIInput{SubjectName: input.SubjectName, Topic: topic, GradeLabel: input.GradeLabel, ClassName: input.ClassName}, category, mode)
	learningObjectives := buildLearningObjectives(TeachingModuleAIInput{SubjectName: input.SubjectName, Topic: topic, GradeLabel: input.GradeLabel, ClassName: input.ClassName}, category, mode)
	materialScopes := buildMaterialScopes(topic, mode)
	facilities := buildFacilities(topic, mode)
	if mode == "general" {
		cpReference = fmt.Sprintf(cpText, topic)
		if len(getObjectives) > 0 {
			learningObjectives = getObjectives
		}
		if len(getMaterials) > 0 {
			materialScopes = getMaterials
		}
	}

	return normalizeTeachingModuleSuggestions(TeachingModuleSuggestionCatalog{
		SubjectCategory: category,
		Titles: []string{
			fmt.Sprintf("Modul Ajar %s", topic),
			fmt.Sprintf("%s untuk %s", topic, subjectName),
			fmt.Sprintf("Eksplorasi %s", topic),
		},
		Topics: []string{
			topic,
			fmt.Sprintf("Penerapan %s", topic),
			fmt.Sprintf("Latihan dan penguatan %s", topic),
		},
		CPReferences: []string{
			cpReference,
			fmt.Sprintf("Peserta didik menguasai pengetahuan dasar %s dan menggunakannya dalam konteks belajar yang relevan.", topic),
		},
		LearningObjectives: learningObjectives,
		MaterialScopes:     materialScopes,
		StudentCharacteristics: []string{
			"Kelas heterogen dengan variasi kesiapan belajar.",
			"Peserta didik membutuhkan contoh yang kontekstual dan bertahap.",
			"Peserta didik cukup aktif jika diberikan pemantik dan latihan terarah.",
		},
		Facilities: facilities,
		AdditionalInstructionTips: []string{
			"Gunakan konteks kehidupan sehari-hari peserta didik.",
			"Fokuskan kegiatan pada diskusi singkat dan latihan bertahap.",
			"Gunakan bahasa sederhana dan kurangi istilah teknis yang tidak perlu.",
		},
	}, input)
}

func buildFallbackTeachingModuleDraft(input TeachingModuleAIInput) TeachingModuleAIDraft {
	title := fallbackText(input.Title, fallbackText(input.Topic, fmt.Sprintf("Modul Ajar %s", fallbackText(input.SubjectName, "Mapel"))))
	topic := fallbackText(input.Topic, "Topik pembelajaran")
	curriculum := fallbackText(input.CurriculumName, "Kurikulum Merdeka")
	gradeLabel := fallbackText(input.GradeLabel, input.ClassName)
	phaseName := fallbackText(input.PhaseName, "Fase belum ditentukan")
	category := inferTeachingModuleSubjectCategory(input.SubjectName)
	instructionMode := inferTeachingModuleInstructionMode(input.SubjectName, input.Topic)

	objectives := ensureMinTeachingModuleList(splitTeachingModuleInput(input.LearningObjectives, buildLearningObjectives(input, category, instructionMode)), nil, 4, 8)

	cpReference := fallbackText(input.CPReference, buildSpecificCPReference(input, category, instructionMode))
	studentCharacteristics := fallbackText(input.StudentCharacteristics, "Kelas heterogen dengan variasi kesiapan belajar dan membutuhkan contoh kontekstual.")
	facilities := normalizeTeachingModuleFacilities(nil, splitTeachingModuleInput(input.Facilities, buildFacilities(topic, instructionMode)), instructionMode)
	pancasilaProfile := normalizePancasilaProfile(nil, splitTeachingModuleInput(input.PancasilaProfile, defaultPancasilaProfile(instructionMode)), instructionMode)

	draft := TeachingModuleAIDraft{
		Title: title,
		Identity: TeachingModuleIdentity{
			SubjectName:    input.SubjectName,
			ClassName:      input.ClassName,
			GradeLabel:     gradeLabel,
			PhaseName:      phaseName,
			CurriculumName: curriculum,
			Topic:          topic,
			Title:          title,
			TimeAllocation: fallbackText(input.TimeAllocation, "2 x 45 menit"),
			Meetings:       input.Meetings,
		},
		GeneralInformation: TeachingModuleGeneralInformation{
			CompetencyPrerequisites: buildCompetencyPrerequisites(topic, instructionMode),
			PancasilaProfile:        pancasilaProfile,
			Facilities:              facilities,
			TargetLearners:          studentCharacteristics,
			LearningModel:           fallbackText(input.LearningModel, "Problem Based Learning (PBL)"),
			LearningApproach:        "Pembelajaran aktif, kontekstual, diferensiatif, dan berpusat pada peserta didik.",
		},
		CoreComponents: TeachingModuleCoreComponents{
			CPReference:             cpReference,
			LearningObjectives:      objectives,
			AchievementIndicators:   buildAchievementIndicators(topic, instructionMode),
			ProjectOutputs:          buildProjectOutputs(topic, instructionMode),
			MeaningfulUnderstanding: buildMeaningfulUnderstanding(topic, category, instructionMode),
			TriggerQuestions:        buildTriggerQuestions(topic, category, instructionMode),
			LearningActivities: TeachingModuleLearningActivities{
				Introduction: buildIntroductionActivities(topic),
				Core:         buildCoreActivities(topic, category, instructionMode),
				Closing:      buildClosingActivities(topic),
			},
			Assessments: TeachingModuleAssessments{
				Diagnostic: buildDiagnosticAssessments(topic, instructionMode),
				Formative:  buildFormativeAssessments(topic, instructionMode),
				Summative:  buildSummativeAssessments(topic, instructionMode),
				Rubric:     buildRubricCriteria(topic, instructionMode),
			},
			Differentiation: TeachingModuleDifferentiation{
				Content: buildDifferentiationContent(topic, instructionMode),
				Process: buildDifferentiationProcess(topic, instructionMode),
				Product: buildDifferentiationProduct(topic, instructionMode),
			},
			Remedial: []string{
				"Guru memberikan penjelasan ulang pada bagian konsep yang belum dikuasai peserta didik.",
				"Peserta didik mengerjakan latihan bertahap dengan contoh yang lebih sederhana.",
				"Guru memberi bimbingan individual atau kelompok kecil pada peserta didik yang belum tuntas.",
			},
			Enrichment: []string{
				"Peserta didik yang sudah tuntas diberi tugas pengayaan dengan konteks yang lebih menantang.",
				"Guru menyediakan soal analitis atau penerapan lanjutan sesuai topik.",
				"Peserta didik diminta merangkum atau mempresentasikan hasil pengayaan secara singkat.",
			},
			StudentReflection: []string{
				"Konsep apa yang paling saya pahami pada pembelajaran hari ini?",
				"Bagian mana yang masih perlu saya pelajari atau latih kembali?",
				"Strategi belajar apa yang paling membantu saya memahami topik ini?",
			},
			TeacherReflection: []string{
				"Bagian kegiatan mana yang paling membantu peserta didik memahami topik?",
				"Apakah tujuan pembelajaran sudah tercapai sesuai indikator yang direncanakan?",
				"Perbaikan apa yang perlu dilakukan pada media, strategi, atau asesmen di pertemuan berikutnya?",
			},
		},
		Attachments: TeachingModuleAttachments{
			StudentWorksheet: []string{
				fmt.Sprintf("LKPD berisi latihan inti terkait %s.", topic),
				fmt.Sprintf("Tugas terstruktur untuk menguji pemahaman peserta didik pada %s.", topic),
				"Pertanyaan reflektif singkat sebagai penutup pembelajaran.",
			},
			ReadingMaterials: []string{
				fmt.Sprintf("Ringkasan materi pokok tentang %s.", topic),
				"Buku ajar atau modul sekolah yang relevan dengan tujuan pembelajaran.",
				"Sumber pendukung yang membantu peserta didik memahami konsep kunci sebelum evaluasi.",
			},
			Glossary: []string{
				fmt.Sprintf("Istilah penting yang sering muncul pada topik %s.", topic),
				"Definisi singkat konsep utama agar peserta didik lebih mudah memahami pembelajaran.",
				"Kosakata teknis atau istilah mapel yang perlu dipahami sejak awal.",
			},
			Bibliography: []string{
				"Buku teks resmi atau modul sekolah yang digunakan guru.",
				"Sumber belajar tepercaya yang relevan dengan topik pembelajaran.",
				"Referensi tambahan untuk penguatan materi, pengayaan, atau remedial.",
			},
		},
		Notes: "Silakan sesuaikan detail akhir modul dengan ATP sekolah, karakteristik peserta didik, dan instrumen penilaian guru.",
	}

	return normalizeTeachingModuleDraft(draft, input)
}

func inferTeachingModuleInstructionMode(subjectName, topic string) string {
	subjectText := strings.ToLower(strings.TrimSpace(subjectName))
	topicText := strings.ToLower(strings.TrimSpace(topic))
	text := strings.TrimSpace(subjectText + " " + topicText)
	isCreativeDigital := strings.Contains(text, "dkv") ||
		strings.Contains(text, "desain komunikasi visual") ||
		strings.Contains(text, "multimedia") ||
		strings.Contains(text, "konten digital") ||
		strings.Contains(text, "produksi konten") ||
		strings.Contains(text, "editing video") ||
		strings.Contains(text, "editing audio") ||
		strings.Contains(text, "branding visual") ||
		strings.Contains(text, "media digital") ||
		strings.Contains(text, "industri kreatif") ||
		strings.Contains(text, "canva") ||
		strings.Contains(text, "capcut") ||
		strings.Contains(text, "adobe")
	isConceptTopic := strings.Contains(topicText, "konsep dasar") ||
		strings.Contains(topicText, "pengertian") ||
		strings.Contains(topicText, "unsur") ||
		strings.Contains(topicText, "fungsi") ||
		strings.Contains(topicText, "prinsip") ||
		strings.Contains(topicText, "pengenalan") ||
		strings.Contains(topicText, "dasar komunikasi") ||
		strings.Contains(topicText, "komunikasi industri kreatif")
	switch {
	case isCreativeDigital && isConceptTopic:
		return "digital_concept"
	case isCreativeDigital:
		return "digital_content"
	case strings.Contains(text, "public speaking"), strings.Contains(text, "komunikasi"), strings.Contains(text, "presentasi"), strings.Contains(text, "pidato"), strings.Contains(text, "debat"), strings.Contains(text, "berbicara di depan umum"):
		return "public_speaking"
	case strings.Contains(text, "praktik"), strings.Contains(text, "praktikum"), strings.Contains(text, "demonstrasi"):
		return "practice"
	default:
		return "general"
	}
}

func defaultPancasilaProfile(mode string) []string {
	switch mode {
	case "digital_content":
		return []string{"Kreatif", "Bernalar kritis", "Mandiri", "Gotong royong"}
	case "digital_concept":
		return []string{"Bernalar kritis", "Kreatif", "Mandiri", "Gotong royong"}
	case "public_speaking":
		return []string{"Mandiri", "Bernalar kritis", "Gotong royong", "Kreatif"}
	default:
		return []string{"Bernalar kritis", "Mandiri", "Gotong royong"}
	}
}

func normalizePancasilaProfile(values, fallback []string, mode string) []string {
	official := map[string]string{
		"beriman":          "Beriman, bertakwa kepada Tuhan YME, dan berakhlak mulia",
		"bertakwa":         "Beriman, bertakwa kepada Tuhan YME, dan berakhlak mulia",
		"akhlak":           "Beriman, bertakwa kepada Tuhan YME, dan berakhlak mulia",
		"berkebinekaan":    "Berkebinekaan global",
		"kebinekaan":       "Berkebinekaan global",
		"global":           "Berkebinekaan global",
		"gotong":           "Gotong royong",
		"kolaborasi":       "Gotong royong",
		"mandiri":          "Mandiri",
		"kritis":           "Bernalar kritis",
		"bernalar":         "Bernalar kritis",
		"berpikir kritis":  "Bernalar kritis",
		"berjiwa kritis":   "Bernalar kritis",
		"kreatif":          "Kreatif",
		"kreativitas":      "Kreatif",
		"berpikir kreatif": "Kreatif",
		"berjiwa kreatif":  "Kreatif",
	}

	normalized := make([]string, 0, 6)
	seen := map[string]bool{}
	add := func(raw string) {
		text := strings.ToLower(strings.TrimSpace(raw))
		if text == "" {
			return
		}
		value := ""
		for key, officialValue := range official {
			if strings.Contains(text, key) {
				value = officialValue
				break
			}
		}
		if value == "" {
			return
		}
		if seen[value] {
			return
		}
		seen[value] = true
		normalized = append(normalized, value)
	}

	for _, item := range values {
		add(item)
	}
	for _, item := range fallback {
		add(item)
	}
	for _, item := range defaultPancasilaProfile(mode) {
		add(item)
	}
	if len(normalized) > 4 {
		normalized = normalized[:4]
	}
	return normalized
}

func buildFacilities(topic, mode string) []string {
	switch mode {
	case "digital_content":
		return []string{
			"Laptop atau PC dengan aplikasi desain/editing seperti Canva, CapCut, Adobe Photoshop, Illustrator, Premiere, atau aplikasi setara.",
			"Smartphone atau kamera untuk pengambilan gambar, video, dan dokumentasi produksi.",
			"Akses internet untuk riset referensi visual, pengumpulan aset, dan publikasi konten digital.",
			"Proyektor atau layar kelas untuk melihat contoh karya, briefing, dan presentasi hasil produksi.",
			"Headset, speaker, atau mikrofon sederhana untuk mengecek kualitas audio/video.",
			"Template brief, storyboard, moodboard, dan lembar checklist produksi konten digital.",
		}
	case "digital_concept":
		return []string{
			"Contoh konten brand, poster digital, feed media sosial, iklan visual, atau video pendek untuk bahan analisis.",
			"LCD/proyektor atau layar kelas untuk menampilkan contoh komunikasi visual industri kreatif.",
			"Laptop/PC atau smartphone untuk mengakses referensi media digital dan mencatat hasil analisis.",
			"LKPD analisis unsur komunikasi, audiens, pesan, media, dan identitas visual.",
			"Papan tulis atau aplikasi kolaborasi sederhana untuk merangkum hasil diskusi kelas.",
		}
	case "public_speaking":
		return []string{
			"Ruang kelas atau area tampil untuk simulasi presentasi.",
			"Perangkat perekam video sederhana untuk refleksi performa berbicara.",
			"Proyektor atau layar presentasi untuk menampilkan contoh dan struktur materi.",
			"Lembar observasi atau rubrik performa public speaking.",
		}
	default:
		return []string{
			"Buku ajar dan lembar kerja peserta didik.",
			"Perangkat tulis dan media presentasi sederhana.",
			"Papan tulis atau layar presentasi untuk penguatan konsep.",
			fmt.Sprintf("Sumber belajar pendukung yang relevan dengan %s.", fallbackText(topic, "topik pembelajaran")),
		}
	}
}

func buildCompetencyPrerequisites(topic, mode string) []string {
	switch mode {
	case "digital_content":
		return []string{
			"Peserta didik telah mengenal dasar desain visual, komposisi sederhana, warna, tipografi, dan penggunaan aplikasi desain/editing dasar.",
			"Peserta didik pernah melihat atau membuat konten digital sederhana seperti poster, feed media sosial, foto produk, atau video pendek.",
			"Peserta didik mampu bekerja dalam kelompok kecil dan mengikuti instruksi produksi sederhana.",
		}
	case "digital_concept":
		return []string{
			"Peserta didik telah mengenal contoh karya visual seperti poster, feed media sosial, iklan, logo, atau video pendek.",
			"Peserta didik dapat membedakan teks, gambar, warna, media, dan audiens secara sederhana.",
			"Peserta didik mampu menyampaikan pendapat singkat tentang contoh konten visual yang mereka lihat.",
		}
	case "public_speaking":
		return []string{
			"Peserta didik pernah berbicara di depan kelas dalam kegiatan sederhana.",
			"Peserta didik mampu menyusun gagasan singkat sebelum menyampaikan pendapat.",
			"Peserta didik dapat memberi dan menerima umpan balik dengan bahasa yang santun.",
		}
	default:
		return []string{
			fmt.Sprintf("Peserta didik telah memiliki pengalaman awal yang berkaitan dengan %s.", fallbackText(topic, "topik pembelajaran")),
			"Peserta didik mampu mengikuti instruksi belajar dasar dan berdiskusi secara terarah.",
			"Peserta didik dapat mencatat informasi penting dari contoh atau penjelasan guru.",
		}
	}
}

func normalizeTeachingModuleFacilities(values, fallback []string, mode string) []string {
	normalized := ensureMinTeachingModuleList(values, fallback, 3, 8)
	if mode != "digital_content" && mode != "digital_concept" {
		return normalized
	}

	joined := strings.ToLower(strings.Join(normalized, " "))
	digitalKeywords := []string{"laptop", "pc", "canva", "capcut", "adobe", "kamera", "smartphone", "internet", "headset", "microphone", "mikrofon", "proyektor"}
	hasDigitalFacility := false
	for _, keyword := range digitalKeywords {
		if strings.Contains(joined, keyword) {
			hasDigitalFacility = true
			break
		}
	}
	if hasDigitalFacility {
		return ensureMinTeachingModuleList(normalized, fallback, 4, 8)
	}
	digitalFallback := buildFacilities("", mode)
	return fallbackTeachingModuleList(digitalFallback, digitalFallback, 8)
}

func buildMaterialScopes(topic, mode string) []string {
	switch mode {
	case "digital_content":
		return []string{
			fmt.Sprintf("Analisis brief dan tujuan komunikasi pada produksi %s.", topic),
			"Riset audiens, referensi visual, moodboard, dan konsep pesan.",
			"Perencanaan workflow produksi: ide, storyboard, shot list, aset visual/audio, editing, revisi, dan publikasi.",
			"Praktik pembuatan konten digital dengan memperhatikan branding visual, storytelling, kualitas teknis, dan etika penggunaan aset.",
		}
	case "digital_concept":
		return []string{
			fmt.Sprintf("Pengertian dan ruang lingkup %s dalam industri kreatif.", topic),
			"Fungsi komunikasi visual untuk menyampaikan pesan, membangun citra, dan memengaruhi audiens.",
			"Unsur komunikasi kreatif: komunikator, pesan, audiens, media, konteks, visual, dan respons.",
			"Contoh penerapan komunikasi brand pada poster, konten media sosial, iklan visual, dan video pendek.",
			"Analisis sederhana terhadap pesan, target audiens, media, dan identitas visual pada contoh karya.",
		}
	case "public_speaking":
		return []string{
			fmt.Sprintf("Struktur opening, isi, dan closing pada presentasi %s.", topic),
			"Teknik vokal, artikulasi, intonasi, kontak mata, gesture, dan pengelolaan rasa gugup.",
			"Simulasi presentasi dan evaluasi performa menggunakan rubrik sederhana.",
		}
	default:
		return []string{
			fmt.Sprintf("Konsep inti %s.", topic),
			fmt.Sprintf("Contoh penerapan %s.", topic),
			fmt.Sprintf("Latihan dan evaluasi %s.", topic),
		}
	}
}

func buildSpecificCPReference(input TeachingModuleAIInput, category, mode string) string {
	topic := fallbackText(input.Topic, "topik pembelajaran")
	subjectName := fallbackText(input.SubjectName, "mata pelajaran")
	switch mode {
	case "digital_content":
		return fmt.Sprintf("Peserta didik menerapkan strategi produksi konten digital pada topik %s dengan menganalisis brief, merancang konsep visual, menyusun workflow produksi, mengolah aset foto/video/audio, menjaga konsistensi branding visual, serta mempublikasikan atau mempresentasikan karya sesuai tujuan komunikasi media digital.", topic)
	case "digital_concept":
		return fmt.Sprintf("Peserta didik memahami konsep %s dalam industri kreatif dengan mengidentifikasi fungsi komunikasi visual, unsur pesan, target audiens, media komunikasi, dan contoh penerapan komunikasi brand pada karya digital sederhana.", topic)
	case "public_speaking":
		return fmt.Sprintf("Peserta didik memahami teknik komunikasi lisan profesional pada topik %s, menyusun pesan secara runtut, serta menampilkan presentasi singkat dengan artikulasi, intonasi, kontak mata, gesture, dan rasa percaya diri yang tepat.", topic)
	case "practice":
		return fmt.Sprintf("Peserta didik memahami prosedur dan prinsip kerja pada topik %s dalam mata pelajaran %s, lalu mendemonstrasikan langkah kerja secara runtut, aman, dan sesuai kriteria performa.", topic, subjectName)
	}
	switch category {
	case "Bahasa":
		return fmt.Sprintf("Peserta didik menganalisis dan mengomunikasikan gagasan pada topik %s secara lisan atau tulis dengan struktur, pilihan bahasa, dan tujuan komunikasi yang tepat.", topic)
	case "Kejuruan":
		return fmt.Sprintf("Peserta didik menerapkan konsep dan prosedur pada topik %s untuk menghasilkan performa kerja atau produk sederhana sesuai standar pembelajaran kejuruan.", topic)
	default:
		return fmt.Sprintf("Peserta didik menguasai konsep inti pada topik %s, menerapkannya pada tugas yang relevan, dan menunjukkan hasil belajar melalui performa atau produk yang dapat diamati.", topic)
	}
}

func buildLearningObjectives(input TeachingModuleAIInput, category, mode string) []string {
	topic := fallbackText(input.Topic, "topik pembelajaran")
	switch mode {
	case "digital_content":
		return []string{
			fmt.Sprintf("Peserta didik mampu menganalisis audiens pada brief produksi %s.", topic),
			fmt.Sprintf("Peserta didik mampu menentukan pesan utama dan gaya visual untuk %s.", topic),
			fmt.Sprintf("Peserta didik mampu menyusun rencana produksi %s berupa moodboard, storyboard atau shot list, dan jadwal kerja.", topic),
			fmt.Sprintf("Peserta didik mampu memproduksi aset visual/audio/video untuk %s menggunakan perangkat dan aplikasi digital yang tersedia.", topic),
			fmt.Sprintf("Peserta didik mampu mengedit dan menyajikan karya %s dengan memperhatikan storytelling, kualitas visual, audio, konsistensi branding, dan ketepatan format media.", topic),
			fmt.Sprintf("Peserta didik mampu mempresentasikan portofolio %s berdasarkan rubrik kreativitas, teknis produksi, dan efektivitas pesan.", topic),
		}
	case "digital_concept":
		return []string{
			fmt.Sprintf("Peserta didik mampu menjelaskan pengertian %s dengan bahasa sendiri.", topic),
			"Peserta didik mampu mengidentifikasi fungsi komunikasi visual dalam contoh karya industri kreatif.",
			"Peserta didik mampu membedakan unsur pesan, audiens, media, visual, dan tujuan komunikasi pada contoh konten.",
			"Peserta didik mampu menganalisis satu contoh komunikasi brand secara sederhana menggunakan lembar observasi.",
			"Peserta didik mampu menyampaikan hasil analisis konsep komunikasi kreatif dalam diskusi atau presentasi singkat.",
		}
	case "public_speaking":
		return []string{
			fmt.Sprintf("Peserta didik mampu mengidentifikasi unsur presentasi efektif pada topik %s melalui pengamatan contoh tampil.", topic),
			fmt.Sprintf("Peserta didik mampu menyusun opening presentasi sederhana tentang %s dengan struktur pembuka yang jelas.", topic),
			fmt.Sprintf("Peserta didik mampu menyampaikan presentasi lisan 2 sampai 3 menit tentang %s dengan artikulasi, intonasi, dan kontak mata yang baik.", topic),
			fmt.Sprintf("Peserta didik mampu mengevaluasi performa berbicara diri sendiri atau teman sebaya pada topik %s menggunakan rubrik sederhana.", topic),
		}
	case "practice":
		return []string{
			fmt.Sprintf("Peserta didik mampu menjelaskan langkah kerja utama pada topik %s secara runtut.", topic),
			fmt.Sprintf("Peserta didik mampu mendemonstrasikan prosedur %s sesuai instruksi dan standar kerja yang ditetapkan.", topic),
			fmt.Sprintf("Peserta didik mampu menghasilkan produk atau performa praktik terkait %s yang memenuhi kriteria dasar.", topic),
			fmt.Sprintf("Peserta didik mampu merefleksikan kekuatan dan perbaikan dari hasil praktik %s berdasarkan umpan balik guru.", topic),
		}
	}
	switch category {
	case "Bahasa":
		return []string{
			fmt.Sprintf("Peserta didik mampu mengidentifikasi informasi penting pada materi %s.", topic),
			fmt.Sprintf("Peserta didik mampu menyusun respons lisan atau tulis terkait %s secara runtut.", topic),
			fmt.Sprintf("Peserta didik mampu mempresentasikan gagasan tentang %s dengan bahasa yang jelas.", topic),
			fmt.Sprintf("Peserta didik mampu meninjau kembali hasil kerja terkait %s berdasarkan umpan balik.", topic),
		}
	default:
		return []string{
			fmt.Sprintf("Peserta didik mampu mengidentifikasi konsep utama pada %s.", topic),
			fmt.Sprintf("Peserta didik mampu menjelaskan hubungan antarunsur pada %s secara runtut.", topic),
			fmt.Sprintf("Peserta didik mampu menerapkan %s pada latihan atau tugas yang relevan.", topic),
			fmt.Sprintf("Peserta didik mampu menyajikan hasil belajar terkait %s secara jelas dan terukur.", topic),
		}
	}
}

func buildAchievementIndicators(topic, mode string) []string {
	topic = fallbackText(topic, "topik pembelajaran")
	switch mode {
	case "digital_content":
		return []string{
			fmt.Sprintf("Peserta didik menghasilkan analisis brief %s yang memuat audiens, pesan utama, platform, gaya visual, dan batasan format.", topic),
			fmt.Sprintf("Peserta didik menyusun perencanaan produksi %s berupa moodboard, storyboard atau shot list, jadwal, pembagian peran, dan daftar aset.", topic),
			fmt.Sprintf("Peserta didik memproduksi dan mengelola aset visual/audio/video untuk %s secara rapi, legal, dan sesuai kebutuhan pesan.", topic),
			fmt.Sprintf("Peserta didik menyelesaikan karya akhir %s dengan kualitas visual, editing, storytelling, audio, dan branding yang sesuai rubrik minimal kategori baik.", topic),
			fmt.Sprintf("Peserta didik mempresentasikan portofolio %s dengan menjelaskan konsep, proses produksi, revisi, dan alasan keputusan desain.", topic),
		}
	case "digital_concept":
		return []string{
			fmt.Sprintf("Peserta didik menjelaskan pengertian %s dan ruang lingkupnya dalam industri kreatif.", topic),
			"Peserta didik menunjukkan unsur komunikasi visual pada contoh karya yang diamati.",
			"Peserta didik mengelompokkan pesan, audiens, media, gaya visual, dan tujuan komunikasi dari minimal dua contoh konten.",
			"Peserta didik menyusun hasil analisis sederhana dalam LKPD atau presentasi singkat.",
		}
	case "public_speaking":
		return []string{
			fmt.Sprintf("Peserta didik menyusun kerangka presentasi %s dengan opening, isi, dan closing yang jelas.", topic),
			"Peserta didik tampil 2 sampai 3 menit dengan artikulasi, intonasi, kontak mata, dan gesture yang sesuai.",
			"Peserta didik menerima dan menggunakan umpan balik untuk memperbaiki performa berbicara.",
			"Peserta didik merefleksikan kekuatan dan area perbaikan performa secara tertulis atau lisan.",
		}
	case "practice":
		return []string{
			fmt.Sprintf("Peserta didik menjelaskan prosedur %s secara runtut sebelum praktik.", topic),
			fmt.Sprintf("Peserta didik mendemonstrasikan praktik %s sesuai langkah kerja dan keselamatan.", topic),
			fmt.Sprintf("Peserta didik menghasilkan produk atau performa %s sesuai kriteria minimal.", topic),
			"Peserta didik memperbaiki hasil praktik berdasarkan umpan balik guru atau teman sebaya.",
		}
	default:
		return []string{
			fmt.Sprintf("Peserta didik mengidentifikasi konsep penting pada %s dengan tepat.", topic),
			fmt.Sprintf("Peserta didik menerapkan konsep %s pada tugas atau latihan yang relevan.", topic),
			fmt.Sprintf("Peserta didik menyajikan hasil belajar %s secara runtut dan jelas.", topic),
			"Peserta didik menunjukkan perbaikan pemahaman berdasarkan umpan balik selama pembelajaran.",
		}
	}
}

func buildProjectOutputs(topic, mode string) []string {
	topic = fallbackText(topic, "topik pembelajaran")
	switch mode {
	case "digital_content":
		return []string{
			fmt.Sprintf("Dokumen brief produksi %s yang berisi tujuan komunikasi, target audiens, pesan utama, platform, dan format akhir.", topic),
			"Moodboard visual, storyboard atau shot list, serta daftar kebutuhan aset foto/video/audio/grafis.",
			fmt.Sprintf("Karya akhir %s dalam format digital sesuai platform, misalnya video pendek, desain feed, poster digital, motion sederhana, atau konten promosi.", topic),
			"Portofolio proses yang memuat dokumentasi produksi, draft, revisi, hasil akhir, dan refleksi keputusan desain.",
		}
	case "digital_concept":
		return []string{
			fmt.Sprintf("LKPD analisis konsep %s berdasarkan contoh poster, konten media sosial, iklan visual, atau video pendek.", topic),
			"Peta konsep yang memuat pengertian, fungsi, unsur komunikasi, media, audiens, dan contoh komunikasi brand.",
			"Presentasi singkat hasil analisis satu contoh komunikasi industri kreatif.",
		}
	case "public_speaking":
		return []string{
			fmt.Sprintf("Kerangka presentasi %s berisi opening, isi utama, closing, dan catatan teknik penyampaian.", topic),
			"Rekaman atau observasi performa presentasi 2 sampai 3 menit.",
			"Lembar refleksi perbaikan performa setelah menerima umpan balik.",
		}
	case "practice":
		return []string{
			fmt.Sprintf("Rencana kerja atau prosedur praktik %s.", topic),
			fmt.Sprintf("Produk atau performa praktik %s sesuai instruksi.", topic),
			"Lembar refleksi hasil praktik dan perbaikan yang dilakukan.",
		}
	default:
		return []string{
			fmt.Sprintf("Lembar kerja atau tugas terstruktur terkait %s.", topic),
			fmt.Sprintf("Hasil presentasi, jawaban, atau produk sederhana yang menunjukkan pemahaman %s.", topic),
			"Refleksi singkat peserta didik terhadap proses dan hasil belajar.",
		}
	}
}

func buildMeaningfulUnderstanding(topic, category, mode string) []string {
	if mode == "digital_content" {
		return []string{
			fmt.Sprintf("Peserta didik memahami bahwa produksi %s membutuhkan hubungan yang utuh antara pesan, audiens, konsep visual, workflow, dan kualitas teknis.", topic),
			"Peserta didik memahami bahwa karya konten digital yang baik tidak hanya menarik secara visual, tetapi juga konsisten dengan tujuan komunikasi dan identitas visual.",
			"Peserta didik memahami bahwa proses revisi, umpan balik, dan ketepatan deadline merupakan bagian penting dari budaya kerja industri kreatif.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			fmt.Sprintf("Peserta didik memahami bahwa %s menjadi dasar untuk membaca pesan visual dalam industri kreatif.", topic),
			"Peserta didik memahami bahwa komunikasi visual yang efektif selalu mempertimbangkan audiens, media, pesan, dan konteks brand.",
			"Peserta didik memahami bahwa analisis contoh karya membantu mereka sebelum masuk ke tahap produksi konten.",
		}
	}
	if mode == "public_speaking" {
		return []string{
			fmt.Sprintf("Peserta didik memahami bahwa keterampilan berbicara pada topik %s tidak hanya bergantung pada isi, tetapi juga pada cara penyampaian.", topic),
			"Peserta didik memahami bahwa artikulasi, intonasi, kontak mata, dan gesture memengaruhi kejelasan pesan yang diterima audiens.",
			"Peserta didik memahami bahwa kepercayaan diri dapat dibangun melalui latihan, umpan balik, dan perbaikan performa secara bertahap.",
		}
	}
	return []string{
		fmt.Sprintf("Peserta didik memahami bahwa %s merupakan bagian penting dari pembelajaran %s.", topic, category),
		fmt.Sprintf("Peserta didik mampu menghubungkan konsep %s dengan situasi nyata atau konteks yang relevan.", topic),
		"Peserta didik memahami bahwa ketepatan konsep dan proses berpikir sama pentingnya dengan hasil akhir.",
	}
}

func buildTriggerQuestions(topic, category, mode string) []string {
	if mode == "digital_content" {
		return []string{
			fmt.Sprintf("Bagaimana sebuah konten %s dapat menarik perhatian audiens sekaligus tetap menyampaikan pesan yang jelas?", topic),
			"Apa perbedaan karya yang sekadar bagus secara visual dengan karya yang efektif secara komunikasi?",
			"Bagaimana workflow produksi membantu tim kreatif menyelesaikan konten tepat waktu dan tetap berkualitas?",
		}
	}
	if mode == "digital_concept" {
		return []string{
			fmt.Sprintf("Mengapa %s penting dipahami sebelum membuat konten digital?", topic),
			"Bagaimana visual, teks, warna, dan media dapat mengubah cara audiens memahami sebuah pesan?",
			"Apa yang membuat sebuah komunikasi brand mudah dikenali oleh audiens?",
		}
	}
	if mode == "public_speaking" {
		return []string{
			"Mengapa dua orang yang menyampaikan isi sama bisa memberi dampak presentasi yang berbeda?",
			fmt.Sprintf("Apa yang membuat presentasi tentang %s terasa meyakinkan bagi audiens?", topic),
			"Bagaimana cara memperbaiki rasa gugup ketika harus berbicara di depan umum?",
		}
	}
	return []string{
		fmt.Sprintf("Mengapa %s penting dipelajari dalam konteks %s?", topic, category),
		fmt.Sprintf("Bagaimana %s dapat ditemukan atau diterapkan dalam kehidupan sehari-hari?", topic),
		fmt.Sprintf("Apa dampaknya jika konsep %s dipahami secara keliru?", topic),
	}
}

func buildIntroductionActivities(topic string) []string {
	return []string{
		"Guru membuka pembelajaran, menyapa peserta didik, dan membangun kesiapan belajar.",
		fmt.Sprintf("Guru mengaitkan pengalaman peserta didik dengan topik %s.", topic),
		"Guru menyampaikan tujuan pembelajaran dan alur kegiatan yang akan dilakukan.",
		"Guru memantik pengetahuan awal melalui tanya jawab singkat atau kuis pembuka.",
	}
}

func buildCoreActivities(topic, category, mode string) []string {
	if mode == "digital_content" {
		return []string{
			fmt.Sprintf("Guru memberikan brief produksi %s yang memuat tujuan komunikasi, target audiens, platform publikasi, batasan durasi/format, dan tenggat kerja.", topic),
			"Peserta didik menganalisis contoh konten digital sejenis untuk mengidentifikasi kekuatan pesan, gaya visual, storytelling, kualitas editing, dan kesesuaian branding.",
			"Peserta didik menyusun konsep produksi dalam kelompok berupa ide utama, moodboard, storyboard atau shot list, kebutuhan aset, pembagian peran, dan jadwal pengerjaan.",
			"Guru melakukan checkpoint rencana produksi dan memberi umpan balik pada konsep visual, kelayakan teknis, serta kesesuaian dengan brief.",
			"Peserta didik melakukan produksi aset foto, video, audio, ilustrasi, tipografi, atau elemen grafis sesuai peran masing-masing.",
			"Peserta didik melakukan editing menggunakan aplikasi digital yang tersedia dengan memperhatikan ritme visual, audio, transisi, warna, tipografi, dan format platform.",
			"Setiap kelompok melakukan peer review menggunakan rubrik kreativitas, kualitas visual, storytelling, teknis audio/video, dan ketepatan pesan.",
			"Peserta didik merevisi karya berdasarkan umpan balik lalu mempresentasikan hasil akhir sebagai portofolio proyek konten digital.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			fmt.Sprintf("Guru menampilkan beberapa contoh komunikasi industri kreatif yang relevan dengan %s, seperti poster digital, konten media sosial, iklan visual, atau video pendek.", topic),
			"Peserta didik mengamati contoh karya untuk menemukan pesan utama, target audiens, media yang digunakan, unsur visual, dan gaya komunikasi.",
			fmt.Sprintf("Guru menjelaskan pengertian, fungsi, dan unsur utama %s melalui contoh konkret dari karya yang diamati.", topic),
			"Peserta didik mengisi LKPD analisis sederhana dengan memisahkan komunikator, pesan, audiens, media, konteks, visual, dan respons yang diharapkan.",
			"Peserta didik berdiskusi kelompok untuk membandingkan dua contoh komunikasi brand dan menemukan perbedaan strategi pesannya.",
			"Setiap kelompok menyampaikan hasil analisis singkat, lalu guru memberi penguatan pada istilah dan konsep yang masih keliru.",
			"Peserta didik membuat peta konsep ringkas tentang fungsi komunikasi visual dalam industri kreatif sebagai rangkuman belajar.",
		}
	}
	if mode == "public_speaking" {
		return []string{
			fmt.Sprintf("Guru menampilkan contoh singkat presentasi tentang %s dan mengajak peserta didik mengamati unsur artikulasi, intonasi, kontak mata, dan gesture.", topic),
			"Peserta didik mencatat kekuatan dan kelemahan performa contoh menggunakan lembar observasi sederhana.",
			fmt.Sprintf("Guru memodelkan cara menyusun opening, isi singkat, dan closing presentasi pada topik %s.", topic),
			fmt.Sprintf("Peserta didik berlatih menyusun naskah atau kerangka presentasi singkat tentang %s secara berpasangan.", topic),
			"Peserta didik melakukan simulasi presentasi 2 sampai 3 menit secara bergiliran di depan kelompok kecil.",
			"Teman sebaya memberi umpan balik berdasarkan indikator intonasi, kontak mata, gesture, kejelasan isi, dan percaya diri.",
			"Guru memberi koreksi performa secara langsung lalu peserta didik melakukan praktik ulang dengan perbaikan yang disarankan.",
			"Beberapa peserta didik menampilkan presentasi akhir di depan kelas untuk dinilai menggunakan rubrik performa.",
		}
	}
	return []string{
		fmt.Sprintf("Guru menyajikan materi inti atau contoh awal yang berkaitan dengan %s.", topic),
		"Peserta didik mengamati, mencatat, dan mengidentifikasi informasi penting dari paparan awal.",
		fmt.Sprintf("Guru memandu peserta didik mendiskusikan konsep utama pada %s dalam kelompok kecil atau klasikal.", topic),
		"Peserta didik mengerjakan latihan atau tugas terarah sesuai tingkat kesiapan belajarnya.",
		"Guru memberi umpan balik, meluruskan miskonsepsi, dan memperkuat pemahaman peserta didik.",
		fmt.Sprintf("Peserta didik mempresentasikan, menuliskan, atau menunjukkan hasil pemahaman mereka pada topik %s.", topic),
	}
}

func buildClosingActivities(topic string) []string {
	return []string{
		fmt.Sprintf("Guru dan peserta didik menyimpulkan poin penting dari topik %s.", topic),
		"Peserta didik mengisi refleksi singkat tentang pemahaman dan tantangan belajar mereka.",
		"Guru menyampaikan tindak lanjut berupa pengayaan, remedial, atau tugas rumah bila diperlukan.",
	}
}

func buildDiagnosticAssessments(topic, mode string) []string {
	if mode == "digital_content" {
		return []string{
			fmt.Sprintf("Analisis cepat karya contoh %s untuk memetakan pemahaman awal peserta didik tentang pesan, audiens, visual, dan teknis editing.", topic),
			"Checklist kesiapan awal terkait pengalaman menggunakan kamera/smartphone, aplikasi desain/editing, dan kerja kelompok produksi.",
			"Tanya jawab awal tentang workflow produksi konten digital, mulai dari brief, ide, produksi aset, editing, revisi, sampai publikasi.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			fmt.Sprintf("Tanya jawab awal tentang pemahaman peserta didik terhadap istilah %s dan contoh komunikasi visual yang pernah mereka lihat.", topic),
			"Identifikasi cepat kemampuan peserta didik dalam membedakan pesan, audiens, media, dan unsur visual pada contoh konten.",
			"Observasi respon peserta didik saat membaca contoh poster, feed media sosial, atau iklan visual sederhana.",
		}
	}
	if mode == "public_speaking" {
		return []string{
			"Observasi awal saat peserta didik memperkenalkan diri singkat untuk memetakan keberanian, volume suara, dan kontak mata.",
			fmt.Sprintf("Tanya jawab awal untuk mengetahui pengalaman peserta didik saat berbicara tentang %s di depan orang lain.", topic),
			"Checklist kesiapan awal terkait struktur pesan, kejelasan suara, dan bahasa tubuh dasar.",
		}
	}
	return []string{
		fmt.Sprintf("Tanya jawab awal untuk mengecek pemahaman dasar peserta didik terhadap %s.", topic),
		"Pertanyaan cepat atau kuis singkat untuk memetakan kesiapan belajar.",
		"Observasi respon awal peserta didik saat menghadapi contoh atau masalah pembuka.",
	}
}

func buildFormativeAssessments(topic, mode string) []string {
	if mode == "digital_content" {
		return []string{
			"Penilaian checkpoint konsep: kesesuaian ide dengan brief, kejelasan audiens, kekuatan pesan, moodboard, storyboard atau shot list, dan kelayakan jadwal produksi.",
			"Observasi proses produksi: kolaborasi tim, pembagian peran, pengambilan aset, manajemen file, penggunaan aplikasi, dan ketepatan mengikuti workflow.",
			"Umpan balik tengah proses pada draft karya berdasarkan kreativitas, komposisi visual, storytelling, kualitas audio/video, dan konsistensi branding.",
			"Peer review sebelum finalisasi menggunakan rubrik sederhana agar peserta didik melakukan revisi berbasis bukti, bukan hanya selera pribadi.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			"Penilaian LKPD analisis contoh karya berdasarkan ketepatan mengidentifikasi pesan, audiens, media, dan unsur visual.",
			"Observasi diskusi kelompok saat peserta didik membandingkan contoh komunikasi brand atau konten industri kreatif.",
			"Umpan balik guru terhadap peta konsep yang dibuat peserta didik agar istilah dan hubungan konsep lebih tepat.",
			"Presentasi singkat hasil analisis kelompok untuk melihat kejelasan argumen dan penggunaan istilah DKV.",
		}
	}
	if mode == "public_speaking" {
		return []string{
			fmt.Sprintf("Observasi performa saat simulasi presentasi %s menggunakan indikator artikulasi, intonasi, kontak mata, gesture, dan struktur penyampaian.", topic),
			"Penilaian teman sebaya terhadap opening, isi, dan closing presentasi menggunakan rubrik sederhana skala 1 sampai 4.",
			"Umpan balik lisan guru setelah latihan pertama, lalu dibandingkan dengan latihan ulang untuk melihat perbaikan performa.",
			"Catatan perkembangan individu terkait keberanian tampil, kelancaran berbicara, dan respons terhadap audiens.",
		}
	}
	return []string{
		fmt.Sprintf("Penilaian proses saat peserta didik mengerjakan latihan terkait %s.", topic),
		"Observasi partisipasi diskusi, ketepatan langkah, dan kejelasan jawaban peserta didik.",
		"Umpan balik lisan atau tertulis terhadap hasil kerja sementara peserta didik.",
		"Pertanyaan pemantauan selama kegiatan inti untuk memastikan tujuan belajar bergerak ke arah yang benar.",
	}
}

func buildSummativeAssessments(topic, mode string) []string {
	if mode == "digital_content" {
		return []string{
			fmt.Sprintf("Produk akhir berupa konten digital %s yang dipresentasikan sebagai portofolio kelompok atau individu sesuai brief.", topic),
			"Rubrik produk skala 1 sampai 4 mencakup kreativitas ide, efektivitas pesan, kualitas visual, storytelling, editing, kualitas audio/video, konsistensi branding, dan ketepatan format platform.",
			"Rubrik proses skala 1 sampai 4 mencakup riset referensi, perencanaan produksi, kolaborasi, kerapian manajemen aset, ketepatan deadline, dan kemampuan melakukan revisi.",
			"Presentasi karya dinilai dari kejelasan alasan konsep, kesesuaian dengan brief, kemampuan menerima umpan balik, dan refleksi perbaikan karya.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			fmt.Sprintf("Tugas akhir berupa analisis satu contoh komunikasi industri kreatif terkait %s menggunakan format LKPD.", topic),
			"Produk akhir berupa peta konsep atau ringkasan visual yang memuat pengertian, fungsi, unsur komunikasi, media, audiens, dan contoh penerapan brand.",
			"Presentasi singkat hasil analisis dinilai dari ketepatan konsep, contoh, alasan, dan kejelasan penyampaian.",
		}
	}
	if mode == "public_speaking" {
		return []string{
			fmt.Sprintf("Tugas akhir berupa presentasi individu 2 sampai 3 menit tentang %s di depan kelas.", topic),
			"Rubrik penilaian mencakup struktur penyampaian, intonasi, artikulasi, kontak mata, gesture, penguasaan isi, dan rasa percaya diri dengan skala 1 sampai 4.",
			"Nilai akhir diambil dari performa presentasi, kualitas naskah/kerangka singkat, dan kemampuan melakukan refleksi setelah tampil.",
		}
	}
	return []string{
		fmt.Sprintf("Tes singkat atau tugas akhir untuk mengukur pemahaman peserta didik terhadap %s.", topic),
		"Produk akhir atau jawaban tertulis yang dinilai berdasarkan ketepatan konsep dan proses.",
		"Rubrik sederhana untuk menilai ketercapaian tujuan pembelajaran pada akhir pertemuan.",
	}
}

func buildRubricCriteria(topic, mode string) []string {
	topic = fallbackText(topic, "topik pembelajaran")
	switch mode {
	case "digital_content":
		return []string{
			"Kesesuaian brief dan pesan: 4=sangat sesuai dengan audiens, platform, tujuan komunikasi, dan pesan utama; 3=sesuai tetapi masih ada bagian yang kurang kuat; 2=sebagian sesuai namun pesan kurang fokus; 1=tidak sesuai brief atau pesan sulit dipahami.",
			"Kreativitas konsep dan storytelling: 4=ide orisinal, alur kuat, dan menarik; 3=ide cukup menarik dan alur jelas; 2=ide masih umum dan alur kurang rapi; 1=ide tidak berkembang atau tidak memiliki alur.",
			"Kualitas visual dan branding: 4=komposisi, warna, tipografi, aset, dan identitas visual konsisten; 3=visual cukup rapi dengan sedikit inkonsistensi; 2=visual kurang rapi dan branding lemah; 1=visual tidak tertata dan tidak mendukung pesan.",
			"Teknis produksi/editing: 4=editing halus, audio/video jelas, format tepat, dan file siap publikasi; 3=teknis cukup baik dengan minor error; 2=masih ada gangguan teknis yang memengaruhi kualitas; 1=hasil tidak memenuhi format atau banyak error teknis.",
			"Workflow dan ketepatan deadline: 4=brief, moodboard, storyboard/shot list, aset, revisi, dan final lengkap tepat waktu; 3=sebagian besar lengkap tepat waktu; 2=workflow kurang rapi atau terlambat; 1=proses tidak terdokumentasi dan tidak selesai tepat waktu.",
			"Presentasi portofolio: 4=mampu menjelaskan konsep, proses, keputusan desain, dan revisi dengan kuat; 3=penjelasan cukup jelas; 2=penjelasan kurang lengkap; 1=tidak mampu menjelaskan proses/karya.",
		}
	case "digital_concept":
		return []string{
			"Ketepatan konsep: 4=menjelaskan pengertian, fungsi, dan unsur komunikasi dengan sangat tepat; 3=sebagian besar tepat; 2=masih ada konsep yang tertukar; 1=belum memahami konsep dasar.",
			"Analisis contoh karya: 4=mampu mengidentifikasi pesan, audiens, media, visual, dan tujuan brand secara lengkap; 3=cukup lengkap; 2=sebagian unsur belum tepat; 1=analisis tidak sesuai contoh.",
			"Penggunaan contoh industri kreatif: 4=contoh relevan dan alasan kuat; 3=contoh relevan tetapi alasan belum mendalam; 2=contoh kurang relevan; 1=tidak memberi contoh yang jelas.",
			"Peta konsep/LKPD: 4=rapi, runtut, dan hubungan konsep jelas; 3=cukup rapi; 2=kurang runtut; 1=tidak lengkap.",
			"Presentasi hasil analisis: 4=penyampaian jelas, istilah tepat, dan argumentasi kuat; 3=cukup jelas; 2=masih banyak membaca atau kurang runtut; 1=tidak mampu menjelaskan hasil analisis.",
		}
	case "public_speaking":
		return []string{
			"Struktur penyampaian: 4=opening, isi, dan closing sangat jelas; 3=struktur jelas; 2=struktur kurang runtut; 1=tidak terstruktur.",
			"Vokal dan artikulasi: 4=suara jelas, intonasi tepat, artikulasi kuat; 3=cukup jelas; 2=sering kurang jelas; 1=sulit dipahami.",
			"Kontak mata dan gesture: 4=sangat mendukung pesan; 3=cukup sesuai; 2=kurang konsisten; 1=tidak mendukung penyampaian.",
			"Kepercayaan diri dan penguasaan materi: 4=sangat percaya diri dan menguasai isi; 3=cukup percaya diri; 2=masih banyak ragu; 1=tidak siap tampil.",
		}
	case "practice":
		return []string{
			"Ketepatan prosedur: 4=seluruh langkah runtut dan tepat; 3=hampir semua langkah tepat; 2=beberapa langkah keliru; 1=prosedur tidak diikuti.",
			"Kualitas produk/performa: 4=memenuhi seluruh kriteria; 3=memenuhi sebagian besar kriteria; 2=memenuhi sebagian kecil kriteria; 1=belum memenuhi kriteria.",
			"Kemandirian dan keselamatan kerja: 4=mandiri dan aman; 3=cukup mandiri; 2=sering perlu bantuan; 1=tidak mandiri atau mengabaikan keselamatan.",
			"Refleksi dan perbaikan: 4=perbaikan jelas berbasis umpan balik; 3=ada perbaikan; 2=perbaikan terbatas; 1=tidak ada perbaikan.",
		}
	default:
		return []string{
			"Ketepatan konsep: 4=sangat tepat dan lengkap; 3=tepat dengan sedikit kekurangan; 2=sebagian tepat; 1=belum tepat.",
			"Kejelasan penyajian: 4=sangat runtut dan mudah dipahami; 3=cukup runtut; 2=kurang runtut; 1=tidak jelas.",
			"Penerapan pada tugas: 4=mampu menerapkan secara mandiri; 3=mampu menerapkan dengan sedikit bantuan; 2=membutuhkan banyak bantuan; 1=belum mampu menerapkan.",
			"Refleksi belajar: 4=refleksi spesifik dan menunjukkan rencana perbaikan; 3=refleksi cukup jelas; 2=refleksi umum; 1=tidak ada refleksi bermakna.",
		}
	}
}

func buildDifferentiationContent(topic, mode string) []string {
	if mode == "digital_content" {
		return []string{
			"Peserta didik yang membutuhkan dukungan mendapat contoh brief, storyboard, dan template workflow yang lebih terstruktur.",
			"Peserta didik yang lebih siap diberi referensi visual yang lebih kompleks dan tantangan eksplorasi gaya desain atau editing.",
			"Materi penguatan mencakup contoh karya baik dan kurang baik agar peserta didik dapat membandingkan kualitas visual, pesan, dan teknis produksi.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			"Peserta didik yang membutuhkan dukungan mendapat contoh karya dengan anotasi pesan, audiens, media, dan unsur visual.",
			"Peserta didik yang lebih siap diberi contoh komunikasi brand yang lebih kompleks untuk dianalisis secara mandiri.",
			"Guru menyediakan daftar istilah DKV sederhana agar peserta didik dapat memakai kosakata yang tepat saat berdiskusi.",
		}
	}
	return []string{
		fmt.Sprintf("Guru menyediakan contoh dasar dan contoh lanjutan pada materi %s.", topic),
		"Peserta didik dengan kesiapan belajar berbeda memperoleh bahan bantu yang berbeda sesuai kebutuhannya.",
		"Materi inti dan penguatan disesuaikan agar tetap dapat diakses seluruh peserta didik.",
	}
}

func buildDifferentiationProcess(topic, mode string) []string {
	if mode == "digital_content" {
		return []string{
			"Kelompok dapat dibagi berdasarkan peran produksi seperti penulis konsep, desainer visual, videografer, editor, audio, dan presenter.",
			"Guru memberi checkpoint bertahap mulai dari brief, konsep, produksi aset, draft editing, revisi, hingga presentasi portofolio.",
			"Peserta didik dapat memilih aplikasi yang dikuasai selama hasil memenuhi brief, format, dan rubrik kualitas karya.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			"Peserta didik dapat menganalisis contoh karya secara individu, berpasangan, atau kelompok kecil sesuai kesiapan belajar.",
			"Guru memberi pertanyaan pemandu bertahap dari pengertian, unsur, fungsi, sampai contoh komunikasi brand.",
			"Kelompok yang lebih cepat selesai diarahkan membandingkan dua media komunikasi yang berbeda.",
		}
	}
	return []string{
		fmt.Sprintf("Peserta didik dapat mempelajari %s melalui diskusi, latihan mandiri, atau bimbingan guru.", topic),
		"Guru memberi variasi ritme kerja, tingkat bantuan, dan bentuk pendampingan selama proses belajar.",
		"Pengelompokan belajar dapat disesuaikan dengan kesiapan, minat, atau gaya belajar peserta didik.",
	}
}

func buildDifferentiationProduct(topic, mode string) []string {
	if mode == "digital_content" {
		return []string{
			fmt.Sprintf("Produk dapat berupa poster digital, konten video pendek, desain feed, motion sederhana, atau format lain yang sesuai dengan brief %s.", topic),
			"Peserta didik boleh memilih format produk sesuai minat dan sarana, tetapi tetap dinilai dengan rubrik pesan, visual, teknis, dan ketepatan format.",
			"Portofolio akhir dilengkapi penjelasan konsep, proses produksi, revisi, dan refleksi kualitas karya.",
		}
	}
	if mode == "digital_concept" {
		return []string{
			"Produk belajar dapat berupa LKPD analisis, peta konsep, atau presentasi singkat tentang contoh komunikasi industri kreatif.",
			"Peserta didik dapat memilih contoh karya dari poster, feed media sosial, iklan visual, atau video pendek yang relevan.",
			"Penilaian menekankan ketepatan konsep dan kualitas analisis, bukan kemampuan produksi desain penuh.",
		}
	}
	return []string{
		fmt.Sprintf("Peserta didik dapat menunjukkan pemahaman %s melalui jawaban tertulis, presentasi, atau produk sederhana.", topic),
		"Guru memberi ruang bagi peserta didik memilih bentuk hasil kerja yang paling sesuai.",
		"Penilaian tetap menekankan kualitas konsep meskipun bentuk produk berbeda.",
	}
}
