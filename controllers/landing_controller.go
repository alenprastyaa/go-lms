package controllers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"lms/models"
	"lms/utils"
)

type landingSectionPayload struct {
	Label     string             `json:"label"`
	Content   models.LandingJSON `json:"content"`
	IsActive  *bool              `json:"is_active"`
	SortOrder int                `json:"sort_order"`
}

type landingBlogPayload struct {
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	Excerpt         string `json:"excerpt"`
	Content         string `json:"content"`
	CoverImageURL   string `json:"cover_image_url"`
	AuthorName      string `json:"author_name"`
	Category        string `json:"category"`
	Tags            string `json:"tags"`
	MetaTitle       string `json:"meta_title"`
	MetaDescription string `json:"meta_description"`
	IsPublished     *bool  `json:"is_published"`
	IsFeatured      bool   `json:"is_featured"`
	SortOrder       int    `json:"sort_order"`
	PublishedAt     string `json:"published_at"`
}

func (a *AppContext) GetPublicLandingContent(c *fiber.Ctx) error {
	var sections []models.LandingSection
	if err := a.DB.Where("is_active = ?", true).Order("sort_order ASC, section_key ASC").Find(&sections).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat konten landing", err.Error())
	}
	content := fiber.Map{}
	for _, section := range sections {
		content[section.Key] = section.Content
	}

	var posts []models.LandingBlogPost
	if err := a.DB.Where("is_published = ?", true).
		Order("is_featured DESC, sort_order ASC, COALESCE(published_at, created_at) DESC, id DESC").
		Limit(3).
		Find(&posts).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat blog landing", err.Error())
	}

	return utils.Success(c, 200, "Konten landing publik", fiber.Map{
		"sections": content,
		"blog":     posts,
	})
}

func (a *AppContext) GetLandingSections(c *fiber.Ctx) error {
	var sections []models.LandingSection
	if err := a.DB.Order("sort_order ASC, section_key ASC").Find(&sections).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat section landing", err.Error())
	}
	return utils.Success(c, 200, "Daftar section landing", fiber.Map{"sections": sections})
}

func (a *AppContext) UpdateLandingSection(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	if key == "" {
		return utils.Error(c, 400, "Section key wajib diisi")
	}

	var body landingSectionPayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Payload section tidak valid")
	}
	if body.Content == nil {
		body.Content = models.LandingJSON{}
	}

	var current models.LandingSection
	err := a.DB.Where("section_key = ?", key).First(&current).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return utils.Error(c, 500, "Gagal memuat section landing", err.Error())
	}
	if err == gorm.ErrRecordNotFound {
		current = models.LandingSection{Key: key, IsActive: true}
	}

	current.Label = strings.TrimSpace(body.Label)
	if current.Label == "" {
		current.Label = key
	}
	current.Content = body.Content
	current.SortOrder = body.SortOrder
	if body.IsActive != nil {
		current.IsActive = *body.IsActive
	}

	if current.ID == 0 {
		if err := a.DB.Create(&current).Error; err != nil {
			return utils.Error(c, 500, "Gagal membuat section landing", err.Error())
		}
	} else if err := a.DB.Model(&current).Updates(map[string]interface{}{
		"label":      current.Label,
		"content":    current.Content,
		"is_active":  current.IsActive,
		"sort_order": current.SortOrder,
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui section landing", err.Error())
	}

	return utils.Success(c, 200, "Section landing berhasil disimpan", fiber.Map{"section": current})
}

func (a *AppContext) UploadLandingAsset(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return utils.Error(c, 400, "File wajib diupload")
	}
	url, upErr := utils.SaveUploadedFile(c, file)
	if upErr != nil {
		return utils.Error(c, 500, "Gagal upload asset landing", upErr.Error())
	}
	return utils.Success(c, 200, "Asset landing berhasil diupload", fiber.Map{"url": url})
}

func (a *AppContext) GetPublicBlogPosts(c *fiber.Ctx) error {
	limit := parseLandingLimit(c.Query("limit"), 12)
	var posts []models.LandingBlogPost
	if err := a.DB.Where("is_published = ?", true).
		Order("is_featured DESC, sort_order ASC, COALESCE(published_at, created_at) DESC, id DESC").
		Limit(limit).
		Find(&posts).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat blog", err.Error())
	}
	return utils.Success(c, 200, "Daftar blog publik", fiber.Map{"posts": posts})
}

func (a *AppContext) GetPublicBlogPost(c *fiber.Ctx) error {
	slug := strings.TrimSpace(c.Params("slug"))
	var post models.LandingBlogPost
	if err := a.DB.Where("slug = ? AND is_published = ?", slug, true).First(&post).Error; err != nil {
		return utils.Error(c, 404, "Artikel tidak ditemukan")
	}
	return utils.Success(c, 200, "Detail blog publik", fiber.Map{"post": post})
}

func (a *AppContext) GetLandingBlogPosts(c *fiber.Ctx) error {
	var posts []models.LandingBlogPost
	if err := a.DB.Order("is_featured DESC, sort_order ASC, COALESCE(published_at, created_at) DESC, id DESC").Find(&posts).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat daftar blog", err.Error())
	}
	return utils.Success(c, 200, "Daftar blog", fiber.Map{"posts": posts})
}

func (a *AppContext) CreateLandingBlogPost(c *fiber.Ctx) error {
	var body landingBlogPayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Payload blog tidak valid")
	}
	post, err := landingBlogPostFromPayload(body)
	if err != nil {
		return utils.Error(c, 422, err.Error())
	}
	if post.IsPublished && post.PublishedAt == nil {
		now := jakartaNow()
		post.PublishedAt = &now
	}
	if err := a.DB.Create(&post).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat blog", err.Error())
	}
	return utils.Success(c, 201, "Blog berhasil dibuat", fiber.Map{"post": post})
}

func (a *AppContext) UpdateLandingBlogPost(c *fiber.Ctx) error {
	id := c.Params("id")
	var current models.LandingBlogPost
	if err := a.DB.Where("id = ?", id).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Blog tidak ditemukan")
	}
	var body landingBlogPayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Payload blog tidak valid")
	}
	next, err := landingBlogPostFromPayload(body)
	if err != nil {
		return utils.Error(c, 422, err.Error())
	}
	if next.IsPublished && current.PublishedAt == nil && next.PublishedAt == nil {
		now := jakartaNow()
		next.PublishedAt = &now
	}
	if err := a.DB.Model(&current).Updates(map[string]interface{}{
		"title":            next.Title,
		"slug":             next.Slug,
		"excerpt":          next.Excerpt,
		"content":          next.Content,
		"cover_image_url":  next.CoverImageURL,
		"author_name":      next.AuthorName,
		"category":         next.Category,
		"tags":             next.Tags,
		"meta_title":       next.MetaTitle,
		"meta_description": next.MetaDescription,
		"is_published":     next.IsPublished,
		"is_featured":      next.IsFeatured,
		"sort_order":       next.SortOrder,
		"published_at":     next.PublishedAt,
		"updated_at":       gorm.Expr("NOW()"),
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui blog", err.Error())
	}
	next.ID = current.ID
	return utils.Success(c, 200, "Blog berhasil diperbarui", fiber.Map{"post": next})
}

func (a *AppContext) DeleteLandingBlogPost(c *fiber.Ctx) error {
	id := c.Params("id")
	result := a.DB.Delete(&models.LandingBlogPost{}, "id = ?", id)
	if result.Error != nil {
		return utils.Error(c, 500, "Gagal menghapus blog", result.Error.Error())
	}
	if result.RowsAffected == 0 {
		return utils.Error(c, 404, "Blog tidak ditemukan")
	}
	return utils.Success(c, 200, "Blog berhasil dihapus", fiber.Map{"id": id})
}

func landingBlogPostFromPayload(body landingBlogPayload) (models.LandingBlogPost, error) {
	title := strings.TrimSpace(body.Title)
	if title == "" {
		return models.LandingBlogPost{}, fmt.Errorf("Judul blog wajib diisi")
	}
	slug := slugifyLandingBlog(firstNonBlank(body.Slug, title))
	if slug == "" {
		return models.LandingBlogPost{}, fmt.Errorf("Slug blog tidak valid")
	}
	isPublished := false
	if body.IsPublished != nil {
		isPublished = *body.IsPublished
	}
	var publishedAt *time.Time
	if strings.TrimSpace(body.PublishedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(body.PublishedAt)); err == nil {
			publishedAt = &parsed
		} else if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(body.PublishedAt)); err == nil {
			publishedAt = &parsed
		}
	}
	return models.LandingBlogPost{
		Title:           title,
		Slug:            slug,
		Excerpt:         strings.TrimSpace(body.Excerpt),
		Content:         strings.TrimSpace(body.Content),
		CoverImageURL:   strings.TrimSpace(body.CoverImageURL),
		AuthorName:      firstNonBlank(strings.TrimSpace(body.AuthorName), "Bitwize Team"),
		Category:        strings.TrimSpace(body.Category),
		Tags:            strings.TrimSpace(body.Tags),
		MetaTitle:       strings.TrimSpace(body.MetaTitle),
		MetaDescription: strings.TrimSpace(body.MetaDescription),
		IsPublished:     isPublished,
		IsFeatured:      body.IsFeatured,
		SortOrder:       body.SortOrder,
		PublishedAt:     publishedAt,
	}, nil
}

func slugifyLandingBlog(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	return strings.Trim(re.ReplaceAllString(raw, "-"), "-")
}

func parseLandingLimit(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	if value > 50 {
		return 50
	}
	return value
}
