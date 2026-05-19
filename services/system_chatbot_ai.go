package services

import (
	"fmt"
	"strings"
)

type SystemChatbotInput struct {
	Question string
}

func buildSystemChatbotPrompt(input SystemChatbotInput) string {
	return strings.Join([]string{
		"Anda adalah asisten umum yang bisa menjawab pertanyaan dari berbagai topik.",
		"Jawab dalam Bahasa Indonesia yang sederhana, ramah, dan mudah dipahami orang awam teknologi.",
		"Jangan gunakan istilah teknis yang rumit. Jika terpaksa, jelaskan artinya dengan singkat.",
		"Berikan langkah yang praktis dan langsung bisa diterapkan jika pertanyaannya membutuhkan langkah.",
		"Jika pertanyaan kurang jelas, tetap bantu dengan jawaban terbaik lalu sarankan pertanyaan lanjutan.",
		"Jika pertanyaannya meminta informasi yang bisa berubah atau perlu verifikasi, sebutkan bahwa jawaban dapat berbeda tergantung konteks atau waktu.",
		"Jika pertanyaannya berbahaya, ilegal, atau melanggar privasi, tolak secara singkat lalu arahkan ke bantuan yang aman.",
		"Jangan balas dalam format JSON, array, object, atau markdown code block.",
		"Balas sebagai teks biasa yang rapi dan enak dibaca.",
		fmt.Sprintf("Pertanyaan pengguna: %s", strings.TrimSpace(input.Question)),
		"Format jawaban yang diinginkan:",
		"Mulai dengan jawaban inti singkat 2 sampai 4 kalimat.",
		"Jika relevan, lanjutkan dengan judul 'Langkah singkat:' lalu daftar bernomor 1 sampai 4.",
		"Akhiri dengan satu kalimat pertanyaan lanjutan.",
	}, "\n")
}

func GenerateSystemChatbotAnswer(input SystemChatbotInput) (string, error) {
	systemMessage := "Anda adalah asisten umum yang menjawab secara praktis, aman, dan sederhana untuk pengguna non-teknis."
	return callHuggingFace(buildSystemChatbotPrompt(input), systemMessage, 0.55)
}
