package controllers

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

type privateChatPeerRow struct {
	UserID            uint       `json:"user_id"`
	Username          string     `json:"username"`
	FullName          string     `json:"full_name"`
	Role              string     `json:"role"`
	ProfileImage      string     `json:"profile_image"`
	ClassName         string     `json:"class_name"`
	LastReadMessageID int64      `json:"last_read_message_id"`
	UnreadCount       int        `json:"unread_count"`
	LastMessageAt     *time.Time `json:"last_message_at"`
	LastMessage       string     `json:"last_message"`
	LastMessageType   string     `json:"last_message_type"`
	LastSenderID      int64      `json:"last_sender_id"`
}

func (a *AppContext) ensurePrivateChatPeer(schoolID uint, userID uint, peerID uint) (map[string]interface{}, error) {
	if peerID == 0 {
		return nil, fiber.NewError(400, "penerima chat tidak valid")
	}
	if peerID == userID {
		return nil, fiber.NewError(400, "tidak bisa chat ke akun sendiri")
	}

	var peer map[string]interface{}
	a.DB.Raw(`
		SELECT u.id, u.username, u.full_name, u.role, u.profile_image, c.class_name
		FROM users u
		LEFT JOIN class c ON c.id = u.class_id
		WHERE u.id = ? AND u.school_id = ? AND u.role IN ('ADMIN', 'KOPERASI', 'GURU', 'SISWA')
		LIMIT 1
	`, peerID, schoolID).Scan(&peer)
	if len(peer) == 0 {
		return nil, fiber.NewError(404, "warga sekolah tidak ditemukan")
	}

	return peer, nil
}

func normalizePrivateChatMessagePreview(message, messageType, attachmentName string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed != "" {
		return trimmed
	}

	switch strings.ToUpper(strings.TrimSpace(messageType)) {
	case "VOICE":
		return "Voice note"
	case "IMAGE":
		return "Gambar"
	case "PDF":
		return "PDF"
	case "FILE":
		if strings.TrimSpace(attachmentName) != "" {
			return strings.TrimSpace(attachmentName)
		}
		return "File"
	default:
		if strings.TrimSpace(attachmentName) != "" {
			return strings.TrimSpace(attachmentName)
		}
		return ""
	}
}

func (a *AppContext) GetPrivateChatSummary(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)

	var rows []privateChatPeerRow
	a.DB.Raw(`
		WITH peer_base AS (
			SELECT DISTINCT
				CASE
					WHEN sender_id = @userID THEN recipient_id
					ELSE sender_id
				END AS peer_user_id
			FROM private_chat_messages
			WHERE school_id = @schoolID
			  AND (sender_id = @userID OR recipient_id = @userID)
		)
		SELECT
			u.id AS user_id,
			u.username,
			COALESCE(NULLIF(u.full_name, ''), u.username) AS full_name,
			u.role,
			COALESCE(u.profile_image, '') AS profile_image,
			COALESCE(cls.class_name, '') AS class_name,
			COALESCE(pcr.last_read_message_id, 0)::bigint AS last_read_message_id,
			COUNT(m.id) FILTER (
				WHERE m.sender_id = u.id
				  AND m.recipient_id = @userID
				  AND m.id > COALESCE(pcr.last_read_message_id, 0)
			)::int AS unread_count,
			MAX(m.created_at) AS last_message_at,
			COALESCE((ARRAY_AGG(m.message ORDER BY m.created_at DESC, m.id DESC))[1], '') AS last_message,
			COALESCE((ARRAY_AGG(m.message_type ORDER BY m.created_at DESC, m.id DESC))[1], 'TEXT') AS last_message_type,
			COALESCE((ARRAY_AGG(m.sender_id ORDER BY m.created_at DESC, m.id DESC))[1], 0)::bigint AS last_sender_id
		FROM peer_base pb
		INNER JOIN users u ON u.id = pb.peer_user_id
		LEFT JOIN class cls ON cls.id = u.class_id
		LEFT JOIN private_chat_reads pcr ON pcr.owner_user_id = @userID AND pcr.peer_user_id = u.id
		LEFT JOIN private_chat_messages m
			ON m.school_id = @schoolID
			AND (
				(m.sender_id = @userID AND m.recipient_id = u.id)
				OR
				(m.sender_id = u.id AND m.recipient_id = @userID)
			)
		WHERE u.school_id = @schoolID
		  AND u.role IN ('ADMIN', 'KOPERASI', 'GURU', 'SISWA')
		GROUP BY u.id, u.username, u.full_name, u.role, u.profile_image, cls.class_name, pcr.last_read_message_id
		ORDER BY last_message_at DESC NULLS LAST, COALESCE(NULLIF(u.full_name, ''), u.username) ASC
		`, sql.Named("userID", userID), sql.Named("schoolID", schoolID)).Scan(&rows)

	items := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		items = append(items, fiber.Map{
			"user_id":              row.UserID,
			"username":             row.Username,
			"full_name":            row.FullName,
			"role":                 row.Role,
			"profile_image":        row.ProfileImage,
			"class_name":           row.ClassName,
			"last_read_message_id": row.LastReadMessageID,
			"unread_count":         row.UnreadCount,
			"last_message_at":      normalizeJakartaDateTimeValue(row.LastMessageAt),
			"last_message":         normalizePrivateChatMessagePreview(row.LastMessage, row.LastMessageType, ""),
			"last_message_type":    row.LastMessageType,
			"last_sender_id":       row.LastSenderID,
		})
	}

	return utils.Success(c, 200, "Success Get Private Chat Summary", items)
}

func (a *AppContext) SearchPrivateChatContacts(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)
	keyword := strings.TrimSpace(c.Query("keyword"))
	limit := utils.ToInt(c.Query("limit", "20"), 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	var rows []map[string]interface{}
	query := `
		SELECT
			u.id AS user_id,
			u.username,
			COALESCE(NULLIF(u.full_name, ''), u.username) AS full_name,
			u.role,
			COALESCE(u.profile_image, '') AS profile_image,
			COALESCE(c.class_name, '') AS class_name
		FROM users u
		LEFT JOIN class c ON c.id = u.class_id
		WHERE u.school_id = ?
		  AND u.id <> ?
		  AND u.role IN ('ADMIN', 'KOPERASI', 'GURU', 'SISWA')
	`
	args := []interface{}{schoolID, userID}
	if keyword != "" {
		query += ` AND (
			COALESCE(u.full_name, '') ILIKE ?
			OR COALESCE(u.username, '') ILIKE ?
			OR COALESCE(u.role, '') ILIKE ?
			OR COALESCE(c.class_name, '') ILIKE ?
		)`
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY COALESCE(NULLIF(u.full_name, ''), u.username) ASC LIMIT ?`
	args = append(args, limit)

	a.DB.Raw(query, args...).Scan(&rows)
	return utils.Success(c, 200, "Success Search Private Chat Contacts", rows)
}

func (a *AppContext) GetPrivateChatMessages(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)
	peerID := uint(utils.ToInt(c.Params("peerUserId"), 0))

	if _, err := a.ensurePrivateChatPeer(schoolID, userID, peerID); err != nil {
		return utils.Error(c, err.(*fiber.Error).Code, err.Error())
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT
			m.*,
			sender.username AS sender_name,
			sender.full_name AS sender_full_name,
			sender.role AS sender_role,
			sender.profile_image AS sender_profile_image
		FROM private_chat_messages m
		LEFT JOIN users sender ON sender.id = m.sender_id
		WHERE m.school_id = ?
		  AND (
			(m.sender_id = ? AND m.recipient_id = ?)
			OR
			(m.sender_id = ? AND m.recipient_id = ?)
		  )
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT 150
	`, schoolID, userID, peerID, peerID, userID).Scan(&rows)
	normalizeJakartaDateTimeRows(rows, "created_at", "edited_at")

	return utils.Success(c, 200, "Success Get Private Chat Messages", rows)
}

func (a *AppContext) CreatePrivateChatMessage(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)
	peerID := uint(utils.ToInt(c.Params("peerUserId"), 0))

	peer, err := a.ensurePrivateChatPeer(schoolID, userID, peerID)
	if err != nil {
		return utils.Error(c, err.(*fiber.Error).Code, err.Error())
	}

	var body struct {
		Message            string  `json:"message"`
		Text               string  `json:"text"`
		MessageType        string  `json:"message_type"`
		AttachmentURL      string  `json:"attachment_url"`
		AttachmentName     string  `json:"attachment_name"`
		AttachmentMimeType string  `json:"attachment_mime_type"`
		AttachmentSize     float64 `json:"attachment_size"`
		ClientID           string  `json:"client_id"`
	}
	_ = c.BodyParser(&body)

	message := strings.TrimSpace(body.Message)
	if message == "" {
		message = strings.TrimSpace(body.Text)
	}
	if message == "" {
		message = strings.TrimSpace(c.FormValue("message"))
	}
	if message == "" {
		message = strings.TrimSpace(c.FormValue("text"))
	}
	messageTypeRaw := strings.TrimSpace(body.MessageType)
	if messageTypeRaw == "" {
		messageTypeRaw = strings.TrimSpace(c.FormValue("message_type"))
	}
	attachmentName := strings.TrimSpace(body.AttachmentName)
	if attachmentName == "" {
		attachmentName = strings.TrimSpace(c.FormValue("attachment_name"))
	}
	attachmentMimeType := strings.TrimSpace(body.AttachmentMimeType)
	if attachmentMimeType == "" {
		attachmentMimeType = strings.TrimSpace(c.FormValue("attachment_mime_type"))
	}
	attachmentSize := int(body.AttachmentSize)
	if attachmentSize == 0 {
		attachmentSize = utils.ToInt(c.FormValue("attachment_size"), 0)
	}
	attachment := strings.TrimSpace(body.AttachmentURL)
	attachmentPreviewURL := ""
	if f, err := c.FormFile("attachment"); err == nil && f != nil {
		uploaded, upErr := utils.SaveUploadedChatAttachment(c, f)
		if upErr == nil {
			attachment = uploaded.URL
			attachmentName = strings.TrimSpace(uploaded.FileName)
			attachmentMimeType = strings.TrimSpace(uploaded.ContentType)
			attachmentSize = uploaded.Size
			attachmentPreviewURL = strings.TrimSpace(uploaded.PreviewURL)
		} else {
			return utils.Error(c, 400, upErr.Error())
		}
	}
	if message == "" && attachment == "" {
		return utils.Error(c, 400, "message is required")
	}

	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(c.FormValue("client_id"))
	}
	messageType := detectChatMessageType(messageTypeRaw, attachmentMimeType, attachmentName, attachment)

	var row map[string]interface{}
	a.DB.Raw(`
		WITH inserted AS (
			INSERT INTO private_chat_messages (
				school_id,
				sender_id,
				recipient_id,
				message,
				message_type,
				attachment_url,
				attachment_preview_url,
				attachment_name,
				attachment_mime_type,
				attachment_size,
				created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
			RETURNING *
		)
		SELECT
			i.*,
			sender.username AS sender_name,
			sender.full_name AS sender_full_name,
			sender.role AS sender_role,
			sender.profile_image AS sender_profile_image
		FROM inserted i
		LEFT JOIN users sender ON sender.id = i.sender_id
		LIMIT 1
	`, schoolID, userID, peerID, message, messageType, nullIfEmpty(attachment), nullIfEmpty(attachmentPreviewURL), nullIfEmpty(attachmentName), nullIfEmpty(attachmentMimeType), nullIfZero(attachmentSize)).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 500, "Failed to create private chat message")
	}

	normalizeJakartaDateTimeFields(row, "created_at", "edited_at")
	row["peer_user_id"] = peerID
	row["peer_username"] = peer["username"]
	row["peer_full_name"] = peer["full_name"]
	row["origin_client_id"] = clientID

	if a.Realtime != nil {
		a.Realtime.BroadcastPrivateChatMessage(schoolID, userID, peerID, row)
	}

	senderLabel := strings.TrimSpace(fmt.Sprint(row["sender_full_name"]))
	if senderLabel == "" {
		senderLabel = strings.TrimSpace(fmt.Sprint(row["sender_name"]))
	}
	if senderLabel == "" {
		senderLabel = strings.TrimSpace(fmt.Sprint(row["sender_username"]))
	}
	if senderLabel == "" {
		senderLabel = strings.TrimSpace(fmt.Sprint(row["sender_role"]))
	}
	go func() {
		_ = a.notifyPrivateChatMessage(userID, peerID, senderLabel, normalizePrivateChatMessagePreview(message, messageType, attachmentName))
	}()

	return utils.Success(c, 201, "Success Create Private Chat Message", row)
}

func (a *AppContext) UpdatePrivateChatMessage(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)
	peerID := uint(utils.ToInt(c.Params("peerUserId"), 0))
	messageID := uint(utils.ToInt(c.Params("messageId"), 0))

	if _, err := a.ensurePrivateChatPeer(schoolID, userID, peerID); err != nil {
		return utils.Error(c, err.(*fiber.Error).Code, err.Error())
	}
	if messageID == 0 {
		return utils.Error(c, 400, "message id is required")
	}

	var body struct {
		Message string `json:"message"`
		Text    string `json:"text"`
	}
	_ = c.BodyParser(&body)

	message := strings.TrimSpace(body.Message)
	if message == "" {
		message = strings.TrimSpace(body.Text)
	}
	if message == "" {
		message = strings.TrimSpace(c.FormValue("message"))
	}
	if message == "" {
		message = strings.TrimSpace(c.FormValue("text"))
	}
	if message == "" {
		return utils.Error(c, 400, "message is required")
	}

	var row map[string]interface{}
	a.DB.Raw(`
		WITH updated AS (
			UPDATE private_chat_messages
			SET message = ?, edited_at = NOW()
			WHERE school_id = ?
			  AND id = ?
			  AND sender_id = ?
			  AND recipient_id = ?
			RETURNING *
		)
		SELECT
			u.*,
			sender.username AS sender_name,
			sender.full_name AS sender_full_name,
			sender.role AS sender_role,
			sender.profile_image AS sender_profile_image
		FROM updated u
		LEFT JOIN users sender ON sender.id = u.sender_id
		LIMIT 1
	`, message, schoolID, messageID, userID, peerID).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "private chat message not found")
	}

	normalizeJakartaDateTimeFields(row, "created_at", "edited_at")
	row["peer_user_id"] = peerID
	row["origin_client_id"] = strings.TrimSpace(c.FormValue("client_id"))

	if a.Realtime != nil {
		a.Realtime.BroadcastPrivateChatMessageUpdated(schoolID, userID, peerID, row)
	}

	return utils.Success(c, 200, "Success Update Private Chat Message", row)
}

func (a *AppContext) MarkPrivateChatAsRead(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)
	peerID := uint(utils.ToInt(c.Params("peerUserId"), 0))

	if _, err := a.ensurePrivateChatPeer(schoolID, userID, peerID); err != nil {
		return utils.Error(c, err.(*fiber.Error).Code, err.Error())
	}

	var body struct {
		ClientID string `json:"client_id"`
	}
	_ = c.BodyParser(&body)

	var last struct {
		ID int64 `json:"id"`
	}
	a.DB.Raw(`
		SELECT id
		FROM private_chat_messages
		WHERE school_id = ?
		  AND (
			(sender_id = ? AND recipient_id = ?)
			OR
			(sender_id = ? AND recipient_id = ?)
		  )
		ORDER BY id DESC
		LIMIT 1
	`, schoolID, userID, peerID, peerID, userID).Scan(&last)

	if last.ID == 0 {
		return utils.Success(c, 200, "Success Mark Private Chat As Read", fiber.Map{
			"peer_user_id":         peerID,
			"user_id":              userID,
			"last_read_message_id": nil,
		})
	}

	a.DB.Exec(`
		INSERT INTO private_chat_reads (owner_user_id, peer_user_id, last_read_message_id, last_read_at, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW(), NOW())
		ON CONFLICT (owner_user_id, peer_user_id)
		DO UPDATE SET
			last_read_message_id = GREATEST(
				COALESCE(private_chat_reads.last_read_message_id, 0),
				COALESCE(EXCLUDED.last_read_message_id, 0)
			),
			last_read_at = NOW(),
			updated_at = NOW()
	`, userID, peerID, last.ID)

	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(c.FormValue("client_id"))
	}

	payload := fiber.Map{
		"user_id":              userID,
		"peer_user_id":         peerID,
		"last_read_message_id": last.ID,
		"origin_client_id":     clientID,
	}
	if a.Realtime != nil {
		a.Realtime.BroadcastPrivateChatReadUpdated(schoolID, userID, peerID, payload)
	}

	return utils.Success(c, 200, "Success Mark Private Chat As Read", payload)
}
