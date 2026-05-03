package controllers

import (
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) RegisterUser(c *fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		SchoolID *uint  `json:"school_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
	user := models.User{Username: body.Username, Password: string(hash), Role: body.Role, SchoolID: body.SchoolID}
	if err := a.DB.Create(&user).Error; err != nil {
		return utils.Error(c, 500, "Registration failed", err.Error())
	}
	return utils.Success(c, 201, "User registered successfully", fiber.Map{
		"id": user.ID, "username": user.Username, "role": user.Role, "school_id": user.SchoolID,
	})
}

func (a *AppContext) Login(c *fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = c.BodyParser(&body)

	var user models.User
	if err := a.DB.Where("username = ?", body.Username).First(&user).Error; err != nil {
		return utils.Error(c, 404, "User not found")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password)) != nil {
		return utils.Error(c, 401, "Invalid Password")
	}

	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": user.ID, "role": user.Role, "schoolId": user.SchoolID, "exp": time.Now().Add(24 * time.Hour).Unix(),
	}).SignedString([]byte(os.Getenv("JWT_SECRET")))

	var school models.School
	var schoolName interface{} = nil
	if user.SchoolID != nil {
		_ = a.DB.Where("id = ?", *user.SchoolID).First(&school).Error
		schoolName = school.Name
	}

	return utils.Success(c, 200, "Login successful", fiber.Map{
		"role": user.Role, "username": user.Username, "school_id": user.SchoolID, "school_name": schoolName, "profile_image": user.ProfileImage, "token": token,
	})
}

func (a *AppContext) RegisterStudent(c *fiber.Ctx) error { return a.registerScopedUser(c, true) }
func (a *AppContext) RegisterUserSchool(c *fiber.Ctx) error {
	return a.registerScopedUser(c, false)
}

func (a *AppContext) registerScopedUser(c *fiber.Ctx, asStudent bool) error {
	var body map[string]interface{}
	_ = c.BodyParser(&body)

	schoolID := c.Locals("schoolID").(uint)
	role := utils.ToString(body["role"])
	if asStudent {
		role = "SISWA"
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(utils.ToString(body["password"])), 8)
	user := models.User{
		Username:    utils.ToString(body["username"]),
		Password:    string(hash),
		Role:        role,
		SchoolID:    &schoolID,
		ParentEmail: utils.StringPtr(body["parent_email"]),
		PhoneNumber: utils.StringPtr(body["phone_number"]),
	}
	if asStudent {
		classID := uint(utils.ToInt(utils.ToString(body["class_id"]), 0))
		user.ClassID = &classID
	}

	if err := a.DB.Create(&user).Error; err != nil {
		return utils.Error(c, 500, "Registration failed", err.Error())
	}
	return utils.Success(c, 201, "User registered successfully", user)
}

func (a *AppContext) GetUserSchoolList(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	role := c.Query("role")
	var users []map[string]interface{}
	q := a.DB.Table("users").Select("id, username, role, school_id, parent_email, phone_number, profile_image").Where("school_id = ?", schoolID)
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if err := q.Order("username asc").Scan(&users).Error; err != nil {
		return utils.Error(c, 500, "Failed Get User School", err.Error())
	}
	return utils.Success(c, 200, "Success Get User School", users)
}

func (a *AppContext) UpdateUserSchool(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Username    *string `json:"username"`
		Password    *string `json:"password"`
		Role        *string `json:"role"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	_ = c.BodyParser(&body)
	var current models.User
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "User school not found")
	}
	if current.Role != "ADMIN" && current.Role != "GURU" {
		return utils.Error(c, 400, "Only school admin and teacher can be updated here")
	}
	nextUsername := current.Username
	if body.Username != nil {
		nextUsername = *body.Username
	}
	nextRole := current.Role
	if body.Role != nil {
		nextRole = *body.Role
	}
	updates := map[string]interface{}{
		"username":     nextUsername,
		"role":         nextRole,
		"parent_email": coalesceStrPtr(body.ParentEmail, current.ParentEmail),
		"phone_number": coalesceStrPtr(body.PhoneNumber, current.PhoneNumber),
	}
	if body.Password != nil && *body.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(*body.Password), 8)
		updates["password"] = string(hash)
	}
	a.DB.Table("users").Where("id = ? AND school_id = ?", id, schoolID).Updates(updates)
	var updated map[string]interface{}
	a.DB.Table("users").Select("id, username, role, school_id, parent_email, phone_number, profile_image").Where("id = ?", id).Scan(&updated)
	return utils.Success(c, 200, "User school updated successfully", updated)
}
func (a *AppContext) DeleteUserSchool(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var current models.User
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "User school not found")
	}
	if current.Role != "ADMIN" && current.Role != "GURU" {
		return utils.Error(c, 400, "Only school admin and teacher can be deleted here")
	}
	a.DB.Exec(`DELETE FROM users WHERE id = ? AND school_id = ?`, id, schoolID)
	return utils.Success(c, 200, fmt.Sprintf(`User "%s" berhasil dihapus`, current.Username), nil)
}

func (a *AppContext) GetMyProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var profile struct {
		ID           uint    `json:"id"`
		Username     string  `json:"username"`
		Role         string  `json:"role"`
		SchoolID     *uint   `json:"school_id"`
		ParentEmail  *string `json:"parent_email"`
		PhoneNumber  *string `json:"phone_number"`
		ProfileImage *string `json:"profile_image"`
		SchoolName   *string `json:"school_name"`
	}
	err := a.DB.Table("users u").
		Select("u.id, u.username, u.role, u.school_id, u.parent_email, u.phone_number, u.profile_image, s.name as school_name").
		Joins("left join schools s on s.id = u.school_id").
		Where("u.id = ?", userID).
		Scan(&profile).Error
	if err != nil {
		return utils.Error(c, 500, "Failed Get Profile", err.Error())
	}
	return utils.Success(c, 200, "Success Get Profile", profile)
}

func coalesceStrPtr(v *string, fallback *string) interface{} {
	if v != nil {
		return *v
	}
	if fallback == nil {
		return nil
	}
	return *fallback
}
