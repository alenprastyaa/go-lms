package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"lms/utils"
)

const defaultQuestionIllustrationModel = "black-forest-labs/FLUX.1-schnell"

func generateQuestionIllustrationURL(input QuestionBankAIInput, item QuestionBankAIItem, index int) (string, error) {
	apiKey := huggingFaceAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("HF_API_KEY belum diatur di server")
	}

	prompt := buildQuestionIllustrationPrompt(input, item)
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt ilustrasi kosong")
	}

	model := huggingFaceImageModel()
	apiURL := huggingFaceImageAPIURL(model)

	payload := map[string]interface{}{
		"inputs": prompt,
		"parameters": map[string]interface{}{
			"width":               768,
			"height":              768,
			"num_inference_steps": 4,
			"guidance_scale":      3.5,
			"negative_prompt":     "teks, tulisan, huruf, angka, kata, caption, label, watermark, logo, subtitle, paragraf, kalimat",
			"seed":                time.Now().UnixNano() + int64(index),
		},
	}

	body, err := jsonMarshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(imageBytes) == 0 {
			return "", fmt.Errorf("request ilustrasi AI gagal dengan status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("request ilustrasi AI gagal dengan status %d: %s", resp.StatusCode, strings.TrimSpace(string(imageBytes)))
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(imageBytes)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", fmt.Errorf("respon ilustrasi AI bukan gambar: %s", contentType)
	}

	if _, format, err := image.DecodeConfig(bytes.NewReader(imageBytes)); err != nil || strings.TrimSpace(format) == "" {
		if err != nil {
			return "", fmt.Errorf("hasil ilustrasi AI tidak valid: %w", err)
		}
	}

	fileName := buildQuestionIllustrationFileName(input, item, index, contentType)
	url, err := utils.UploadBytesToR2(context.Background(), imageBytes, fileName, contentType)
	if err != nil {
		return "", err
	}

	return url, nil
}

func populateQuestionBankIllustrations(input QuestionBankAIInput, items []QuestionBankAIItem) []QuestionBankAIItem {
	if !input.IncludeIllustration {
		return items
	}

	for index := range items {
		if strings.TrimSpace(items[index].QuestionImageURL) != "" {
			continue
		}

		url, err := generateQuestionIllustrationURL(input, items[index], index)
		if err != nil {
			continue
		}
		items[index].QuestionImageURL = url
	}

	return items
}

func buildQuestionIllustrationPrompt(input QuestionBankAIInput, item QuestionBankAIItem) string {
	questionText := normalizeIllustrationQuestionText(item.QuestionText)
	baseParts := []string{
		"Buat ilustrasi edukatif untuk soal sekolah.",
		fmt.Sprintf("Mapel: %s.", fallbackText(input.SubjectName, "-")),
		fmt.Sprintf("Kelas: %s.", fallbackText(input.ClassName, "-")),
		fmt.Sprintf("Topik: %s.", fallbackText(input.Topic, "-")),
		fmt.Sprintf("Jenis soal: %s.", normalizeQuestionType(input.QuestionType)),
		fmt.Sprintf("Konsep soal: %s.", questionText),
		"Gaya visual: bersih, modern, rapi, mudah dipahami siswa.",
		"Komposisi harus jelas dan relevan dengan isi soal.",
		"Jangan menampilkan tulisan apa pun di dalam gambar, termasuk huruf, angka, judul, caption, opsi jawaban, atau rumus tertulis.",
		"Jika ada rumus, tampilkan sebagai konsep visual, bukan teks rumus yang rumit.",
	}

	if input.IncludeIllustration {
		baseParts = append(baseParts,
			"Fokus pada objek utama yang paling membantu memahami soal.",
			"Buat ilustrasi yang tampak seperti materi pembelajaran yang siap dipakai di LMS sekolah.",
		)
	}

	return strings.Join(baseParts, " ")
}

func buildQuestionIllustrationFileName(input QuestionBankAIInput, item QuestionBankAIItem, index int, contentType string) string {
	ext := imageExtensionFromContentType(contentType)
	subject := slugifyForFileName(input.SubjectName)
	topic := slugifyForFileName(input.Topic)
	question := slugifyForFileName(item.QuestionText)
	if subject == "" {
		subject = "subject"
	}
	if topic == "" {
		topic = "topic"
	}
	if question == "" {
		question = fmt.Sprintf("question-%d", index+1)
	}
	return fmt.Sprintf("ai-illustration-%s-%s-%s-%d%s", subject, topic, question, index+1, ext)
}

func imageExtensionFromContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func normalizeIllustrationQuestionText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "-"
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		text = text[:240] + "..."
	}
	return text
}

func slugifyForFileName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func huggingFaceImageModel() string {
	if model := strings.TrimSpace(os.Getenv("HUGGINGFACE_IMAGE_MODEL")); model != "" {
		return model
	}
	if model := strings.TrimSpace(os.Getenv("HF_IMAGE_MODEL")); model != "" {
		return model
	}
	return defaultQuestionIllustrationModel
}

func huggingFaceImageAPIURL(model string) string {
	if endpoint := strings.TrimSpace(os.Getenv("HUGGINGFACE_IMAGE_API_URL")); endpoint != "" {
		return endpoint
	}
	if endpoint := strings.TrimSpace(os.Getenv("HF_IMAGE_API_URL")); endpoint != "" {
		return endpoint
	}
	return "https://router.huggingface.co/hf-inference/models/" + strings.TrimSpace(model)
}

func jsonMarshal(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}
