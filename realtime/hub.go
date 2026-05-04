package realtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
)

type AuthClaims struct {
	UserID   uint
	SchoolID uint
	Role     string
}

type sseEvent struct {
	Name string
	Data []byte
}

type subscriber struct {
	id        uint64
	userID    uint
	schoolID  uint
	subjects  map[uint]struct{}
	ch        chan sseEvent
	closeOnce sync.Once
}

type subjectCacheEntry struct {
	subjects  map[uint]struct{}
	expiresAt time.Time
}

type Hub struct {
	db *gorm.DB

	mu           sync.RWMutex
	subscribers  map[uint64]*subscriber
	nextID       atomic.Uint64
	heartbeatGap time.Duration

	cacheMu          sync.RWMutex
	subjectsCache    map[string]subjectCacheEntry
	subjectsCacheTTL time.Duration
}

func NewHub(db *gorm.DB) *Hub {
	return &Hub{
		db:               db,
		subscribers:      make(map[uint64]*subscriber),
		heartbeatGap:     25 * time.Second,
		subjectsCache:    make(map[string]subjectCacheEntry),
		subjectsCacheTTL: 45 * time.Second,
	}
}

func (h *Hub) Handler() http.Handler {
	return http.HandlerFunc(h.serveHTTP)
}

func (h *Hub) FiberHandler(c *fiber.Ctx) error {
	claims, err := h.extractClaimsFromFiber(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	subjects := h.loadAllowedSubjects(claims)
	client := &subscriber{
		id:       h.nextID.Add(1),
		userID:   claims.UserID,
		schoolID: claims.SchoolID,
		subjects: subjects,
		ch:       make(chan sseEvent, 64),
	}

	h.register(client)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache, no-transform")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	if origin := allowedOrigin(c.Get("Origin")); origin != "" {
		c.Set("Access-Control-Allow-Origin", origin)
		c.Set("Access-Control-Allow-Credentials", "true")
	}

	done := c.Context().Done()
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.unregister(client.id)

		if err := writeSSEToWriter(w, "realtime:connected", map[string]any{
			"success":      true,
			"user_id":      client.userID,
			"school_id":    client.schoolID,
			"subjects":     len(client.subjects),
			"online_count": h.OnlineCountBySchool(client.schoolID),
		}); err != nil {
			return
		}
		_ = w.Flush()

		heartbeat := time.NewTicker(h.heartbeatGap)
		defer heartbeat.Stop()

		for {
			select {
			case event, ok := <-client.ch:
				if !ok {
					return
				}

				if err := writeSSEToWriter(w, event.Name, json.RawMessage(event.Data)); err != nil {
					return
				}
				_ = w.Flush()
			case <-heartbeat.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
					return
				}
				_ = w.Flush()
			case <-done:
				return
			}
		}
	})

	return nil
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for id, sub := range h.subscribers {
		sub.closeOnce.Do(func() {
			close(sub.ch)
		})
		delete(h.subscribers, id)
	}
}

func (h *Hub) BroadcastSubjectChatMessage(subjectID any, payload any) {
	h.broadcastSubjectEvent("learning-chat:new-message", subjectID, payload)
}

func (h *Hub) BroadcastSubjectReadUpdated(subjectID any, payload any) {
	h.broadcastSubjectEvent("learning-chat:read-updated", subjectID, payload)
}

func (h *Hub) BroadcastSubjectTyping(subjectID any, payload any) {
	h.broadcastSubjectEvent("learning-chat:typing", subjectID, payload)
}

func (h *Hub) broadcastSubjectEvent(eventName string, subjectID any, payload any) {
	if h == nil {
		return
	}

	normalizedSubjectID := uint(firstInt([]any{subjectID}))
	if normalizedSubjectID == 0 {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscribers {
		if len(sub.subjects) > 0 {
			if _, ok := sub.subjects[normalizedSubjectID]; !ok {
				continue
			}
		}

		select {
		case sub.ch <- sseEvent{Name: eventName, Data: data}:
		default:
			// Drop stale events instead of blocking the whole broadcaster.
		}
	}
}

func (h *Hub) serveHTTP(w http.ResponseWriter, r *http.Request) {
	claims, err := h.extractClaims(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	subjects := h.loadAllowedSubjects(claims)
	client := &subscriber{
		id:       h.nextID.Add(1),
		userID:   claims.UserID,
		schoolID: claims.SchoolID,
		subjects: subjects,
		ch:       make(chan sseEvent, 64),
	}

	h.register(client)
	defer h.unregister(client.id)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin(r.Header.Get("Origin")))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	if origin := allowedOrigin(r.Header.Get("Origin")); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if err := writeSSE(w, "realtime:connected", map[string]any{
		"success":      true,
		"user_id":      client.userID,
		"school_id":    client.schoolID,
		"subjects":     len(client.subjects),
		"online_count": h.OnlineCountBySchool(client.schoolID),
	}); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(h.heartbeatGap)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case event, ok := <-client.ch:
			if !ok {
				return
			}

			if err := writeSSE(w, event.Name, json.RawMessage(event.Data)); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) register(client *subscriber) {
	h.mu.Lock()
	h.subscribers[client.id] = client
	schoolID := client.schoolID
	h.mu.Unlock()

	h.broadcastPresenceUpdate(schoolID)
}

func (h *Hub) unregister(id uint64) {
	h.mu.Lock()
	var schoolID uint
	if sub, ok := h.subscribers[id]; ok {
		schoolID = sub.schoolID
		sub.closeOnce.Do(func() {
			close(sub.ch)
		})
		delete(h.subscribers, id)
	}
	h.mu.Unlock()

	if schoolID != 0 {
		h.broadcastPresenceUpdate(schoolID)
	}
}

func (h *Hub) broadcastPresenceUpdate(schoolID uint) {
	if h == nil || schoolID == 0 {
		return
	}

	count := h.OnlineCountBySchool(schoolID)
	payload, err := json.Marshal(map[string]any{
		"school_id":    schoolID,
		"online_count": count,
		"updated_at":   time.Now().UnixMilli(),
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sub := range h.subscribers {
		if sub.schoolID != schoolID {
			continue
		}
		select {
		case sub.ch <- sseEvent{Name: "learning-presence:updated", Data: payload}:
		default:
		}
	}
}

func (h *Hub) OnlineCountBySchool(schoolID uint) int {
	if h == nil || schoolID == 0 {
		return 0
	}

	onlineUsers := map[uint]struct{}{}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sub := range h.subscribers {
		if sub.schoolID != schoolID || sub.userID == 0 {
			continue
		}
		onlineUsers[sub.userID] = struct{}{}
	}
	return len(onlineUsers)
}

func (h *Hub) SubjectOnlineUsers(schoolID, subjectID uint) []uint {
	if h == nil || schoolID == 0 || subjectID == 0 {
		return []uint{}
	}

	onlineUsers := map[uint]struct{}{}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sub := range h.subscribers {
		if sub.schoolID != schoolID || sub.userID == 0 {
			continue
		}
		if len(sub.subjects) > 0 {
			if _, ok := sub.subjects[subjectID]; !ok {
				continue
			}
		}
		onlineUsers[sub.userID] = struct{}{}
	}

	result := make([]uint, 0, len(onlineUsers))
	for userID := range onlineUsers {
		result = append(result, userID)
	}
	return result
}

func (h *Hub) loadAllowedSubjects(claims *AuthClaims) map[uint]struct{} {
	if h == nil || h.db == nil || claims == nil {
		return map[uint]struct{}{}
	}

	cacheKey := fmt.Sprintf("%d:%d:%s", claims.SchoolID, claims.UserID, strings.ToUpper(strings.TrimSpace(claims.Role)))
	if cached, ok := h.getSubjectsFromCache(cacheKey); ok {
		return cached
	}

	subjectIDs := make([]uint, 0)

	switch strings.ToUpper(strings.TrimSpace(claims.Role)) {
	case "GURU":
		_ = h.db.Table("learning_subjects").
			Select("id").
			Where("teacher_id = ? AND school_id = ?", claims.UserID, claims.SchoolID).
			Scan(&subjectIDs).Error
	case "SISWA":
		_ = h.db.Raw(`
			SELECT ls.id
			FROM learning_subjects ls
			WHERE ls.school_id = ?
			  AND ls.class_id = (SELECT class_id FROM users WHERE id = ? LIMIT 1)
		`, claims.SchoolID, claims.UserID).Scan(&subjectIDs).Error
	default:
		// Keep an empty subscription set for roles that do not use live chat.
	}

	allowed := make(map[uint]struct{}, len(subjectIDs))
	for _, id := range subjectIDs {
		if id == 0 {
			continue
		}
		allowed[id] = struct{}{}
	}

	h.setSubjectsCache(cacheKey, allowed)
	return allowed
}

func (h *Hub) getSubjectsFromCache(key string) (map[uint]struct{}, bool) {
	if h == nil || key == "" {
		return nil, false
	}

	h.cacheMu.RLock()
	entry, ok := h.subjectsCache[key]
	h.cacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			h.cacheMu.Lock()
			delete(h.subjectsCache, key)
			h.cacheMu.Unlock()
		}
		return nil, false
	}

	out := make(map[uint]struct{}, len(entry.subjects))
	for id := range entry.subjects {
		out[id] = struct{}{}
	}
	return out, true
}

func (h *Hub) setSubjectsCache(key string, subjects map[uint]struct{}) {
	if h == nil || key == "" {
		return
	}

	cp := make(map[uint]struct{}, len(subjects))
	for id := range subjects {
		cp[id] = struct{}{}
	}

	h.cacheMu.Lock()
	h.subjectsCache[key] = subjectCacheEntry{
		subjects:  cp,
		expiresAt: time.Now().Add(h.subjectsCacheTTL),
	}
	h.cacheMu.Unlock()
}

func (h *Hub) extractClaims(r *http.Request) (*AuthClaims, error) {
	token := extractTokenFromRequest(r)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %T", token.Method)
		}

		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || parsedToken == nil || !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &AuthClaims{
		UserID:   uint(asFloat64(claims["id"])),
		SchoolID: uint(asFloat64(claims["schoolId"])),
		Role:     fmt.Sprint(claims["role"]),
	}, nil
}

func (h *Hub) extractClaimsFromFiber(c *fiber.Ctx) (*AuthClaims, error) {
	token := extractTokenFromFiber(c)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %T", token.Method)
		}

		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || parsedToken == nil || !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &AuthClaims{
		UserID:   uint(asFloat64(claims["id"])),
		SchoolID: uint(asFloat64(claims["schoolId"])),
		Role:     fmt.Sprint(claims["role"]),
	}, nil
}

func extractTokenFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}

	for _, key := range []string{"token", "authToken", "authorization"} {
		token := strings.TrimSpace(r.URL.Query().Get(key))
		if token == "" {
			continue
		}

		token = strings.TrimPrefix(token, "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")
		if token != "" {
			return token
		}
	}

	return ""
}

func extractTokenFromFiber(c *fiber.Ctx) string {
	if c == nil {
		return ""
	}

	for _, key := range []string{"token", "authToken", "authorization"} {
		token := strings.TrimSpace(c.Query(key))
		if token == "" {
			continue
		}

		token = strings.TrimPrefix(token, "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")
		if token != "" {
			return token
		}
	}

	return ""
}

func allowedOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}

	allowed := map[string]struct{}{
		"https://school-system.my.id": {},
		"https://alentest.my.id":      {},
		"http://localhost:8080":       {},
		"http://localhost:5173":       {},
		"http://127.0.0.1:8080":       {},
		"http://127.0.0.1:5173":       {},
	}
	if _, ok := allowed[origin]; ok {
		return origin
	}

	return ""
}

func writeSSE(w http.ResponseWriter, eventName string, data any) error {
	payload, err := normalizeSSEPayload(data)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}

	for _, line := range strings.Split(string(payload), "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}

	_, err = fmt.Fprint(w, "\n")
	return err
}

func writeSSEToWriter(w *bufio.Writer, eventName string, data any) error {
	payload, err := normalizeSSEPayload(data)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}

	for _, line := range strings.Split(string(payload), "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}

	_, err = fmt.Fprint(w, "\n")
	return err
}

func normalizeSSEPayload(data any) ([]byte, error) {
	switch value := data.(type) {
	case nil:
		return []byte("null"), nil
	case []byte:
		return value, nil
	case json.RawMessage:
		return value, nil
	case string:
		return []byte(value), nil
	default:
		return json.Marshal(value)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"success":false,"message":%q}`, message)
}

func firstInt(values []any) int {
	if len(values) == 0 {
		return 0
	}

	switch value := values[0].(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}
		return n
	default:
		n, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		if err != nil {
			return 0
		}
		return n
	}
}

func asFloat64(v any) float64 {
	switch value := v.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int8:
		return float64(value)
	case int16:
		return float64(value)
	case int32:
		return float64(value)
	case int64:
		return float64(value)
	case uint:
		return float64(value)
	case uint8:
		return float64(value)
	case uint16:
		return float64(value)
	case uint32:
		return float64(value)
	case uint64:
		return float64(value)
	default:
		n, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		if err != nil {
			return 0
		}
		return n
	}
}
