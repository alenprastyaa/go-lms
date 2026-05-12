package services

import (
	"fmt"
	"strings"
)

type SystemChatbotInput struct {
	Role     string
	Question string
	Teachers int
	Students int
	Classes  int
	Subjects int
}

func buildSystemChatbotPrompt(input SystemChatbotInput) string {
	role := strings.ToUpper(strings.TrimSpace(input.Role))
	if role == "" {
		role = "PENGGUNA"
	}

	return strings.Join([]string{
		"Anda adalah asisten penggunaan LMS sekolah untuk Admin dan Guru.",
		"Jawab dalam Bahasa Indonesia yang sederhana, ramah, dan mudah dipahami orang awam teknologi.",
		"Jangan gunakan istilah teknis yang rumit. Jika terpaksa, jelaskan artinya dengan singkat.",
		"Berikan langkah yang praktis dan langsung bisa dilakukan di menu sistem.",
		"Jika pertanyaan kurang jelas, tetap bantu dengan jawaban terbaik lalu sarankan pertanyaan lanjutan.",
		"Jangan balas dalam format JSON, array, object, atau markdown code block.",
		"Balas sebagai teks biasa yang rapi dan enak dibaca.",
		fmt.Sprintf("Peran pengguna saat ini: %s.", role),
		fmt.Sprintf("Data ringkas sekolah saat ini: %d guru, %d siswa, %d kelas, %d mata pelajaran.", input.Teachers, input.Students, input.Classes, input.Subjects),
		"Menu yang tersedia antara lain: Dashboard, User Sekolah, Kelas, Siswa, Tahun Ajaran, Kurikulum, Ujian Resmi, Billing, Setting, Pembelajaran, Live Chat, Bank Soal, Quiz, Penilaian, Rapor.",
		fmt.Sprintf("Pertanyaan pengguna: %s", strings.TrimSpace(input.Question)),
		"Format jawaban yang diinginkan:",
		"Mulai dengan jawaban inti singkat 2 sampai 4 kalimat.",
		"Jika relevan, lanjutkan dengan judul 'Langkah singkat:' lalu daftar bernomor 1 sampai 4.",
		"Akhiri dengan satu kalimat pertanyaan lanjutan.",
	}, "\n")
}

func GenerateSystemChatbotAnswer(input SystemChatbotInput) (string, error) {
	systemMessage := "Anda adalah asisten LMS sekolah yang menjawab secara praktis dan sederhana untuk pengguna non-teknis."
	return callHuggingFace(buildSystemChatbotPrompt(input), systemMessage, 0.55)
}
