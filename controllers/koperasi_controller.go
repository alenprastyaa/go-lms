package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"lms/models"
	"lms/utils"
)

const (
	koperasiStatusPending    = "PENDING"
	koperasiStatusProcessing = "PROCESSING"
	koperasiStatusReady      = "READY"
	koperasiStatusCompleted  = "COMPLETED"
	koperasiStatusCanceled   = "CANCELED"

	koperasiPaymentMethodCash     = "TUNAI"
	koperasiPaymentMethodNonCash  = "NON_TUNAI"
	koperasiPaymentProviderCash   = "CASH"
	koperasiPaymentProviderXendit = "XENDIT"
	koperasiPaymentStatusCashDue  = "CASH_DUE"
	koperasiPaymentStatusPending  = "PENDING"
	koperasiPaymentStatusPaid     = "PAID"
	koperasiPaymentStatusFailed   = "FAILED"
	koperasiPaymentStatusExpired  = "EXPIRED"
)

func koperasiManageRoles() []string {
	return []string{"ADMIN", "KOPERASI"}
}

func koperasiCanManage(role string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(role))
	return normalized == "ADMIN" || normalized == "KOPERASI"
}

func koperasiRoleOrEmpty(value interface{}) string {
	return strings.ToUpper(strings.TrimSpace(fmt.Sprint(value)))
}

func koperasiNormalizeStatus(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func koperasiAllowedStatus(value string) bool {
	switch koperasiNormalizeStatus(value) {
	case koperasiStatusPending, koperasiStatusProcessing, koperasiStatusReady, koperasiStatusCompleted, koperasiStatusCanceled:
		return true
	default:
		return false
	}
}

func koperasiOrderNumber() string {
	now := time.Now().UTC()
	return fmt.Sprintf("KOP-%s-%d", now.Format("20060102"), now.UnixNano())
}

func koperasiNormalizePaymentMethod(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case "", "TUNAI", "CASH":
		return koperasiPaymentMethodCash, nil
	case "NON_TUNAI", "NON-TUNAI", "NON TUNAI", "TRANSFER", "QRIS":
		return koperasiPaymentMethodNonCash, nil
	default:
		return "", fmt.Errorf("Metode pembayaran tidak valid")
	}
}

func koperasiNormalizePaymentMethodValue(value string) string {
	normalized, err := koperasiNormalizePaymentMethod(value)
	if err != nil {
		return strings.ToUpper(strings.TrimSpace(value))
	}
	return normalized
}

func koperasiStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func koperasiPaymentStatusForMethod(method string) string {
	if method == koperasiPaymentMethodNonCash {
		return koperasiPaymentStatusPending
	}
	return koperasiPaymentStatusCashDue
}

func koperasiPaymentExpiryMinutes() int {
	value := strings.TrimSpace(os.Getenv("KOPERASI_QRIS_EXPIRE_AFTER_MINUTES"))
	if value == "" {
		return 15
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 15
	}
	return parsed
}

func koperasiPaymentExpiresAt() *time.Time {
	minutes := koperasiPaymentExpiryMinutes()
	if minutes <= 0 {
		return nil
	}
	expiresAt := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	return &expiresAt
}

func koperasiPaymentStatusLabel(status, method string) string {
	normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	switch normalizedStatus {
	case koperasiPaymentStatusPaid:
		return "Lunas"
	case koperasiPaymentStatusCashDue:
		return "Tunai saat terima"
	case koperasiPaymentStatusPending:
		if normalizedMethod == koperasiPaymentMethodNonCash {
			return "Menunggu QRIS"
		}
		return "Menunggu pembayaran"
	case koperasiPaymentStatusFailed:
		return "Gagal"
	case koperasiPaymentStatusExpired:
		return "Kedaluwarsa"
	default:
		if normalizedStatus == "" {
			return "-"
		}
		return strings.Title(strings.ToLower(normalizedStatus))
	}
}

func koperasiPaymentStatusBadge(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case koperasiPaymentStatusPaid:
		return "PAID"
	case koperasiPaymentStatusPending:
		return "PENDING"
	case koperasiPaymentStatusCashDue:
		return "CASH_DUE"
	case koperasiPaymentStatusExpired:
		return "EXPIRED"
	case koperasiPaymentStatusFailed:
		return "FAILED"
	default:
		return strings.ToUpper(strings.TrimSpace(status))
	}
}

func koperasiPaymentLogMetadata(order models.KoperasiOrder) string {
	payload := map[string]interface{}{
		"order_number":     order.OrderNumber,
		"payment_method":   koperasiStringValue(order.PaymentMethod),
		"payment_provider": koperasiStringValue(order.PaymentProvider),
		"payment_status":   order.PaymentStatus,
	}
	if order.PaymentExpiresAt != nil {
		payload["payment_expires_at"] = order.PaymentExpiresAt.UTC().Format(time.RFC3339)
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func (a *AppContext) createKoperasiPaymentLog(tx *gorm.DB, order models.KoperasiOrder, eventType, status string, paymentRequestID *string, note string) error {
	if tx == nil {
		tx = a.DB
	}

	var notePtr *string
	if trimmed := strings.TrimSpace(note); trimmed != "" {
		notePtr = &trimmed
	}
	metadata := koperasiPaymentLogMetadata(order)
	var paymentRequest *string
	if paymentRequestID != nil {
		value := strings.TrimSpace(*paymentRequestID)
		if value != "" {
			paymentRequest = &value
		}
	}

	log := models.KoperasiPaymentLog{
		SchoolID:         order.SchoolID,
		OrderID:          order.ID,
		EventType:        strings.ToUpper(strings.TrimSpace(eventType)),
		Status:           strings.ToUpper(strings.TrimSpace(status)),
		PaymentRequestID: paymentRequest,
		Note:             notePtr,
		Metadata:         &metadata,
	}
	return tx.Create(&log).Error
}

func (a *AppContext) loadKoperasiRealtimeOrderPayload(orderID uint) (map[string]interface{}, error) {
	var order map[string]interface{}
	if err := a.DB.Table("koperasi_orders ko").
		Select(`
			ko.id,
			ko.school_id,
			ko.order_number,
			ko.buyer_id,
			ko.buyer_role,
			ko.status,
			ko.payment_method,
			ko.payment_provider,
			ko.payment_status,
			ko.payment_request_id,
			ko.payment_qr_string,
			ko.payment_expires_at,
			ko.paid_at,
			ko.note,
			ko.total_amount,
			ko.handled_by,
			ko.created_at,
			ko.updated_at,
			COALESCE(NULLIF(b.full_name, ''), '-') AS buyer_name,
			COALESCE(cl.class_name, '-') AS buyer_class_name,
			COALESCE(h.full_name, h.username) AS handled_by_name
		`).
		Joins("INNER JOIN users b ON b.id = ko.buyer_id").
		Joins("LEFT JOIN student_class_enrollments sce ON sce.student_id = b.id AND sce.school_id = ko.school_id AND sce.is_active = true").
		Joins("LEFT JOIN class cl ON cl.id = COALESCE(b.class_id, sce.class_id)").
		Joins("LEFT JOIN users h ON h.id = ko.handled_by").
		Where("ko.id = ?", orderID).
		Scan(&order).Error; err != nil {
		return nil, err
	}

	normalizeKoperasiOrderMap(order)
	koperasiOrderResponseAugment(order)
	return order, nil
}

func (a *AppContext) broadcastKoperasiOrderEvent(eventName string, orderID uint, orderAction string, schoolID uint) {
	if a == nil || a.Realtime == nil || orderID == 0 {
		return
	}

	payload, err := a.loadKoperasiRealtimeOrderPayload(orderID)
	if err != nil {
		return
	}

	payload["event"] = eventName
	payload["action"] = strings.ToUpper(strings.TrimSpace(orderAction))
	payload["channel"] = "koperasi"
	if schoolID != 0 {
		payload["school_id"] = schoolID
	}

	a.Realtime.BroadcastSchoolRoleEvent(eventName, schoolID, []string{"ADMIN", "KOPERASI"}, payload)
}

func koperasiXenditSandboxEnabled() bool {
	value := strings.ToUpper(strings.TrimSpace(os.Getenv("XENDIT_SANDBOX_MODE")))
	if value == "" {
		value = strings.ToUpper(strings.TrimSpace(os.Getenv("XENDIT_ENV")))
	}
	switch value {
	case "1", "TRUE", "YES", "ON", "SANDBOX":
		return true
	default:
		return false
	}
}

func koperasiSandboxQrisString(referenceID string, amount int64, schoolID uint) string {
	return fmt.Sprintf("KOPERASI-SANDBOX|%s|%d|%d|%s", referenceID, amount, schoolID, time.Now().UTC().Format("20060102150405"))
}

type koperasiProductInput struct {
	Name        string
	Code        string
	Category    string
	Description string
	ImageURL    string
	Price       int64
	Stock       int
	IsActive    *bool
}

type xenditKoperasiPaymentRequest struct {
	ReferenceID   string            `json:"reference_id"`
	Type          string            `json:"type"`
	Country       string            `json:"country"`
	Currency      string            `json:"currency"`
	RequestAmount int64             `json:"request_amount"`
	CaptureMethod string            `json:"capture_method"`
	ChannelCode   string            `json:"channel_code"`
	Description   string            `json:"description,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type xenditKoperasiPaymentResponse struct {
	PaymentRequestID string `json:"payment_request_id"`
	ReferenceID      string `json:"reference_id"`
	Status           string `json:"status"`
	Actions          []struct {
		Type       string `json:"type"`
		Descriptor string `json:"descriptor"`
		Value      string `json:"value"`
	} `json:"actions"`
	Message string `json:"message"`
}

type xenditKoperasiPaymentWebhook struct {
	PaymentCapture *struct {
		Value xenditKoperasiPaymentWebhookValue `json:"value"`
	} `json:"paymentCapture,omitempty"`
	PaymentAuthorization *struct {
		Value xenditKoperasiPaymentWebhookValue `json:"value"`
	} `json:"paymentAuthorization,omitempty"`
	PaymentFailure *struct {
		Value xenditKoperasiPaymentWebhookValue `json:"value"`
	} `json:"paymentFailure,omitempty"`
	PaymentRequest *struct {
		Value xenditKoperasiPaymentWebhookValue `json:"value"`
	} `json:"paymentRequest,omitempty"`
}

type xenditKoperasiPaymentWebhookValue struct {
	Event string `json:"event"`
	Data  struct {
		ReferenceID      string `json:"reference_id"`
		PaymentRequestID string `json:"payment_request_id"`
		Status           string `json:"status"`
		ChannelCode      string `json:"channel_code"`
	} `json:"data"`
}

func parseKoperasiProductInput(c *fiber.Ctx) (*koperasiProductInput, error) {
	input := &koperasiProductInput{}
	contentType := strings.ToLower(strings.TrimSpace(c.Get("Content-Type")))

	if strings.Contains(contentType, "multipart/form-data") {
		input.Name = c.FormValue("name")
		input.Code = c.FormValue("code")
		input.Category = c.FormValue("category")
		input.Description = c.FormValue("description")
		input.ImageURL = c.FormValue("image_url")
		if rawPrice := strings.TrimSpace(c.FormValue("price")); rawPrice != "" {
			if parsed, parseErr := strconv.ParseInt(rawPrice, 10, 64); parseErr == nil {
				input.Price = parsed
			}
		}
		if rawStock := strings.TrimSpace(c.FormValue("stock")); rawStock != "" {
			if parsed, parseErr := strconv.Atoi(rawStock); parseErr == nil {
				input.Stock = parsed
			}
		}

		isActiveRaw := strings.TrimSpace(c.FormValue("is_active"))
		if isActiveRaw != "" {
			value := strings.EqualFold(isActiveRaw, "true") || isActiveRaw == "1" || strings.EqualFold(isActiveRaw, "on")
			input.IsActive = &value
		}
		return input, nil
	}

	var body struct {
		Name        string `json:"name"`
		Code        string `json:"code"`
		Category    string `json:"category"`
		Description string `json:"description"`
		ImageURL    string `json:"image_url"`
		Price       int64  `json:"price"`
		Stock       int    `json:"stock"`
		IsActive    *bool  `json:"is_active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return nil, err
	}

	input.Name = body.Name
	input.Code = body.Code
	input.Category = body.Category
	input.Description = body.Description
	input.ImageURL = body.ImageURL
	input.Price = body.Price
	input.Stock = body.Stock
	input.IsActive = body.IsActive
	return input, nil
}

func createXenditKoperasiQrisPayment(referenceID string, amount int64, schoolID uint) (*xenditKoperasiPaymentResponse, error) {
	if koperasiXenditSandboxEnabled() {
		qrString := koperasiSandboxQrisString(referenceID, amount, schoolID)
		return &xenditKoperasiPaymentResponse{
			PaymentRequestID: fmt.Sprintf("sandbox-%s", referenceID),
			ReferenceID:      referenceID,
			Status:           "REQUIRES_ACTION",
			Actions: []struct {
				Type       string `json:"type"`
				Descriptor string `json:"descriptor"`
				Value      string `json:"value"`
			}{
				{
					Type:       "PRESENT_TO_CUSTOMER",
					Descriptor: "QR_STRING",
					Value:      qrString,
				},
			},
		}, nil
	}

	apiKey := strings.TrimSpace(os.Getenv("XENDIT_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("XENDIT_API_KEY is not configured")
	}

	baseURL := strings.TrimSpace(os.Getenv("XENDIT_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.xendit.co"
	}

	payload := xenditKoperasiPaymentRequest{
		ReferenceID:   referenceID,
		Type:          "PAY",
		Country:       "ID",
		Currency:      "IDR",
		RequestAmount: amount,
		CaptureMethod: "AUTOMATIC",
		ChannelCode:   "QRIS",
		Description:   fmt.Sprintf("Pembayaran koperasi %s", referenceID),
		Metadata: map[string]string{
			"order_number": referenceID,
			"school_id":    fmt.Sprintf("%d", schoolID),
			"module":       "koperasi",
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", baseURL+"/v3/payment_requests", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-version", "2024-11-11")
	req.SetBasicAuth(apiKey, "")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed xenditKoperasiPaymentResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return nil, fmt.Errorf("xendit qris response parse failed: %w; body=%s", err, string(rawBody))
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(parsed.Message)
		if msg == "" {
			msg = string(rawBody)
		}
		return nil, fmt.Errorf("xendit qris error: %s", msg)
	}

	if parsed.PaymentRequestID == "" {
		return nil, fmt.Errorf("xendit qris response missing payment_request_id: %s", string(rawBody))
	}

	return &parsed, nil
}

func extractKoperasiQrisString(resp *xenditKoperasiPaymentResponse) string {
	if resp == nil {
		return ""
	}
	for _, action := range resp.Actions {
		if strings.EqualFold(strings.TrimSpace(action.Descriptor), "QR_STRING") {
			return strings.TrimSpace(action.Value)
		}
	}
	return ""
}

func normalizeKoperasiOrderMap(row map[string]interface{}) {
	if len(row) == 0 {
		return
	}

	for _, key := range []string{"created_at", "updated_at", "paid_at", "payment_expires_at"} {
		if value, ok := row[key]; ok {
			row[key] = normalizeKoperasiJakartaDateTimeValue(value)
		}
	}
}

func normalizeKoperasiOrderMaps(rows []map[string]interface{}) {
	for _, row := range rows {
		normalizeKoperasiOrderMap(row)
	}
}

func koperasiOrderResponseAugment(order map[string]interface{}) {
	if len(order) == 0 {
		return
	}
	order["payment_sandbox"] = koperasiXenditSandboxEnabled()
	if koperasiOrderMapHasExpired(order) {
		order["payment_is_expired"] = true
		order["payment_status_label"] = koperasiPaymentStatusLabel(koperasiPaymentStatusExpired, fmt.Sprint(order["payment_method"]))
		order["payment_status_badge"] = koperasiPaymentStatusBadge(koperasiPaymentStatusExpired)
		return
	}
	if status, ok := order["payment_status"]; ok {
		order["payment_status_label"] = koperasiPaymentStatusLabel(fmt.Sprint(status), fmt.Sprint(order["payment_method"]))
		order["payment_status_badge"] = koperasiPaymentStatusBadge(fmt.Sprint(status))
	}
}

func koperasiOrderMapHasExpired(order map[string]interface{}) bool {
	if len(order) == 0 {
		return false
	}
	status := strings.ToUpper(strings.TrimSpace(fmt.Sprint(order["payment_status"])))
	if status != koperasiPaymentStatusPending {
		return false
	}
	expiresRaw, ok := order["payment_expires_at"]
	if !ok {
		return false
	}
	expiresAt := koperasiParseTimestamp(fmt.Sprint(expiresRaw))
	return expiresAt != nil && time.Now().UTC().After(expiresAt.UTC())
}

func koperasiParseTimestamp(value string) *time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "-" {
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return &parsed
		}
	}
	return nil
}

func koperasiExtractPaymentReference(order models.KoperasiOrder) *string {
	if order.PaymentRequestID != nil && strings.TrimSpace(*order.PaymentRequestID) != "" {
		value := strings.TrimSpace(*order.PaymentRequestID)
		return &value
	}
	return nil
}

func koperasiPaymentEventTypeFromStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case koperasiPaymentStatusPaid:
		return "PAID"
	case koperasiPaymentStatusExpired:
		return "EXPIRED"
	case koperasiPaymentStatusFailed:
		return "FAILED"
	case koperasiPaymentStatusCashDue:
		return "CREATED"
	default:
		return "UPDATED"
	}
}

func koperasiPaymentReferenceMatches(current, incoming *string) bool {
	currentValue := strings.TrimSpace(koperasiStringValue(current))
	incomingValue := strings.TrimSpace(koperasiStringValue(incoming))
	if currentValue == "" || incomingValue == "" {
		return true
	}
	return currentValue == incomingValue
}

func (a *AppContext) GetKoperasiDashboard(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	if schoolID == 0 {
		return utils.Error(c, 400, "School ID wajib tersedia untuk dashboard koperasi")
	}

	type KV struct {
		Key   string `json:"key"`
		Value int64  `json:"value"`
	}

	var overviewRows []KV
	if err := a.DB.Raw(`
		SELECT 'products_total' AS key, COUNT(*)::bigint AS value FROM koperasi_products WHERE school_id = ?
		UNION ALL
		SELECT 'products_active' AS key, COUNT(*)::bigint AS value FROM koperasi_products WHERE school_id = ? AND is_active = true
		UNION ALL
		SELECT 'stock_total' AS key, COALESCE(SUM(stock), 0)::bigint AS value FROM koperasi_products WHERE school_id = ?
		UNION ALL
		SELECT 'low_stock_products' AS key, COUNT(*)::bigint AS value FROM koperasi_products WHERE school_id = ? AND is_active = true AND stock <= 5
		UNION ALL
		SELECT 'pending_orders' AS key, COUNT(*)::bigint AS value FROM koperasi_orders WHERE school_id = ? AND status = 'PENDING'
		UNION ALL
		SELECT 'processing_orders' AS key, COUNT(*)::bigint AS value FROM koperasi_orders WHERE school_id = ? AND status = 'PROCESSING'
		UNION ALL
		SELECT 'ready_orders' AS key, COUNT(*)::bigint AS value FROM koperasi_orders WHERE school_id = ? AND status = 'READY'
		UNION ALL
		SELECT 'completed_orders' AS key, COUNT(*)::bigint AS value FROM koperasi_orders WHERE school_id = ? AND status = 'COMPLETED'
		UNION ALL
		SELECT 'canceled_orders' AS key, COUNT(*)::bigint AS value FROM koperasi_orders WHERE school_id = ? AND status = 'CANCELED'
		UNION ALL
		SELECT 'revenue_total' AS key, COALESCE(SUM(total_amount), 0)::bigint AS value FROM koperasi_orders WHERE school_id = ? AND status = 'COMPLETED'
		UNION ALL
		SELECT 'orders_today' AS key, COUNT(*)::bigint AS value FROM koperasi_orders WHERE school_id = ? AND DATE(created_at) = CURRENT_DATE
	`, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID).Scan(&overviewRows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat dashboard koperasi", err.Error())
	}

	overview := map[string]int64{}
	for _, row := range overviewRows {
		overview[row.Key] = row.Value
	}

	var school map[string]interface{}
	if err := a.DB.Raw(`SELECT id, name FROM schools WHERE id = ?`, schoolID).Scan(&school).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat data sekolah", err.Error())
	}

	var lowStockProducts []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			id,
			name,
			COALESCE(code, '-') AS code,
			COALESCE(category, '-') AS category,
			price,
			stock,
			is_active
		FROM koperasi_products
		WHERE school_id = ?
		  AND is_active = true
		ORDER BY stock ASC, name ASC
		LIMIT 8
	`, schoolID).Scan(&lowStockProducts).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat produk koperasi", err.Error())
	}

	var recentOrders []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			ko.id,
			ko.order_number,
			ko.status,
			ko.total_amount,
			ko.created_at,
			ko.payment_method,
			ko.payment_provider,
			ko.payment_status,
			ko.payment_request_id,
			ko.payment_qr_string,
			ko.payment_expires_at,
			ko.paid_at,
			ko.note,
			COALESCE(NULLIF(b.full_name, ''), '-') AS buyer_name,
			COALESCE(cl.class_name, '-') AS buyer_class_name,
			b.role AS buyer_role,
			COALESCE(h.full_name, h.username) AS handled_by_name,
			(
				SELECT COUNT(*)::int
				FROM koperasi_order_items koi
				WHERE koi.order_id = ko.id
			) AS item_count
		FROM koperasi_orders ko
		INNER JOIN users b ON b.id = ko.buyer_id
		LEFT JOIN student_class_enrollments sce ON sce.student_id = b.id AND sce.school_id = ko.school_id AND sce.is_active = true
		LEFT JOIN class cl ON cl.id = COALESCE(b.class_id, sce.class_id)
		LEFT JOIN users h ON h.id = ko.handled_by
		WHERE ko.school_id = ?
		ORDER BY ko.created_at DESC, ko.id DESC
		LIMIT 10
	`, schoolID).Scan(&recentOrders).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pesanan terbaru", err.Error())
	}
	normalizeKoperasiOrderMaps(recentOrders)
	for _, order := range recentOrders {
		koperasiOrderResponseAugment(order)
	}

	announcements, err := a.fetchAnnouncementsForSchool(schoolID, "KOPERASI", false, 3)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat pengumuman dashboard", err.Error())
	}

	return utils.Success(c, 200, "Success Get Koperasi Dashboard", fiber.Map{
		"generatedAt":   jakartaNow().Format(time.RFC3339),
		"school":        school,
		"overview":      overview,
		"announcements": announcements,
		"lowStockItems": recentOrEmpty(lowStockProducts),
		"recentOrders":  recentOrEmpty(recentOrders),
	})
}

func (a *AppContext) GetKoperasiProducts(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	includeInactive := c.Query("include_inactive") == "1" && koperasiCanManage(role)
	search := strings.TrimSpace(c.Query("search"))
	category := strings.TrimSpace(c.Query("category"))
	activeFilter := strings.ToLower(strings.TrimSpace(c.Query("active")))
	page, limit := parseInventoryPagination(c)
	offset := (page - 1) * limit

	query := a.DB.Table("koperasi_products kp").
		Select(`
			kp.id,
			kp.school_id,
			kp.name,
			kp.code,
			kp.category,
			kp.description,
			kp.image_url,
			kp.price,
			kp.stock,
			kp.is_active,
			kp.created_by,
			kp.updated_by,
			kp.created_at,
			kp.updated_at
		`).
		Where("kp.school_id = ?", schoolID)
	if !includeInactive {
		query = query.Where("kp.is_active = true")
	}
	switch activeFilter {
	case "1", "true", "active":
		query = query.Where("kp.is_active = true")
	case "0", "false", "inactive":
		query = query.Where("kp.is_active = false")
	}
	if search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where(`
			LOWER(COALESCE(kp.name, '')) LIKE ? OR
			LOWER(COALESCE(kp.code, '')) LIKE ? OR
			LOWER(COALESCE(kp.category, '')) LIKE ? OR
			LOWER(COALESCE(kp.description, '')) LIKE ?
		`, pattern, pattern, pattern, pattern)
	}
	if category != "" {
		query = query.Where("LOWER(COALESCE(kp.category, '')) = ?", strings.ToLower(category))
	}

	var items []map[string]interface{}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat produk koperasi", err.Error())
	}
	if err := query.Order("kp.is_active DESC, kp.stock ASC, kp.name ASC").Limit(limit).Offset(offset).Scan(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat produk koperasi", err.Error())
	}

	return utils.Success(c, 200, "Success Get Koperasi Products", fiber.Map{
		"items":      recentOrEmpty(items),
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPagesFromCount(total, limit),
	})
}

func (a *AppContext) CreateKoperasiProduct(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	if !koperasiCanManage(role) {
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}

	body, err := parseKoperasiProductInput(c)
	if err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		return utils.Error(c, 400, "Nama produk wajib diisi")
	}
	if body.Price < 0 {
		return utils.Error(c, 400, "Harga tidak boleh negatif")
	}
	if body.Stock < 0 {
		return utils.Error(c, 400, "Stok tidak boleh negatif")
	}

	imageURL := strings.TrimSpace(body.ImageURL)
	if file, fileErr := c.FormFile("image"); fileErr == nil && file != nil {
		saved, saveErr := utils.SaveUploadedFile(c, file)
		if saveErr != nil {
			return utils.Error(c, 500, "Gagal upload gambar produk", saveErr.Error())
		}
		imageURL = saved
	}

	item := models.KoperasiProduct{
		SchoolID:    schoolID,
		Name:        name,
		Code:        stringPtrIfNotBlank(body.Code),
		Category:    stringPtrIfNotBlank(body.Category),
		Description: stringPtrIfNotBlank(body.Description),
		ImageURL:    stringPtrIfNotBlank(imageURL),
		Price:       body.Price,
		Stock:       body.Stock,
		IsActive:    body.IsActive == nil || *body.IsActive,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
	}

	if err := a.DB.Create(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat produk koperasi", err.Error())
	}

	return a.respondKoperasiProduct(c, item.ID, 201, "Produk koperasi berhasil dibuat")
}

func (a *AppContext) UpdateKoperasiProduct(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	if !koperasiCanManage(role) {
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}

	id := c.Params("id")
	var current models.KoperasiProduct
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Produk tidak ditemukan")
	}

	body, err := parseKoperasiProductInput(c)
	if err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	updates := map[string]interface{}{
		"updated_by": userID,
	}
	if strings.TrimSpace(body.Name) != "" {
		updates["name"] = strings.TrimSpace(body.Name)
	}
	if body.Code != "" || strings.Contains(strings.ToLower(strings.TrimSpace(c.Get("Content-Type"))), "multipart/form-data") {
		updates["code"] = valueIfNotBlank(body.Code)
	}
	if body.Category != "" || strings.Contains(strings.ToLower(strings.TrimSpace(c.Get("Content-Type"))), "multipart/form-data") {
		updates["category"] = valueIfNotBlank(body.Category)
	}
	if body.Description != "" || strings.Contains(strings.ToLower(strings.TrimSpace(c.Get("Content-Type"))), "multipart/form-data") {
		updates["description"] = valueIfNotBlank(body.Description)
	}
	if body.ImageURL != "" || strings.Contains(strings.ToLower(strings.TrimSpace(c.Get("Content-Type"))), "multipart/form-data") {
		updates["image_url"] = valueIfNotBlank(body.ImageURL)
	}
	if body.Price < 0 {
		return utils.Error(c, 400, "Harga tidak boleh negatif")
	}
	if body.Price != 0 || strings.Contains(strings.ToLower(strings.TrimSpace(c.Get("Content-Type"))), "multipart/form-data") {
		updates["price"] = body.Price
	}
	if body.Stock < 0 {
		return utils.Error(c, 400, "Stok tidak boleh negatif")
	}
	if body.Stock != 0 || strings.Contains(strings.ToLower(strings.TrimSpace(c.Get("Content-Type"))), "multipart/form-data") {
		updates["stock"] = body.Stock
	}
	if body.IsActive != nil {
		updates["is_active"] = *body.IsActive
	}

	if file, fileErr := c.FormFile("image"); fileErr == nil && file != nil {
		saved, saveErr := utils.SaveUploadedFile(c, file)
		if saveErr != nil {
			return utils.Error(c, 500, "Gagal upload gambar produk", saveErr.Error())
		}
		updates["image_url"] = saved
	}

	if len(updates) == 1 {
		return utils.Error(c, 400, "Tidak ada perubahan produk")
	}

	if err := a.DB.Model(&models.KoperasiProduct{}).Where("id = ? AND school_id = ?", current.ID, schoolID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui produk koperasi", err.Error())
	}

	return a.respondKoperasiProduct(c, current.ID, 200, "Produk koperasi berhasil diperbarui")
}

func (a *AppContext) DeleteKoperasiProduct(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	if !koperasiCanManage(role) {
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}

	id := c.Params("id")
	var current models.KoperasiProduct
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Produk tidak ditemukan")
	}

	if err := a.DB.Model(&models.KoperasiProduct{}).Where("id = ? AND school_id = ?", current.ID, schoolID).Updates(map[string]interface{}{
		"is_active":  false,
		"updated_by": userID,
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menonaktifkan produk koperasi", err.Error())
	}

	return utils.Success(c, 200, "Produk koperasi berhasil dinonaktifkan", fiber.Map{
		"id": current.ID,
	})
}

func (a *AppContext) GetKoperasiCart(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	buyerID := c.Locals("userID").(uint)
	return a.respondKoperasiCart(c, schoolID, buyerID, 200, "Success Get Koperasi Cart")
}

func (a *AppContext) UpsertKoperasiCartItem(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	buyerID := c.Locals("userID").(uint)

	var body struct {
		ProductID uint `json:"product_id"`
		Quantity  int  `json:"quantity"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}
	if body.ProductID == 0 {
		return utils.Error(c, 400, "Produk tidak valid")
	}
	if body.Quantity <= 0 {
		return a.DeleteKoperasiCartItem(c)
	}

	var product models.KoperasiProduct
	if err := a.DB.Where("id = ? AND school_id = ? AND is_active = true", body.ProductID, schoolID).First(&product).Error; err != nil {
		return utils.Error(c, 404, "Produk tidak ditemukan atau tidak aktif")
	}
	if body.Quantity > product.Stock {
		return utils.Error(c, 400, fmt.Sprintf("Stok produk %s tidak mencukupi", product.Name))
	}

	item := models.KoperasiCartItem{
		SchoolID:  schoolID,
		BuyerID:   buyerID,
		ProductID: product.ID,
		Quantity:  body.Quantity,
	}
	if err := a.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "school_id"}, {Name: "buyer_id"}, {Name: "product_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"quantity":   body.Quantity,
			"updated_at": time.Now().UTC(),
		}),
	}).Create(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan keranjang", err.Error())
	}

	return a.respondKoperasiCart(c, schoolID, buyerID, 200, "Keranjang diperbarui")
}

func (a *AppContext) DeleteKoperasiCartItem(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	buyerID := c.Locals("userID").(uint)

	var productID uint
	if raw := strings.TrimSpace(c.Params("productId")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			return utils.Error(c, 400, "Produk tidak valid")
		}
		productID = uint(parsed)
	} else {
		var body struct {
			ProductID uint `json:"product_id"`
		}
		if err := c.BodyParser(&body); err != nil {
			return utils.Error(c, 400, "Invalid request")
		}
		productID = body.ProductID
	}
	if productID == 0 {
		return utils.Error(c, 400, "Produk tidak valid")
	}

	if err := a.DB.Where("school_id = ? AND buyer_id = ? AND product_id = ?", schoolID, buyerID, productID).
		Delete(&models.KoperasiCartItem{}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus item keranjang", err.Error())
	}

	return a.respondKoperasiCart(c, schoolID, buyerID, 200, "Item keranjang dihapus")
}

func (a *AppContext) ClearKoperasiCart(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	buyerID := c.Locals("userID").(uint)

	if err := a.DB.Where("school_id = ? AND buyer_id = ?", schoolID, buyerID).
		Delete(&models.KoperasiCartItem{}).Error; err != nil {
		return utils.Error(c, 500, "Gagal mengosongkan keranjang", err.Error())
	}

	return utils.Success(c, 200, "Keranjang dikosongkan", fiber.Map{"items": []interface{}{}})
}

func (a *AppContext) GetKoperasiOrders(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	userID := c.Locals("userID").(uint)
	search := strings.TrimSpace(c.Query("search"))
	statusFilter := koperasiNormalizeStatus(c.Query("status"))
	paymentStatusFilter := koperasiNormalizeStatus(c.Query("payment_status"))
	paymentMethodFilter := ""
	if rawPaymentMethod := strings.TrimSpace(c.Query("payment_method")); rawPaymentMethod != "" {
		paymentMethodFilter = koperasiNormalizePaymentMethodValue(rawPaymentMethod)
	}
	page, limit := parseInventoryPagination(c)
	offset := (page - 1) * limit
	canManage := koperasiCanManage(role)

	query := a.DB.Table("koperasi_orders ko").
		Select(`
			ko.id,
			ko.school_id,
			ko.order_number,
			ko.buyer_id,
			ko.buyer_role,
			ko.status,
			ko.payment_method,
			ko.payment_provider,
			ko.payment_status,
			ko.payment_request_id,
			ko.payment_qr_string,
			ko.payment_expires_at,
			ko.paid_at,
			ko.note,
			ko.total_amount,
			ko.handled_by,
			ko.created_at,
			ko.updated_at,
			COALESCE(NULLIF(b.full_name, ''), '-') AS buyer_name,
			COALESCE(cl.class_name, '-') AS buyer_class_name,
			COALESCE(h.full_name, h.username) AS handled_by_name
		`).
		Joins("INNER JOIN users b ON b.id = ko.buyer_id").
		Joins("LEFT JOIN student_class_enrollments sce ON sce.student_id = b.id AND sce.school_id = ko.school_id AND sce.is_active = true").
		Joins("LEFT JOIN class cl ON cl.id = COALESCE(b.class_id, sce.class_id)").
		Joins("LEFT JOIN users h ON h.id = ko.handled_by").
		Where("ko.school_id = ?", schoolID)
	if !canManage {
		query = query.Where("ko.buyer_id = ?", userID)
	}
	if statusFilter != "" && koperasiAllowedStatus(statusFilter) {
		query = query.Where("ko.status = ?", statusFilter)
	}
	if paymentStatusFilter != "" {
		query = query.Where("UPPER(COALESCE(ko.payment_status, '')) = ?", paymentStatusFilter)
	}
	if paymentMethodFilter != "" {
		query = query.Where("UPPER(COALESCE(ko.payment_method, '')) = ?", paymentMethodFilter)
	}
	if search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where(`
			LOWER(COALESCE(ko.order_number, '')) LIKE ? OR
			LOWER(COALESCE(b.full_name, '')) LIKE ? OR
			LOWER(COALESCE(cl.class_name, '')) LIKE ?
		`, pattern, pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pesanan koperasi", err.Error())
	}

	var orders []map[string]interface{}
	if err := query.Order("ko.created_at DESC, ko.id DESC").Limit(limit).Offset(offset).Scan(&orders).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pesanan koperasi", err.Error())
	}
	normalizeKoperasiOrderMaps(orders)
	for _, order := range orders {
		koperasiOrderResponseAugment(order)
	}

	orderIDs := make([]uint, 0, len(orders))
	for _, item := range orders {
		if id, ok := item["id"].(int64); ok && id > 0 {
			orderIDs = append(orderIDs, uint(id))
		}
	}

	itemsByOrder := map[uint][]map[string]interface{}{}
	if len(orderIDs) > 0 {
		var orderItems []map[string]interface{}
		if err := a.DB.Table("koperasi_order_items koi").
			Select(`
				koi.id,
				koi.order_id,
				koi.product_id,
				koi.quantity,
				koi.price,
				koi.subtotal,
				koi.product_name_snapshot,
				koi.product_code_snapshot,
				koi.product_category_snapshot,
				kp.image_url
			`).
			Joins("LEFT JOIN koperasi_products kp ON kp.id = koi.product_id").
			Where("koi.order_id IN ?", orderIDs).
			Order("koi.id ASC").
			Scan(&orderItems).Error; err != nil {
			return utils.Error(c, 500, "Gagal memuat item pesanan koperasi", err.Error())
		}

		for _, item := range orderItems {
			orderID := uint(0)
			switch raw := item["order_id"].(type) {
			case int64:
				orderID = uint(raw)
			case int:
				orderID = uint(raw)
			case uint:
				orderID = raw
			}
			if orderID == 0 {
				continue
			}
			itemsByOrder[orderID] = append(itemsByOrder[orderID], item)
		}
	}

	for idx := range orders {
		orderID := uint(0)
		switch raw := orders[idx]["id"].(type) {
		case int64:
			orderID = uint(raw)
		case int:
			orderID = uint(raw)
		case uint:
			orderID = raw
		}
		orders[idx]["items"] = itemsByOrder[orderID]
		orders[idx]["item_count"] = len(itemsByOrder[orderID])
	}

	return utils.Success(c, 200, "Success Get Koperasi Orders", fiber.Map{
		"items":      recentOrEmpty(orders),
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPagesFromCount(total, limit),
	})
}

func (a *AppContext) CreateKoperasiOrder(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	buyerID := c.Locals("userID").(uint)
	buyerRole := koperasiRoleOrEmpty(c.Locals("userRole"))

	var body struct {
		PaymentMethod string `json:"payment_method"`
		Note          string `json:"note"`
		Items         []struct {
			ProductID uint `json:"product_id"`
			Quantity  int  `json:"quantity"`
		} `json:"items"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	if len(body.Items) == 0 {
		return utils.Error(c, 400, "Minimal satu produk harus dipilih")
	}

	paymentMethod, err := koperasiNormalizePaymentMethod(body.PaymentMethod)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}
	paymentProvider := koperasiPaymentProviderCash
	if paymentMethod == koperasiPaymentMethodNonCash {
		paymentProvider = koperasiPaymentProviderXendit
	}
	paymentStatus := koperasiPaymentStatusForMethod(paymentMethod)

	note := strings.TrimSpace(body.Note)
	orderNumber := koperasiOrderNumber()
	now := time.Now().UTC()
	var orderExpiresAt *time.Time
	if paymentMethod == koperasiPaymentMethodNonCash {
		orderExpiresAt = koperasiPaymentExpiresAt()
	}

	var createdOrder models.KoperasiOrder
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		order := models.KoperasiOrder{
			SchoolID:         schoolID,
			OrderNumber:      orderNumber,
			BuyerID:          buyerID,
			BuyerRole:        buyerRole,
			Status:           koperasiStatusPending,
			PaymentMethod:    &paymentMethod,
			PaymentProvider:  &paymentProvider,
			PaymentStatus:    paymentStatus,
			PaymentExpiresAt: orderExpiresAt,
			Note:             stringPtrIfNotBlank(note),
			TotalAmount:      0,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		var totalAmount int64
		orderItems := make([]models.KoperasiOrderItem, 0, len(body.Items))
		for _, input := range body.Items {
			if input.ProductID == 0 {
				return fiber.NewError(400, "Produk tidak valid")
			}
			quantity := input.Quantity
			if quantity <= 0 {
				quantity = 1
			}

			var product models.KoperasiProduct
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND school_id = ? AND is_active = true", input.ProductID, schoolID).
				First(&product).Error; err != nil {
				return fiber.NewError(400, "Produk tidak ditemukan atau tidak aktif")
			}
			if product.Stock < quantity {
				return fiber.NewError(400, fmt.Sprintf("Stok produk %s tidak mencukupi", product.Name))
			}

			subtotal := product.Price * int64(quantity)
			totalAmount += subtotal
			orderItems = append(orderItems, models.KoperasiOrderItem{
				OrderID:                 order.ID,
				ProductID:               product.ID,
				Quantity:                quantity,
				Price:                   product.Price,
				Subtotal:                subtotal,
				ProductNameSnapshot:     product.Name,
				ProductCodeSnapshot:     product.Code,
				ProductCategorySnapshot: product.Category,
			})

			if err := tx.Model(&models.KoperasiProduct{}).
				Where("id = ? AND school_id = ?", product.ID, schoolID).
				Update("stock", gorm.Expr("stock - ?", quantity)).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&models.KoperasiOrder{}).
			Where("id = ?", order.ID).
			Updates(map[string]interface{}{
				"total_amount": totalAmount,
			}).Error; err != nil {
			return err
		}

		for _, item := range orderItems {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}

		createdOrder = order
		createdOrder.TotalAmount = totalAmount
		createdOrder.PaymentMethod = &paymentMethod
		createdOrder.PaymentProvider = &paymentProvider
		createdOrder.PaymentStatus = paymentStatus
		createdOrder.PaymentExpiresAt = orderExpiresAt
		createdOrder.Note = stringPtrIfNotBlank(note)
		createdOrder.CreatedAt = now
		return nil
	})
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat pesanan koperasi", err.Error())
	}

	clearCartItems := func() {
		_ = a.DB.Where("school_id = ? AND buyer_id = ?", schoolID, buyerID).Delete(&models.KoperasiCartItem{}).Error
	}

	if paymentMethod == koperasiPaymentMethodNonCash {
		qrisResp, qrisErr := createXenditKoperasiQrisPayment(createdOrder.OrderNumber, createdOrder.TotalAmount, schoolID)
		if qrisErr != nil {
			_ = a.DB.Transaction(func(tx *gorm.DB) error {
				var items []models.KoperasiOrderItem
				if err := tx.Where("order_id = ?", createdOrder.ID).Find(&items).Error; err != nil {
					return err
				}
				for _, item := range items {
					if err := tx.Model(&models.KoperasiProduct{}).
						Where("id = ? AND school_id = ?", item.ProductID, schoolID).
						Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
						return err
					}
				}

				failedStatus := koperasiPaymentStatusFailed
				failedProvider := koperasiPaymentProviderXendit
				cancelStatus := koperasiStatusCanceled
				return tx.Model(&models.KoperasiOrder{}).
					Where("id = ? AND school_id = ?", createdOrder.ID, schoolID).
					Updates(map[string]interface{}{
						"status":           cancelStatus,
						"payment_provider": failedProvider,
						"payment_status":   failedStatus,
						"updated_at":       time.Now(),
					}).Error
			})
			return utils.Error(c, 500, "Gagal membuat QRIS pembayaran", qrisErr.Error())
		}

		qrString := extractKoperasiQrisString(qrisResp)
		if qrString == "" {
			_ = a.DB.Transaction(func(tx *gorm.DB) error {
				var items []models.KoperasiOrderItem
				if err := tx.Where("order_id = ?", createdOrder.ID).Find(&items).Error; err != nil {
					return err
				}
				for _, item := range items {
					if err := tx.Model(&models.KoperasiProduct{}).
						Where("id = ? AND school_id = ?", item.ProductID, schoolID).
						Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
						return err
					}
				}
				return tx.Model(&models.KoperasiOrder{}).
					Where("id = ? AND school_id = ?", createdOrder.ID, schoolID).
					Updates(map[string]interface{}{
						"status":         koperasiStatusCanceled,
						"payment_status": koperasiPaymentStatusFailed,
						"updated_at":     time.Now(),
					}).Error
			})
			return utils.Error(c, 500, "QRIS tidak tersedia dari Xendit")
		}

		now := time.Now()
		if err := a.DB.Model(&models.KoperasiOrder{}).
			Where("id = ? AND school_id = ?", createdOrder.ID, schoolID).
			Updates(map[string]interface{}{
				"payment_provider":   koperasiPaymentProviderXendit,
				"payment_status":     koperasiPaymentStatusPending,
				"payment_request_id": qrisResp.PaymentRequestID,
				"payment_qr_string":  qrString,
				"payment_expires_at": orderExpiresAt,
				"updated_at":         now,
			}).Error; err != nil {
			_ = a.DB.Transaction(func(tx *gorm.DB) error {
				var items []models.KoperasiOrderItem
				if err := tx.Where("order_id = ?", createdOrder.ID).Find(&items).Error; err != nil {
					return err
				}
				for _, item := range items {
					if err := tx.Model(&models.KoperasiProduct{}).
						Where("id = ? AND school_id = ?", item.ProductID, schoolID).
						Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
						return err
					}
				}
				return tx.Model(&models.KoperasiOrder{}).
					Where("id = ? AND school_id = ?", createdOrder.ID, schoolID).
					Updates(map[string]interface{}{
						"status":         koperasiStatusCanceled,
						"payment_status": koperasiPaymentStatusFailed,
						"updated_at":     time.Now(),
					}).Error
			})
			return utils.Error(c, 500, "Gagal menyimpan QRIS koperasi", err.Error())
		}

		createdOrder.PaymentProvider = &paymentProvider
		createdOrder.PaymentStatus = koperasiPaymentStatusPending
		createdOrder.PaymentRequestID = &qrisResp.PaymentRequestID
		createdOrder.PaymentQRString = &qrString
		createdOrder.PaymentExpiresAt = orderExpiresAt

		if err := a.DB.Transaction(func(tx *gorm.DB) error {
			return a.createKoperasiPaymentLog(tx, createdOrder, "CREATED", koperasiPaymentStatusPending, &qrisResp.PaymentRequestID, "QRIS dibuat")
		}); err != nil {
			return utils.Error(c, 500, "Gagal mencatat log pembayaran koperasi", err.Error())
		}

		clearCartItems()
		a.broadcastKoperasiOrderEvent("koperasi:order-created", createdOrder.ID, "CREATED", schoolID)

		return a.respondKoperasiOrder(c, createdOrder.ID, 201, "Pesanan koperasi berhasil dibuat")
	}

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		return a.createKoperasiPaymentLog(tx, createdOrder, "CREATED", koperasiPaymentStatusCashDue, nil, "Pembayaran tunai di tempat")
	}); err != nil {
		return utils.Error(c, 500, "Gagal mencatat log pembayaran koperasi", err.Error())
	}

	clearCartItems()
	a.broadcastKoperasiOrderEvent("koperasi:order-created", createdOrder.ID, "CREATED", schoolID)

	return a.respondKoperasiOrder(c, createdOrder.ID, 201, "Pesanan koperasi berhasil dibuat")
}

func (a *AppContext) UpdateKoperasiOrderStatus(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	canManage := koperasiCanManage(role)

	id := c.Params("id")
	var current models.KoperasiOrder
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Pesanan tidak ditemukan")
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	nextStatus := koperasiNormalizeStatus(body.Status)
	if !koperasiAllowedStatus(nextStatus) {
		return utils.Error(c, 400, "Status pesanan tidak valid")
	}
	if !canManage {
		if current.BuyerID != userID {
			return utils.Error(c, 403, "Forbidden: Insufficient privileges")
		}
		if nextStatus != koperasiStatusCanceled {
			return utils.Error(c, 403, "Pesanan hanya bisa dibatalkan oleh pemiliknya")
		}
		if current.Status != koperasiStatusPending {
			return utils.Error(c, 400, "Pesanan hanya bisa dibatalkan saat masih pending")
		}
	}
	if current.Status == koperasiStatusCompleted && nextStatus != koperasiStatusCompleted {
		return utils.Error(c, 400, "Pesanan yang sudah selesai tidak bisa diubah")
	}
	if current.Status == koperasiStatusCanceled && nextStatus != koperasiStatusCanceled {
		return utils.Error(c, 400, "Pesanan yang sudah dibatalkan tidak bisa diubah")
	}

	err := a.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status": nextStatus,
		}
		if canManage {
			updates["handled_by"] = userID
		}
		if nextStatus == koperasiStatusCompleted && current.PaymentMethod != nil && koperasiNormalizePaymentMethodValue(*current.PaymentMethod) == koperasiPaymentMethodCash {
			now := time.Now()
			updates["payment_status"] = koperasiPaymentStatusPaid
			updates["paid_at"] = &now
		}

		if nextStatus == koperasiStatusCanceled && current.Status != koperasiStatusCanceled {
			if current.PaymentMethod != nil && koperasiNormalizePaymentMethodValue(*current.PaymentMethod) == koperasiPaymentMethodNonCash && current.PaymentStatus != koperasiPaymentStatusPaid {
				updates["payment_status"] = koperasiPaymentStatusFailed
			}
			var items []models.KoperasiOrderItem
			if err := tx.Where("order_id = ?", current.ID).Find(&items).Error; err != nil {
				return err
			}
			for _, item := range items {
				if err := tx.Model(&models.KoperasiProduct{}).
					Where("id = ? AND school_id = ?", item.ProductID, schoolID).
					Update("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
					return err
				}
			}
		}

		if err := tx.Model(&models.KoperasiOrder{}).Where("id = ? AND school_id = ?", current.ID, schoolID).Updates(updates).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return utils.Error(c, 500, "Gagal memperbarui status pesanan koperasi", err.Error())
	}

	a.broadcastKoperasiOrderEvent("koperasi:order-updated", current.ID, "STATUS_UPDATED", schoolID)

	return a.respondKoperasiOrder(c, current.ID, 200, "Status pesanan koperasi berhasil diperbarui")
}

func (a *AppContext) GetKoperasiOrderByID(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	userID := c.Locals("userID").(uint)
	canManage := koperasiCanManage(role)

	id := c.Params("id")
	var order models.KoperasiOrder
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&order).Error; err != nil {
		return utils.Error(c, 404, "Pesanan tidak ditemukan")
	}
	if !canManage && order.BuyerID != userID {
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}

	return a.respondKoperasiOrder(c, order.ID, 200, "Success Get Koperasi Order")
}

func (a *AppContext) SimulateKoperasiSandboxPayment(c *fiber.Ctx) error {
	if !koperasiXenditSandboxEnabled() {
		return utils.Error(c, 403, "Simulasi pembayaran hanya tersedia pada mode sandbox")
	}

	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	canManage := koperasiCanManage(role)
	id := c.Params("id")

	var order models.KoperasiOrder
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&order).Error; err != nil {
		return utils.Error(c, 404, "Pesanan tidak ditemukan")
	}
	if !canManage && order.BuyerID != userID {
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}

	if koperasiNormalizePaymentMethodValue(koperasiStringValue(order.PaymentMethod)) != koperasiPaymentMethodNonCash {
		return utils.Error(c, 400, "Simulasi hanya untuk pembayaran non tunai")
	}
	if strings.EqualFold(order.PaymentStatus, koperasiPaymentStatusPaid) {
		return a.respondKoperasiOrder(c, order.ID, 200, "Pesanan sudah lunas")
	}

	paymentRequestID := koperasiStringValue(order.PaymentRequestID)
	if paymentRequestID == "" {
		paymentRequestID = fmt.Sprintf("sandbox-%s", order.OrderNumber)
	}
	if err := a.applyKoperasiPaymentWebhook(order.OrderNumber, paymentRequestID, "SUCCEEDED"); err != nil {
		return utils.Error(c, 500, "Gagal mensimulasikan pembayaran sandbox", err.Error())
	}

	return a.respondKoperasiOrder(c, order.ID, 200, "Pembayaran sandbox berhasil disimulasikan")
}

func (a *AppContext) ReissueKoperasiPayment(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	role := koperasiRoleOrEmpty(c.Locals("userRole"))
	canManage := koperasiCanManage(role)
	id := c.Params("id")

	var current models.KoperasiOrder
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Pesanan tidak ditemukan")
	}
	if !canManage && current.BuyerID != userID {
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}

	if koperasiNormalizePaymentMethodValue(koperasiStringValue(current.PaymentMethod)) != koperasiPaymentMethodNonCash {
		return utils.Error(c, 400, "Bayar ulang hanya tersedia untuk pembayaran non tunai")
	}
	if strings.EqualFold(current.Status, koperasiStatusCanceled) {
		return utils.Error(c, 400, "Pesanan yang sudah dibatalkan tidak bisa dibayar ulang")
	}
	if strings.EqualFold(current.PaymentStatus, koperasiPaymentStatusPaid) {
		return utils.Error(c, 400, "Pesanan sudah lunas")
	}
	if current.PaymentExpiresAt != nil && time.Now().UTC().Before(current.PaymentExpiresAt.UTC()) {
		return utils.Error(c, 400, "QRIS belum expired")
	}

	qrisResp, qrisErr := createXenditKoperasiQrisPayment(current.OrderNumber, current.TotalAmount, schoolID)
	if qrisErr != nil {
		return utils.Error(c, 500, "Gagal membuat QRIS pembayaran", qrisErr.Error())
	}

	qrString := extractKoperasiQrisString(qrisResp)
	if qrString == "" {
		return utils.Error(c, 500, "QRIS tidak tersedia dari Xendit")
	}

	expiresAt := koperasiPaymentExpiresAt()
	now := time.Now()
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		newPaymentRequestID := strings.TrimSpace(qrisResp.PaymentRequestID)
		xenditProvider := koperasiPaymentProviderXendit
		updates := map[string]interface{}{
			"payment_provider":   koperasiPaymentProviderXendit,
			"payment_status":     koperasiPaymentStatusPending,
			"payment_request_id": newPaymentRequestID,
			"payment_qr_string":  qrString,
			"payment_expires_at": expiresAt,
			"updated_at":         now,
		}
		if err := tx.Model(&models.KoperasiOrder{}).Where("id = ? AND school_id = ?", current.ID, schoolID).Updates(updates).Error; err != nil {
			return err
		}

		current.PaymentProvider = &xenditProvider
		current.PaymentStatus = koperasiPaymentStatusPending
		current.PaymentRequestID = &newPaymentRequestID
		current.PaymentQRString = &qrString
		current.PaymentExpiresAt = expiresAt
		current.UpdatedAt = now
		return a.createKoperasiPaymentLog(tx, current, "REISSUED", koperasiPaymentStatusPending, &newPaymentRequestID, "QRIS dibuat ulang")
	})
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat ulang QRIS koperasi", err.Error())
	}

	a.broadcastKoperasiOrderEvent("koperasi:order-updated", current.ID, "REISSUED", schoolID)

	return a.respondKoperasiOrder(c, current.ID, 200, "QRIS pembayaran berhasil dibuat ulang")
}

func (a *AppContext) applyKoperasiPaymentWebhook(referenceID, paymentRequestID, status string) error {
	referenceID = strings.TrimSpace(referenceID)
	if referenceID == "" {
		return fmt.Errorf("reference_id is required")
	}

	var order models.KoperasiOrder
	if err := a.DB.Where("order_number = ?", referenceID).First(&order).Error; err != nil {
		return err
	}

	normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
	currentPaymentStatus := strings.ToUpper(strings.TrimSpace(order.PaymentStatus))
	currentOrderStatus := strings.ToUpper(strings.TrimSpace(order.Status))
	if currentPaymentStatus == koperasiPaymentStatusPaid {
		return nil
	}
	if !koperasiPaymentReferenceMatches(order.PaymentRequestID, &paymentRequestID) {
		return nil
	}
	if (normalizedStatus == "SUCCEEDED" || normalizedStatus == "SUCCESS" || normalizedStatus == "PAID" || normalizedStatus == "COMPLETED" || normalizedStatus == "SETTLED" || normalizedStatus == "CAPTURED") && currentOrderStatus == koperasiStatusCanceled {
		return nil
	}
	now := time.Now()
	updates := map[string]interface{}{
		"updated_at":       now,
		"payment_provider": koperasiPaymentProviderXendit,
	}
	if strings.TrimSpace(paymentRequestID) != "" {
		updates["payment_request_id"] = paymentRequestID
	}

	switch normalizedStatus {
	case "SUCCEEDED", "SUCCESS", "PAID", "COMPLETED", "SETTLED", "CAPTURED":
		updates["payment_status"] = koperasiPaymentStatusPaid
		updates["paid_at"] = &now
	case "EXPIRED":
		updates["payment_status"] = koperasiPaymentStatusExpired
	case "FAILED", "CANCELED", "CANCELLED", "REJECTED":
		updates["payment_status"] = koperasiPaymentStatusFailed
	default:
		updates["payment_status"] = koperasiPaymentStatusPending
	}

	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.KoperasiOrder{}).Where("id = ? AND school_id = ?", order.ID, order.SchoolID).Updates(updates).Error; err != nil {
			return err
		}
		eventType := koperasiPaymentEventTypeFromStatus(normalizedStatus)
		if eventType != "" {
			updatedOrder := order
			xenditProvider := koperasiPaymentProviderXendit
			updatedOrder.PaymentProvider = &xenditProvider
			if updatedStatus := updates["payment_status"]; updatedStatus != nil {
				updatedOrder.PaymentStatus = fmt.Sprint(updatedStatus)
			}
			if nextRequestID, ok := updates["payment_request_id"].(string); ok && strings.TrimSpace(nextRequestID) != "" {
				updatedOrder.PaymentRequestID = &nextRequestID
			}
			if nextExpiresAt, ok := updates["payment_expires_at"].(*time.Time); ok {
				updatedOrder.PaymentExpiresAt = nextExpiresAt
			}
			if normalizedStatus == "SUCCEEDED" || normalizedStatus == "SUCCESS" || normalizedStatus == "PAID" || normalizedStatus == "COMPLETED" || normalizedStatus == "SETTLED" || normalizedStatus == "CAPTURED" {
				updatedOrder.PaidAt = &now
			}
			if err := a.createKoperasiPaymentLog(tx, updatedOrder, eventType, updatedOrder.PaymentStatus, updatedOrder.PaymentRequestID, fmt.Sprintf("Webhook %s", normalizedStatus)); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	if normalizedStatus == "SUCCEEDED" || normalizedStatus == "SUCCESS" || normalizedStatus == "PAID" || normalizedStatus == "COMPLETED" || normalizedStatus == "SETTLED" || normalizedStatus == "CAPTURED" || normalizedStatus == "EXPIRED" || normalizedStatus == "FAILED" || normalizedStatus == "CANCELED" || normalizedStatus == "CANCELLED" || normalizedStatus == "REJECTED" {
		a.broadcastKoperasiOrderEvent("koperasi:order-updated", order.ID, normalizedStatus, order.SchoolID)
	}

	return nil
}

func koperasiParseReportBoundary(value string, location *time.Location, endOfDay bool) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	layouts := []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"}
	var parsed time.Time
	var err error
	for _, layout := range layouts {
		parsed, err = time.ParseInLocation(layout, trimmed, location)
		if err == nil {
			if layout == "2006-01-02" {
				if endOfDay {
					return time.Date(parsed.Year(), parsed.Month(), parsed.Day()+1, 0, 0, 0, 0, location), nil
				}
				return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, location), nil
			}
			return parsed.In(location), nil
		}
	}
	return time.Time{}, err
}

func koperasiReportLabelFromInput(raw string, value time.Time, location *time.Location) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return value.In(location).Format("02 Jan 2006")
	}
	if parsed, err := time.ParseInLocation("2006-01-02", trimmed, location); err == nil {
		return parsed.Format("02 Jan 2006")
	}
	if parsed, err := time.ParseInLocation(time.RFC3339, trimmed, location); err == nil {
		return parsed.In(location).Format("02 Jan 2006 15:04")
	}
	return value.In(location).Format("02 Jan 2006")
}

func koperasiJakartaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

func koperasiParseReportRange(c *fiber.Ctx) (time.Time, time.Time, string, string, error) {
	loc := koperasiJakartaLocation()
	now := time.Now().In(loc)
	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))

	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	fromLabel := from.Format("02 Jan 2006")
	if fromRaw != "" {
		parsed, err := koperasiParseReportBoundary(fromRaw, loc, false)
		if err != nil {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("parameter from tidak valid")
		}
		from = parsed
		fromLabel = koperasiReportLabelFromInput(fromRaw, from, loc)
	}

	to := now.Add(time.Second)
	toLabel := now.Format("02 Jan 2006")
	if toRaw != "" {
		parsed, err := koperasiParseReportBoundary(toRaw, loc, true)
		if err != nil {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("parameter to tidak valid")
		}
		to = parsed
		toLabel = koperasiReportLabelFromInput(toRaw, to.Add(-time.Second), loc)
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, "", "", fmt.Errorf("rentang tanggal tidak valid")
	}

	return from.UTC(), to.UTC(), fromLabel, toLabel, nil
}

func (a *AppContext) GetKoperasiReportSummary(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	from, to, fromLabel, toLabel, err := koperasiParseReportRange(c)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	type kv struct {
		Key   string `json:"key"`
		Value int64  `json:"value"`
	}

	var overviewRows []kv
	if err := a.DB.Raw(`
		SELECT 'orders_total' AS key, COUNT(*)::bigint AS value
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ?
		UNION ALL
		SELECT 'orders_paid' AS key, COUNT(*)::bigint AS value
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ? AND payment_status = 'PAID'
		UNION ALL
		SELECT 'orders_pending' AS key, COUNT(*)::bigint AS value
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ? AND payment_status = 'PENDING'
		UNION ALL
		SELECT 'orders_expired' AS key, COUNT(*)::bigint AS value
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ? AND payment_status = 'EXPIRED'
		UNION ALL
		SELECT 'orders_failed' AS key, COUNT(*)::bigint AS value
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ? AND payment_status = 'FAILED'
		UNION ALL
		SELECT 'orders_canceled' AS key, COUNT(*)::bigint AS value
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ? AND status = 'CANCELED'
		UNION ALL
		SELECT 'revenue_total' AS key, COALESCE(SUM(total_amount), 0)::bigint AS value
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ? AND payment_status = 'PAID'
	`, schoolID, from, to, schoolID, from, to, schoolID, from, to, schoolID, from, to, schoolID, from, to, schoolID, from, to, schoolID, from, to).Scan(&overviewRows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat ringkasan koperasi", err.Error())
	}

	overview := map[string]int64{}
	for _, row := range overviewRows {
		overview[row.Key] = row.Value
	}

	var statusBreakdown []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT COALESCE(payment_status, '-') AS payment_status, COUNT(*)::bigint AS total
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY payment_status
		ORDER BY total DESC, payment_status ASC
	`, schoolID, from, to).Scan(&statusBreakdown).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat status pembayaran koperasi", err.Error())
	}

	var methodBreakdown []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT COALESCE(payment_method, '-') AS payment_method, COUNT(*)::bigint AS total
		FROM koperasi_orders
		WHERE school_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY payment_method
		ORDER BY total DESC, payment_method ASC
	`, schoolID, from, to).Scan(&methodBreakdown).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat metode pembayaran koperasi", err.Error())
	}

	var topProducts []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			koi.product_name_snapshot AS product_name,
			COALESCE(koi.product_code_snapshot, '-') AS product_code,
			COALESCE(koi.product_category_snapshot, '-') AS product_category,
			SUM(koi.quantity)::bigint AS total_quantity,
			SUM(koi.subtotal)::bigint AS revenue
		FROM koperasi_order_items koi
		INNER JOIN koperasi_orders ko ON ko.id = koi.order_id
		WHERE ko.school_id = ? AND ko.created_at >= ? AND ko.created_at < ?
		GROUP BY koi.product_name_snapshot, koi.product_code_snapshot, koi.product_category_snapshot
		ORDER BY total_quantity DESC, revenue DESC, product_name ASC
		LIMIT 10
	`, schoolID, from, to).Scan(&topProducts).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat produk terlaris koperasi", err.Error())
	}

	var lowStockItems []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			id,
			name,
			COALESCE(code, '-') AS code,
			COALESCE(category, '-') AS category,
			price,
			stock
		FROM koperasi_products
		WHERE school_id = ? AND is_active = true
		ORDER BY stock ASC, name ASC
		LIMIT 10
	`, schoolID).Scan(&lowStockItems).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat stok koperasi", err.Error())
	}

	return utils.Success(c, 200, "Success Get Koperasi Report Summary", fiber.Map{
		"generated_at":             jakartaNow().Format(time.RFC3339),
		"range":                    fiber.Map{"from": fromLabel, "to": toLabel},
		"overview":                 overview,
		"payment_status_breakdown": recentOrEmpty(statusBreakdown),
		"payment_method_breakdown": recentOrEmpty(methodBreakdown),
		"top_products":             recentOrEmpty(topProducts),
		"low_stock_items":          recentOrEmpty(lowStockItems),
	})
}

func (a *AppContext) XenditKoperasiWebhook(c *fiber.Ctx) error {
	expectedToken := strings.TrimSpace(os.Getenv("XENDIT_CALLBACK_TOKEN"))
	if expectedToken == "" {
		return utils.Error(c, 500, "XENDIT_CALLBACK_TOKEN is not configured")
	}
	headerToken := strings.TrimSpace(c.Get("X-Callback-Token"))
	if headerToken == "" {
		headerToken = strings.TrimSpace(c.Get("x-callback-token"))
	}
	if headerToken != expectedToken {
		return utils.Error(c, 401, "Invalid callback token")
	}

	var payload xenditKoperasiPaymentWebhook
	if err := c.BodyParser(&payload); err != nil {
		return utils.Error(c, 400, "Invalid webhook payload")
	}

	extract := func(value *xenditKoperasiPaymentWebhookValue) (string, string, string, string) {
		if value == nil {
			return "", "", "", ""
		}
		return strings.TrimSpace(value.Data.ReferenceID), strings.TrimSpace(value.Data.PaymentRequestID), strings.TrimSpace(value.Data.Status), strings.TrimSpace(value.Event)
	}

	referenceID, paymentRequestID, status, event := "", "", "", ""
	if referenceID == "" && payload.PaymentCapture != nil {
		referenceID, paymentRequestID, status, event = extract(&payload.PaymentCapture.Value)
	}
	if referenceID == "" && payload.PaymentAuthorization != nil {
		referenceID, paymentRequestID, status, event = extract(&payload.PaymentAuthorization.Value)
	}
	if referenceID == "" && payload.PaymentFailure != nil {
		referenceID, paymentRequestID, status, event = extract(&payload.PaymentFailure.Value)
	}
	if referenceID == "" && payload.PaymentRequest != nil {
		referenceID, paymentRequestID, status, event = extract(&payload.PaymentRequest.Value)
	}
	if status == "" {
		status = event
	}

	switch strings.ToLower(status) {
	case "payment.capture", "payment_session.completed":
		status = "SUCCEEDED"
	case "payment.failure":
		status = "FAILED"
	case "payment.expiry", "payment_request.expiry", "payment_session.expired":
		status = "EXPIRED"
	}

	if referenceID == "" {
		return utils.Error(c, 400, "reference_id is required")
	}
	if err := a.applyKoperasiPaymentWebhook(referenceID, paymentRequestID, status); err != nil {
		return utils.Error(c, 500, "Gagal memperbarui status pembayaran koperasi", err.Error())
	}

	return utils.Success(c, 200, "Webhook processed", fiber.Map{
		"reference_id":       referenceID,
		"payment_request_id": paymentRequestID,
		"status":             status,
	})
}

func (a *AppContext) respondKoperasiProduct(c *fiber.Ctx, productID uint, code int, message string) error {
	var item map[string]interface{}
	if err := a.DB.Table("koperasi_products").
		Select(`
			id,
			school_id,
			name,
			code,
			category,
			description,
			image_url,
			price,
			stock,
			is_active,
			created_by,
			updated_by,
			created_at,
			updated_at
		`).
		Where("id = ?", productID).
		Scan(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat produk koperasi", err.Error())
	}

	return utils.Success(c, code, message, item)
}

func (a *AppContext) respondKoperasiCart(c *fiber.Ctx, schoolID, buyerID uint, code int, message string) error {
	var items []map[string]interface{}
	if err := a.DB.Table("koperasi_cart_items c").
		Select(`
			c.id,
			c.school_id,
			c.buyer_id,
			c.product_id,
			c.quantity,
			c.created_at,
			c.updated_at,
			p.id AS product__id,
			p.school_id AS product__school_id,
			p.name AS product__name,
			p.code AS product__code,
			p.category AS product__category,
			p.description AS product__description,
			p.image_url AS product__image_url,
			p.price AS product__price,
			p.stock AS product__stock,
			p.is_active AS product__is_active
		`).
		Joins("INNER JOIN koperasi_products p ON p.id = c.product_id AND p.school_id = c.school_id").
		Where("c.school_id = ? AND c.buyer_id = ?", schoolID, buyerID).
		Order("c.updated_at DESC, c.id DESC").
		Scan(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat keranjang koperasi", err.Error())
	}

	for _, item := range items {
		for _, key := range []string{"created_at", "updated_at"} {
			if value, ok := item[key]; ok {
				item[key] = normalizeKoperasiJakartaDateTimeValue(value)
			}
		}
		item["product"] = map[string]interface{}{
			"id":          item["product__id"],
			"school_id":   item["product__school_id"],
			"name":        item["product__name"],
			"code":        item["product__code"],
			"category":    item["product__category"],
			"description": item["product__description"],
			"image_url":   item["product__image_url"],
			"price":       item["product__price"],
			"stock":       item["product__stock"],
			"is_active":   item["product__is_active"],
		}
		for _, key := range []string{
			"product__id",
			"product__school_id",
			"product__name",
			"product__code",
			"product__category",
			"product__description",
			"product__image_url",
			"product__price",
			"product__stock",
			"product__is_active",
		} {
			delete(item, key)
		}
	}

	return utils.Success(c, code, message, fiber.Map{
		"items": recentOrEmpty(items),
	})
}

func (a *AppContext) respondKoperasiOrder(c *fiber.Ctx, orderID uint, code int, message string) error {
	var order map[string]interface{}
	if err := a.DB.Table("koperasi_orders ko").
		Select(`
			ko.id,
			ko.school_id,
			ko.order_number,
			ko.buyer_id,
			ko.buyer_role,
			ko.status,
			ko.payment_method,
			ko.payment_provider,
			ko.payment_status,
			ko.payment_request_id,
			ko.payment_qr_string,
			ko.payment_expires_at,
			ko.paid_at,
			ko.note,
			ko.total_amount,
			ko.handled_by,
			ko.created_at,
			ko.updated_at,
			COALESCE(NULLIF(b.full_name, ''), '-') AS buyer_name,
			COALESCE(cl.class_name, '-') AS buyer_class_name,
			COALESCE(h.full_name, h.username) AS handled_by_name
		`).
		Joins("INNER JOIN users b ON b.id = ko.buyer_id").
		Joins("LEFT JOIN student_class_enrollments sce ON sce.student_id = b.id AND sce.school_id = ko.school_id AND sce.is_active = true").
		Joins("LEFT JOIN class cl ON cl.id = COALESCE(b.class_id, sce.class_id)").
		Joins("LEFT JOIN users h ON h.id = ko.handled_by").
		Where("ko.id = ?", orderID).
		Scan(&order).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pesanan koperasi", err.Error())
	}
	normalizeKoperasiOrderMap(order)
	koperasiOrderResponseAugment(order)

	var items []map[string]interface{}
	if err := a.DB.Table("koperasi_order_items koi").
		Select(`
			koi.id,
			koi.order_id,
			koi.product_id,
			koi.quantity,
			koi.price,
			koi.subtotal,
			koi.product_name_snapshot,
			koi.product_code_snapshot,
			koi.product_category_snapshot,
			kp.image_url
		`).
		Joins("LEFT JOIN koperasi_products kp ON kp.id = koi.product_id").
		Where("koi.order_id = ?", orderID).
		Order("koi.id ASC").
		Scan(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat item pesanan koperasi", err.Error())
	}

	order["items"] = recentOrEmpty(items)
	order["item_count"] = len(items)

	var paymentHistory []map[string]interface{}
	if err := a.DB.Table("koperasi_payment_logs").
		Select(`
			id,
			school_id,
			order_id,
			event_type,
			status,
			payment_request_id,
			note,
			metadata,
			created_at
		`).
		Where("order_id = ? AND school_id = ?", orderID, order["school_id"]).
		Order("created_at ASC, id ASC").
		Scan(&paymentHistory).Error; err == nil {
		for _, item := range paymentHistory {
			if value, ok := item["created_at"]; ok {
				item["created_at"] = normalizeKoperasiJakartaDateTimeValue(value)
			}
			if value, ok := item["status"]; ok {
				item["status_label"] = koperasiPaymentStatusLabel(fmt.Sprint(value), fmt.Sprint(order["payment_method"]))
			}
		}
	}
	order["payment_history"] = recentOrEmpty(paymentHistory)
	return utils.Success(c, code, message, order)
}

func normalizeKoperasiJakartaDateTimeValue(value interface{}) interface{} {
	switch t := value.(type) {
	case time.Time:
		return t.In(koperasiJakartaLocation()).Format("2006-01-02 15:04:05")
	case *time.Time:
		if t == nil {
			return nil
		}
		return t.In(koperasiJakartaLocation()).Format("2006-01-02 15:04:05")
	case string:
		parsed := koperasiParseTimestamp(t)
		if parsed == nil {
			return t
		}
		return parsed.In(koperasiJakartaLocation()).Format("2006-01-02 15:04:05")
	default:
		return value
	}
}
