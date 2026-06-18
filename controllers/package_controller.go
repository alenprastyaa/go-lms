package controllers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"lms/models"
	"lms/utils"
)

type packagePayload struct {
	Name          string                 `json:"name"`
	Tagline       string                 `json:"tagline"`
	Price         float64                `json:"price"`
	OriginalPrice *float64               `json:"original_price"`
	BillingPeriod string                 `json:"billing_period"`
	BadgeText     string                 `json:"badge_text"`
	IsPopular     bool                   `json:"is_popular"`
	IsActive      *bool                  `json:"is_active"`
	SortOrder     int                    `json:"sort_order"`
	CTALabel      string                 `json:"cta_label"`
	CTAURL        string                 `json:"cta_url"`
	Modules       []models.PackageModule `json:"modules"`
}

func packageModuleKey(label string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(label))), "_")
}

func sanitizePackageModules(in []models.PackageModule) models.PackageModules {
	out := models.PackageModules{}
	for _, m := range in {
		label := strings.TrimSpace(m.Label)
		if label == "" || packageModuleKey(label) == "guru_personal" {
			continue
		}
		out = append(out, models.PackageModule{
			Label:    label,
			Icon:     strings.TrimSpace(m.Icon),
			Included: m.Included,
		})
	}
	return out
}

func sanitizePackageItems(items []models.Package) []models.Package {
	for i := range items {
		items[i].Modules = sanitizePackageModules([]models.PackageModule(items[i].Modules))
	}
	return items
}

func sanitizePackageItem(item models.Package) models.Package {
	item.Modules = sanitizePackageModules([]models.PackageModule(item.Modules))
	return item
}

func applyPackagePayload(p *models.Package, body packagePayload) {
	p.Name = strings.TrimSpace(body.Name)
	p.Tagline = strings.TrimSpace(body.Tagline)
	p.Price = body.Price
	p.OriginalPrice = body.OriginalPrice
	p.BillingPeriod = strings.TrimSpace(body.BillingPeriod)
	if p.BillingPeriod == "" {
		p.BillingPeriod = "bulan"
	}
	p.BadgeText = strings.TrimSpace(body.BadgeText)
	p.IsPopular = body.IsPopular
	p.SortOrder = body.SortOrder
	p.CTALabel = strings.TrimSpace(body.CTALabel)
	if p.CTALabel == "" {
		p.CTALabel = "Pilih Paket"
	}
	p.CTAURL = strings.TrimSpace(body.CTAURL)
	p.Modules = sanitizePackageModules(body.Modules)
}

func (a *AppContext) UploadPackageLogo(c *fiber.Ctx) error {
	id := c.Params("id")

	var current models.Package
	if err := a.DB.Where("id = ?", id).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Paket tidak ditemukan")
	}

	file, err := c.FormFile("logo")
	if err != nil {
		return utils.Error(c, 400, "Logo paket wajib diupload")
	}

	logoURL, upErr := utils.SaveUploadedFile(c, file)
	if upErr != nil {
		return utils.Error(c, 500, "Gagal upload logo paket", upErr.Error())
	}

	if err := a.DB.Model(&current).Updates(map[string]interface{}{
		"logo_url":   logoURL,
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan logo paket", err.Error())
	}
	current.LogoURL = logoURL

	return utils.Success(c, 200, "Logo paket berhasil diupload", fiber.Map{"package": current, "logo_url": logoURL})
}

func (a *AppContext) ClearPackageLogo(c *fiber.Ctx) error {
	id := c.Params("id")

	var current models.Package
	if err := a.DB.Where("id = ?", id).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Paket tidak ditemukan")
	}

	if err := a.DB.Model(&current).Updates(map[string]interface{}{
		"logo_url":   "",
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus logo paket", err.Error())
	}
	current.LogoURL = ""

	return utils.Success(c, 200, "Logo paket berhasil dihapus", fiber.Map{"package": current})
}

// GetPublicPackages returns active packages for the public sales landing page.
func (a *AppContext) GetPublicPackages(c *fiber.Ctx) error {
	var items []models.Package
	if err := a.DB.Where("is_active = ?", true).
		Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat daftar paket", err.Error())
	}
	items = sanitizePackageItems(items)
	return utils.Success(c, 200, "Daftar paket publik", fiber.Map{"packages": items})
}

// GetPackages returns all packages for the Super Admin CMS.
func (a *AppContext) GetPackages(c *fiber.Ctx) error {
	var items []models.Package
	if err := a.DB.Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat daftar paket", err.Error())
	}
	items = sanitizePackageItems(items)
	return utils.Success(c, 200, "Daftar paket", fiber.Map{"packages": items})
}

func (a *AppContext) CreatePackage(c *fiber.Ctx) error {
	var body packagePayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	item := models.Package{IsActive: true}
	if body.IsActive != nil {
		item.IsActive = *body.IsActive
	}
	applyPackagePayload(&item, body)

	if item.Name == "" {
		return utils.Error(c, 422, "Nama paket wajib diisi")
	}

	if err := a.DB.Create(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat paket", err.Error())
	}
	item = sanitizePackageItem(item)
	return utils.Success(c, 201, "Paket berhasil dibuat", fiber.Map{"package": item})
}

func (a *AppContext) UpdatePackage(c *fiber.Ctx) error {
	id := c.Params("id")

	var current models.Package
	if err := a.DB.Where("id = ?", id).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Paket tidak ditemukan")
	}

	var body packagePayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	current.IsActive = true
	if body.IsActive != nil {
		current.IsActive = *body.IsActive
	}
	applyPackagePayload(&current, body)

	if current.Name == "" {
		return utils.Error(c, 422, "Nama paket wajib diisi")
	}

	if err := a.DB.Model(&current).Select(
		"name", "tagline", "price", "original_price", "billing_period",
		"badge_text", "is_popular", "is_active", "sort_order", "cta_label",
		"cta_url", "modules", "updated_at",
	).Updates(map[string]interface{}{
		"name":           current.Name,
		"tagline":        current.Tagline,
		"price":          current.Price,
		"original_price": current.OriginalPrice,
		"billing_period": current.BillingPeriod,
		"badge_text":     current.BadgeText,
		"is_popular":     current.IsPopular,
		"is_active":      current.IsActive,
		"sort_order":     current.SortOrder,
		"cta_label":      current.CTALabel,
		"cta_url":        current.CTAURL,
		"modules":        current.Modules,
		"updated_at":     gorm.Expr("NOW()"),
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui paket", err.Error())
	}

	current = sanitizePackageItem(current)
	return utils.Success(c, 200, "Paket berhasil diperbarui", fiber.Map{"package": current})
}

func (a *AppContext) DeletePackage(c *fiber.Ctx) error {
	id := c.Params("id")

	var current models.Package
	if err := a.DB.Where("id = ?", id).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Paket tidak ditemukan")
	}

	var orderCount int64
	if err := a.DB.Model(&models.PackageCheckoutOrder{}).Where("package_id = ?", current.ID).Count(&orderCount).Error; err != nil {
		return utils.Error(c, 500, "Gagal memeriksa riwayat checkout paket", err.Error())
	}

	if orderCount > 0 {
		if err := a.DB.Model(&current).Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": gorm.Expr("NOW()"),
		}).Error; err != nil {
			return utils.Error(c, 500, "Gagal mengarsipkan paket", err.Error())
		}
		current.IsActive = false
		return utils.Success(c, 200, "Paket memiliki riwayat checkout, jadi paket diarsipkan agar transaksi lama tetap aman.", fiber.Map{
			"id":        current.ID,
			"archived":  true,
			"package":   current,
			"order_cnt": orderCount,
		})
	}

	if err := a.DB.Delete(&current).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus paket", err.Error())
	}
	return utils.Success(c, 200, "Paket berhasil dihapus", fiber.Map{"id": current.ID, "archived": false})
}
