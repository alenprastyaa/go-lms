package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"lms/models"
)

type pushNotificationMessage struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Kind     string `json:"kind,omitempty"`
	URL      string `json:"url,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Badge    string `json:"badge,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Renotify bool   `json:"renotify,omitempty"`
}

type pushNotificationTarget struct {
	UserID  uint
	Message pushNotificationMessage
}

type pushNotificationRecipient struct {
	UserID uint   `gorm:"column:user_id"`
	Role   string `gorm:"column:role"`
}

type vapidConfig struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

func loadVapidConfig() (string, string, string, error) {
	publicKey := strings.TrimSpace(os.Getenv("PUSH_VAPID_PUBLIC_KEY"))
	privateKey := strings.TrimSpace(os.Getenv("PUSH_VAPID_PRIVATE_KEY"))
	if publicKey == "" || privateKey == "" {
		return "", "", "", errors.New("VAPID key belum dikonfigurasi")
	}

	subject := "https://school-system.my.id"

	return publicKey, privateKey, subject, nil
}

func (a *AppContext) sendPushTargets(targets []pushNotificationTarget) error {
	if a == nil || a.DB == nil || len(targets) == 0 {
		return nil
	}

	publicKey, privateKey, subject, err := loadVapidConfig()
	if err != nil {
		return err
	}

	userIDSet := map[uint]struct{}{}
	for _, target := range targets {
		if target.UserID > 0 {
			userIDSet[target.UserID] = struct{}{}
		}
	}

	userIDs := make([]uint, 0, len(userIDSet))
	for userID := range userIDSet {
		userIDs = append(userIDs, userID)
	}
	if len(userIDs) == 0 {
		return nil
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })

	var subscriptions []models.PushSubscription
	if err := a.DB.Where("user_id IN ? AND is_active = true", userIDs).Find(&subscriptions).Error; err != nil {
		return err
	}
	if len(subscriptions) == 0 {
		return nil
	}

	targetMap := make(map[uint]pushNotificationMessage, len(targets))
	for _, target := range targets {
		if target.UserID > 0 {
			targetMap[target.UserID] = target.Message
		}
	}

	payloads := make(map[uint][]byte, len(targetMap))
	for userID, message := range targetMap {
		payload, marshalErr := json.Marshal(message)
		if marshalErr != nil {
			continue
		}
		payloads[userID] = payload
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	httpClient := &http.Client{Timeout: 12 * time.Second}
	expiredEndpoints := make([]string, 0)

	for _, subscription := range subscriptions {
		payload, ok := payloads[subscription.UserID]
		if !ok {
			continue
		}
		if strings.TrimSpace(subscription.Endpoint) == "" || strings.TrimSpace(subscription.P256DH) == "" || strings.TrimSpace(subscription.Auth) == "" {
			continue
		}

		resp, sendErr := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
			Endpoint: subscription.Endpoint,
			Keys: webpush.Keys{
				Auth:   subscription.Auth,
				P256dh: subscription.P256DH,
			},
		}, &webpush.Options{
			HTTPClient:      httpClient,
			Subscriber:      subject,
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			TTL:             30,
		})

		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
				expiredEndpoints = append(expiredEndpoints, subscription.Endpoint)
				continue
			}
		}

		if sendErr != nil {
			continue
		}
	}

	if len(expiredEndpoints) > 0 {
		_ = a.DB.Where("endpoint IN ?", expiredEndpoints).Delete(&models.PushSubscription{}).Error
	}

	return nil
}

func (a *AppContext) notifyAnnouncementPublished(announcement models.SchoolAnnouncement) error {
	roles := announcementPushRoles(announcement.TargetAudience)
	if len(roles) == 0 {
		return nil
	}

	var recipients []pushNotificationRecipient
	if err := a.DB.Table("users").
		Select("id AS user_id, role").
		Where("school_id = ? AND role IN ?", announcement.SchoolID, roles).
		Scan(&recipients).Error; err != nil {
		return err
	}

	targets := make([]pushNotificationTarget, 0, len(recipients))
	for _, recipient := range recipients {
		targets = append(targets, pushNotificationTarget{
			UserID: recipient.UserID,
			Message: pushNotificationMessage{
				Title:    announcement.Title,
				Body:     previewPushText(announcement.Content, 120),
				Kind:     "announcement",
				URL:      announcementRouteForRole(recipient.Role, announcement.ID),
				Icon:     "/pwa-icon.svg",
				Badge:    "/logo.png",
				Tag:      "announcement:" + fmt.Sprint(announcement.ID),
				Renotify: true,
			},
		})
	}

	return a.sendPushTargets(targets)
}

func (a *AppContext) notifyPrivateChatMessage(senderID, recipientID uint, senderName, messagePreview string) error {
	targets := []pushNotificationTarget{
		{
			UserID: recipientID,
			Message: pushNotificationMessage{
				Title:    senderName,
				Body:     previewPushText(messagePreview, 120),
				Kind:     "chat",
				URL:      "/private-chat?user=" + fmt.Sprint(senderID),
				Icon:     "/pwa-icon.svg",
				Badge:    "/logo.png",
				Tag:      "private-chat:" + fmt.Sprint(senderID) + ":" + fmt.Sprint(recipientID),
				Renotify: true,
			},
		},
	}

	return a.sendPushTargets(targets)
}

func (a *AppContext) notifySubjectChatMessage(subjectID string, senderID uint, senderName, messagePreview string) error {
	var subject struct {
		ID        uint   `gorm:"column:id"`
		SchoolID  uint   `gorm:"column:school_id"`
		ClassID   *uint  `gorm:"column:class_id"`
		TeacherID *uint  `gorm:"column:teacher_id"`
		Name      string `gorm:"column:name"`
	}
	if err := a.DB.Raw(`SELECT id, school_id, class_id, teacher_id, name FROM learning_subjects WHERE id = ? LIMIT 1`, subjectID).Scan(&subject).Error; err != nil {
		return err
	}
	if subject.ID == 0 {
		return nil
	}

	recipients := make([]pushNotificationRecipient, 0)
	if subject.TeacherID != nil && *subject.TeacherID != senderID {
		recipients = append(recipients, pushNotificationRecipient{UserID: *subject.TeacherID, Role: "GURU"})
	}
	if subject.ClassID != nil {
		var students []pushNotificationRecipient
		if err := a.DB.Table("users").
			Select("id AS user_id, role").
			Where("school_id = ? AND role = 'SISWA' AND class_id = ?", subject.SchoolID, *subject.ClassID).
			Scan(&students).Error; err != nil {
			return err
		}
		for _, student := range students {
			if student.UserID == senderID {
				continue
			}
			recipients = append(recipients, student)
		}
	}

	targets := make([]pushNotificationTarget, 0, len(recipients))
	for _, recipient := range recipients {
		targets = append(targets, pushNotificationTarget{
			UserID: recipient.UserID,
			Message: pushNotificationMessage{
				Title:    fmt.Sprintf("Chat %s", subject.Name),
				Body:     previewPushText(messagePreview, 120),
				Kind:     "chat",
				URL:      subjectChatRoute(recipient.Role, subjectID),
				Icon:     "/pwa-icon.svg",
				Badge:    "/logo.png",
				Tag:      "subject-chat:" + subjectID,
				Renotify: true,
			},
		})
	}

	return a.sendPushTargets(targets)
}

func (a *AppContext) notifyAssignmentCreated(subjectID string, assignmentID uint, assignmentType, title, description string) error {
	var subject struct {
		ID       uint  `gorm:"column:id"`
		SchoolID uint  `gorm:"column:school_id"`
		ClassID  *uint `gorm:"column:class_id"`
	}
	if err := a.DB.Raw(`SELECT id, school_id, class_id FROM learning_subjects WHERE id = ? LIMIT 1`, subjectID).Scan(&subject).Error; err != nil {
		return err
	}
	if subject.ID == 0 || subject.ClassID == nil {
		return nil
	}

	var recipients []pushNotificationRecipient
	if err := a.DB.Table("users").
		Select("id AS user_id, role").
		Where("school_id = ? AND role = 'SISWA' AND class_id = ?", subject.SchoolID, *subject.ClassID).
		Scan(&recipients).Error; err != nil {
		return err
	}

	kind := assignmentKindLabel(assignmentType)
	routes := assignmentRoute(assignmentType, subject.ID, assignmentID)
	targets := make([]pushNotificationTarget, 0, len(recipients))
	for _, recipient := range recipients {
		bodyText := previewPushText(description, 120)
		if strings.TrimSpace(bodyText) == "" {
			bodyText = previewPushText(title, 120)
		}

		targets = append(targets, pushNotificationTarget{
			UserID: recipient.UserID,
			Message: pushNotificationMessage{
				Title:    fmt.Sprintf("Ada %s baru", kind),
				Body:     bodyText,
				Kind:     "assignment",
				URL:      routes,
				Icon:     "/pwa-icon.svg",
				Badge:    "/logo.png",
				Tag:      "assignment:" + fmt.Sprint(assignmentID),
				Renotify: true,
			},
		})
	}

	return a.sendPushTargets(targets)
}

func (a *AppContext) notifyAnnouncementIfActive(item models.SchoolAnnouncement) error {
	if strings.ToUpper(strings.TrimSpace(item.Status)) != announcementStatusActive {
		return nil
	}
	return a.notifyAnnouncementPublished(item)
}
