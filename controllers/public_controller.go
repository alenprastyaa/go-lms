package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) GetPublicRegistrationOptions(c *fiber.Ctx) error {
	token := c.Query("token")
	schoolID, err := utils.ParseSchoolRegistrationToken(token)
	if err != nil {
		return utils.Error(c, 401, "Link pendaftaran tidak valid atau sudah kadaluarsa")
	}

	var school models.School
	if err := a.DB.Where("id = ?", schoolID).First(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}

	var classes []models.Class
	a.DB.Where("school_id = ?", schoolID).Order("class_name asc").Find(&classes)
	return utils.Success(c, 200, "Success Get Registration Options", fiber.Map{
		"school": fiber.Map{
			"id":   school.ID,
			"name": school.Name,
		},
		"classes": classes,
	})
}

func (a *AppContext) RegisterStudentPublic(c *fiber.Ctx) error {
	var body struct {
		Token   string `json:"token"`
		ClassID uint   `json:"class_id"`
	}
	_ = c.BodyParser(&body)

	schoolID, err := utils.ParseSchoolRegistrationToken(body.Token)
	if err != nil {
		return utils.Error(c, 401, "Link pendaftaran tidak valid atau sudah kadaluarsa")
	}

	var classItem models.Class
	if err := a.DB.Where("id = ? AND school_id = ?", body.ClassID, schoolID).First(&classItem).Error; err != nil {
		return utils.Error(c, 404, "Class not found")
	}

	c.Locals("schoolID", schoolID)
	return a.registerScopedUser(c, true)
}

func (a *AppContext) SearchPublicLocations(c *fiber.Ctx) error {
	query := strings.TrimSpace(c.Query("q"))
	if len(query) < 3 {
		return utils.Success(c, 200, "Success Search Locations", fiber.Map{"items": []interface{}{}})
	}

	endpoint, err := url.Parse("https://nominatim.openstreetmap.org/search")
	if err != nil {
		return utils.Error(c, 500, "Gagal menyiapkan pencarian lokasi")
	}
	values := endpoint.Query()
	values.Set("format", "jsonv2")
	values.Set("addressdetails", "1")
	values.Set("limit", "6")
	values.Set("accept-language", "id")
	values.Set("q", query)
	endpoint.RawQuery = values.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return utils.Error(c, 500, "Gagal menyiapkan pencarian lokasi")
	}
	request.Header.Set("User-Agent", "SchoolSystem/1.0")
	request.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return utils.Error(c, 502, "Gagal mencari lokasi", err.Error())
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return utils.Error(c, 502, "Pencarian lokasi gagal", fmt.Sprintf("status %d", response.StatusCode))
	}

	var rawItems []struct {
		PlaceID    interface{} `json:"place_id"`
		DisplayName string      `json:"display_name"`
		Name       string      `json:"name"`
		Lat        string      `json:"lat"`
		Lon        string      `json:"lon"`
	}
	if err := json.NewDecoder(response.Body).Decode(&rawItems); err != nil {
		return utils.Error(c, 500, "Gagal membaca hasil lokasi", err.Error())
	}

	items := make([]map[string]interface{}, 0, len(rawItems))
	for _, item := range rawItems {
		items = append(items, map[string]interface{}{
			"place_id":     item.PlaceID,
			"display_name": item.DisplayName,
			"name":         item.Name,
			"lat":          item.Lat,
			"lon":          item.Lon,
		})
	}

	return utils.Success(c, 200, "Success Search Locations", fiber.Map{
		"items": items,
	})
}
