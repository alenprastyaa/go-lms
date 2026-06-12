package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/models"
	"lms/utils"
)

type schoolVisitTargetPayload struct {
	Name          *string  `json:"name"`
	Email         *string  `json:"email"`
	Wakur         *string  `json:"wakur"`
	Kepsek        *string  `json:"kepsek"`
	FullAddress   *string  `json:"full_address"`
	Province      *string  `json:"province"`
	City          *string  `json:"city"`
	District      *string  `json:"district"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
	GoogleMapsURL *string  `json:"google_maps_url"`
	PlaceID       *string  `json:"place_id"`
	IsPlanned     *bool    `json:"is_planned"`
	IsVisited     *bool    `json:"is_visited"`
}

type resolvedSchoolAddress struct {
	FullAddress string
	Province    string
	City        string
	District    string
	Latitude    *float64
	Longitude   *float64
	PlaceID     string
	MapsURL     string
}

func (a *AppContext) GetSchoolVisitTargets(c *fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("search"))
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", "10"), 10)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	query := a.DB.Model(&models.SchoolVisitTarget{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(`
				name ILIKE ?
				OR COALESCE(email, '') ILIKE ?
				OR COALESCE(wakur, '') ILIKE ?
			OR COALESCE(kepsek, '') ILIKE ?
			OR COALESCE(full_address, '') ILIKE ?
			OR COALESCE(province, '') ILIKE ?
			OR COALESCE(city, '') ILIKE ?
			OR COALESCE(district, '') ILIKE ?
			`, like, like, like, like, like, like, like, like)
	}
	if rawVisited := strings.TrimSpace(c.Query("is_visited")); rawVisited != "" {
		if visited, err := strconv.ParseBool(rawVisited); err == nil {
			query = query.Where("is_visited = ?", visited)
		}
	}
	if rawPlanned := strings.TrimSpace(c.Query("is_planned")); rawPlanned != "" {
		if planned, err := strconv.ParseBool(rawPlanned); err == nil {
			query = query.Where("is_planned = ?", planned)
		}
	}
	switch strings.ToLower(strings.TrimSpace(c.Query("visit_status"))) {
	case "complete":
		query = query.Where("is_visited = TRUE")
	case "planned":
		query = query.Where("is_planned = TRUE AND is_visited = FALSE")
	case "unplanned":
		query = query.Where("is_planned = FALSE AND is_visited = FALSE")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghitung list sekolah", err.Error())
	}

	var items []models.SchoolVisitTarget
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat list sekolah", err.Error())
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	return utils.Success(c, 200, "Success Get List Sekolah", fiber.Map{
		"items":      items,
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (a *AppContext) CreateSchoolVisitTarget(c *fiber.Ctx) error {
	var body schoolVisitTargetPayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	name := ""
	if body.Name != nil {
		name = strings.TrimSpace(*body.Name)
	}
	if name == "" {
		return utils.Error(c, 400, "Nama sekolah wajib diisi")
	}

	userID, _ := c.Locals("userID").(uint)
	now := time.Now()
	item := models.SchoolVisitTarget{
		Name:          name,
		Email:         trimPtr(body.Email),
		Wakur:         trimPtr(body.Wakur),
		Kepsek:        trimPtr(body.Kepsek),
		FullAddress:   trimPtr(body.FullAddress),
		Province:      trimPtr(body.Province),
		City:          trimPtr(body.City),
		District:      trimPtr(body.District),
		Latitude:      body.Latitude,
		Longitude:     body.Longitude,
		GoogleMapsURL: trimPtr(body.GoogleMapsURL),
		PlaceID:       trimPtr(body.PlaceID),
		IsPlanned:     false,
		PlannedAt:     nil,
		CreatedBy:     userIDPtr(userID),
		UpdatedBy:     userIDPtr(userID),
	}
	if body.IsPlanned != nil {
		item.IsPlanned = *body.IsPlanned
		if item.IsPlanned {
			item.PlannedAt = &now
		} else {
			item.PlannedAt = nil
		}
	}
	if body.IsVisited != nil {
		item.IsVisited = *body.IsVisited
		if item.IsVisited {
			item.VisitedAt = &now
			if !item.IsPlanned {
				item.IsPlanned = true
				item.PlannedAt = &now
			}
		}
	}
	if item.GoogleMapsURL == nil {
		item.GoogleMapsURL = buildGoogleMapsURLPtr(item.Name, item.Latitude, item.Longitude)
	}

	if err := a.DB.Create(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat list sekolah", err.Error())
	}

	return utils.Success(c, 201, "List sekolah berhasil dibuat", item)
}

func (a *AppContext) UpdateSchoolVisitTarget(c *fiber.Ctx) error {
	id := c.Params("id")

	var current models.SchoolVisitTarget
	if err := a.DB.Where("id = ?", id).First(&current).Error; err != nil {
		return utils.Error(c, 404, "List sekolah tidak ditemukan")
	}

	var body schoolVisitTargetPayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	updates := map[string]interface{}{}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return utils.Error(c, 400, "Nama sekolah wajib diisi")
		}
		updates["name"] = name
		current.Name = name
	}
	if body.FullAddress != nil {
		updates["full_address"] = nullableTrim(*body.FullAddress)
	}
	if body.Email != nil {
		updates["email"] = nullableTrim(*body.Email)
	}
	if body.Wakur != nil {
		updates["wakur"] = nullableTrim(*body.Wakur)
	}
	if body.Kepsek != nil {
		updates["kepsek"] = nullableTrim(*body.Kepsek)
	}
	if body.Province != nil {
		updates["province"] = nullableTrim(*body.Province)
	}
	if body.City != nil {
		updates["city"] = nullableTrim(*body.City)
	}
	if body.District != nil {
		updates["district"] = nullableTrim(*body.District)
	}
	if body.Latitude != nil {
		updates["latitude"] = *body.Latitude
		current.Latitude = body.Latitude
	}
	if body.Longitude != nil {
		updates["longitude"] = *body.Longitude
		current.Longitude = body.Longitude
	}
	if body.PlaceID != nil {
		updates["place_id"] = nullableTrim(*body.PlaceID)
	}
	if body.GoogleMapsURL != nil {
		updates["google_maps_url"] = nullableTrim(*body.GoogleMapsURL)
	} else if body.Name != nil || body.Latitude != nil || body.Longitude != nil {
		updates["google_maps_url"] = buildGoogleMapsURL(current.Name, current.Latitude, current.Longitude)
	}
	if body.IsPlanned != nil {
		updates["is_planned"] = *body.IsPlanned
		if *body.IsPlanned {
			updates["planned_at"] = time.Now()
		} else {
			updates["planned_at"] = nil
			updates["is_visited"] = false
			updates["visited_at"] = nil
		}
	}
	if body.IsVisited != nil {
		updates["is_visited"] = *body.IsVisited
		if *body.IsVisited {
			now := time.Now()
			updates["visited_at"] = now
			updates["is_planned"] = true
			updates["planned_at"] = now
		} else {
			updates["visited_at"] = nil
		}
	}

	userID, _ := c.Locals("userID").(uint)
	if userID > 0 {
		updates["updated_by"] = userID
	}
	if len(updates) == 0 {
		return utils.Error(c, 400, "Tidak ada perubahan list sekolah")
	}

	if err := a.DB.Model(&current).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui list sekolah", err.Error())
	}

	var item models.SchoolVisitTarget
	if err := a.DB.Where("id = ?", current.ID).First(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat list sekolah", err.Error())
	}
	return utils.Success(c, 200, "List sekolah berhasil diperbarui", item)
}

func (a *AppContext) DeleteSchoolVisitTarget(c *fiber.Ctx) error {
	id := c.Params("id")

	var item models.SchoolVisitTarget
	if err := a.DB.Where("id = ?", id).First(&item).Error; err != nil {
		return utils.Error(c, 404, "List sekolah tidak ditemukan")
	}
	if err := a.DB.Delete(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus list sekolah", err.Error())
	}

	return utils.Success(c, 200, "List sekolah berhasil dihapus", fiber.Map{
		"id":   item.ID,
		"name": item.Name,
	})
}

func (a *AppContext) ResolveSchoolVisitTargetAddress(c *fiber.Ctx) error {
	id := c.Params("id")

	var current models.SchoolVisitTarget
	if err := a.DB.Where("id = ?", id).First(&current).Error; err != nil {
		return utils.Error(c, 404, "List sekolah tidak ditemukan")
	}

	var body struct {
		Query string `json:"query"`
	}
	_ = c.BodyParser(&body)
	query := strings.TrimSpace(body.Query)
	if query == "" {
		query = current.Name
	}
	if query == "" {
		return utils.Error(c, 400, "Nama sekolah wajib diisi untuk mencari alamat")
	}

	resolved, err := resolveSchoolAddress(query)
	if err != nil {
		return utils.Error(c, 502, "Gagal mencari alamat sekolah", err.Error())
	}
	if resolved.FullAddress == "" && resolved.Province == "" && resolved.City == "" && resolved.District == "" {
		return utils.Error(c, 404, "Alamat sekolah tidak ditemukan")
	}

	updates := map[string]interface{}{
		"full_address":    nullableTrim(resolved.FullAddress),
		"province":        nullableTrim(resolved.Province),
		"city":            nullableTrim(resolved.City),
		"district":        nullableTrim(resolved.District),
		"google_maps_url": buildResolvedMapsURL(query, resolved),
	}
	if resolved.Latitude != nil {
		updates["latitude"] = *resolved.Latitude
	}
	if resolved.Longitude != nil {
		updates["longitude"] = *resolved.Longitude
	}
	if resolved.PlaceID != "" {
		updates["place_id"] = resolved.PlaceID
	}
	if userID, _ := c.Locals("userID").(uint); userID > 0 {
		updates["updated_by"] = userID
	}

	if err := a.DB.Model(&current).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan alamat sekolah", err.Error())
	}

	var item models.SchoolVisitTarget
	if err := a.DB.Where("id = ?", current.ID).First(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat list sekolah", err.Error())
	}
	return utils.Success(c, 200, "Alamat sekolah berhasil diisi otomatis", item)
}

func trimPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableTrim(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func userIDPtr(userID uint) *uint {
	if userID == 0 {
		return nil
	}
	return &userID
}

func buildGoogleMapsURLPtr(name string, lat *float64, lng *float64) *string {
	value := buildGoogleMapsURL(name, lat, lng)
	return &value
}

func buildGoogleMapsURL(name string, lat *float64, lng *float64) string {
	query := strings.TrimSpace(name)
	if lat != nil && lng != nil {
		query = strings.TrimSpace(formatCoordinate(*lat, *lng))
	}
	if query == "" {
		query = strings.TrimSpace(name)
	}
	return "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(query)
}

func formatCoordinate(lat float64, lng float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(lat, 'f', 6, 64), "0"), ".") + "," + strings.TrimRight(strings.TrimRight(strconv.FormatFloat(lng, 'f', 6, 64), "0"), ".")
}

func resolveSchoolAddress(query string) (resolvedSchoolAddress, error) {
	if apiKey := strings.TrimSpace(os.Getenv("GOOGLE_MAPS_API_KEY")); apiKey != "" {
		return resolveSchoolAddressWithGoogle(query, apiKey)
	}
	return resolveSchoolAddressWithNominatim(query)
}

func resolveSchoolAddressWithGoogle(query string, apiKey string) (resolvedSchoolAddress, error) {
	endpoint, err := url.Parse("https://maps.googleapis.com/maps/api/geocode/json")
	if err != nil {
		return resolvedSchoolAddress{}, err
	}
	values := endpoint.Query()
	values.Set("address", query)
	values.Set("region", "id")
	values.Set("language", "id")
	values.Set("key", apiKey)
	endpoint.RawQuery = values.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return resolvedSchoolAddress{}, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return resolvedSchoolAddress{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return resolvedSchoolAddress{}, fmt.Errorf("Google Geocoding status %d", response.StatusCode)
	}

	var payload struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		Results      []struct {
			FormattedAddress string `json:"formatted_address"`
			PlaceID          string `json:"place_id"`
			Geometry         struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
			AddressComponents []struct {
				LongName string   `json:"long_name"`
				Types    []string `json:"types"`
			} `json:"address_components"`
		} `json:"results"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return resolvedSchoolAddress{}, err
	}
	if payload.Status != "OK" || len(payload.Results) == 0 {
		if payload.ErrorMessage != "" {
			return resolvedSchoolAddress{}, errors.New(payload.ErrorMessage)
		}
		return resolvedSchoolAddress{}, fmt.Errorf("alamat tidak ditemukan")
	}

	result := payload.Results[0]
	lat := result.Geometry.Location.Lat
	lng := result.Geometry.Location.Lng
	resolved := resolvedSchoolAddress{
		FullAddress: result.FormattedAddress,
		Latitude:    &lat,
		Longitude:   &lng,
		PlaceID:     result.PlaceID,
	}
	for _, component := range result.AddressComponents {
		if hasAddressType(component.Types, "administrative_area_level_1") {
			resolved.Province = component.LongName
		}
		if hasAddressType(component.Types, "administrative_area_level_2") {
			resolved.City = component.LongName
		}
		if resolved.District == "" && (hasAddressType(component.Types, "administrative_area_level_3") || hasAddressType(component.Types, "sublocality_level_1")) {
			resolved.District = component.LongName
		}
	}
	return resolved, nil
}

func resolveSchoolAddressWithNominatim(query string) (resolvedSchoolAddress, error) {
	endpoint, err := url.Parse("https://nominatim.openstreetmap.org/search")
	if err != nil {
		return resolvedSchoolAddress{}, err
	}
	values := endpoint.Query()
	values.Set("format", "jsonv2")
	values.Set("addressdetails", "1")
	values.Set("limit", "1")
	values.Set("accept-language", "id")
	values.Set("q", query)
	endpoint.RawQuery = values.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return resolvedSchoolAddress{}, err
	}
	request.Header.Set("User-Agent", "SchoolSystem/1.0")
	request.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return resolvedSchoolAddress{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return resolvedSchoolAddress{}, fmt.Errorf("Nominatim status %d", response.StatusCode)
	}

	var payload []struct {
		PlaceID     interface{}       `json:"place_id"`
		DisplayName string            `json:"display_name"`
		Lat         string            `json:"lat"`
		Lon         string            `json:"lon"`
		Address     map[string]string `json:"address"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return resolvedSchoolAddress{}, err
	}
	if len(payload) == 0 {
		return resolvedSchoolAddress{}, fmt.Errorf("alamat tidak ditemukan")
	}

	item := payload[0]
	lat, latErr := strconv.ParseFloat(item.Lat, 64)
	lng, lngErr := strconv.ParseFloat(item.Lon, 64)
	resolved := resolvedSchoolAddress{
		FullAddress: item.DisplayName,
		Province:    firstNonEmpty(item.Address["state"], item.Address["province"]),
		City:        firstNonEmpty(item.Address["city"], item.Address["town"], item.Address["county"], item.Address["municipality"]),
		District:    firstNonEmpty(item.Address["city_district"], item.Address["district"], item.Address["suburb"], item.Address["village"]),
		PlaceID:     fmt.Sprint(item.PlaceID),
	}
	if latErr == nil {
		resolved.Latitude = &lat
	}
	if lngErr == nil {
		resolved.Longitude = &lng
	}
	return resolved, nil
}

func hasAddressType(types []string, expected string) bool {
	for _, item := range types {
		if item == expected {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildResolvedMapsURL(query string, resolved resolvedSchoolAddress) string {
	if resolved.MapsURL != "" {
		return resolved.MapsURL
	}
	if resolved.PlaceID != "" && strings.TrimSpace(os.Getenv("GOOGLE_MAPS_API_KEY")) != "" {
		return "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(query) + "&query_place_id=" + url.QueryEscape(resolved.PlaceID)
	}
	return buildGoogleMapsURL(query, resolved.Latitude, resolved.Longitude)
}
