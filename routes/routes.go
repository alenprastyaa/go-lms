package routes

import (
	"lms/controllers"
	"lms/middlewares"
	"lms/realtime"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func Register(app *fiber.App, db *gorm.DB, hub *realtime.Hub) {
	ctx := &controllers.AppContext{DB: db, Realtime: hub}
	api := app.Group("/api")

	registerAuth(api, ctx)
	registerSchool(api, ctx)
	registerClass(api, ctx)
	registerStudent(api, ctx)
	registerPublic(api, ctx)
	registerReceipt(api, ctx)
	registerAdmin(api, ctx)
	registerAcademic(api, ctx)
	registerLearningAdmin(api, ctx)
	registerPrivateChat(api, ctx)
	registerGuru(api, ctx)
	registerSiswa(api, ctx)
	registerInventory(api, ctx)
	registerKoperasi(api, ctx)
	registerNotifications(api, ctx)
}

func registerAuth(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/auth")
	r.Post("/register", ctx.RegisterUser)
	r.Post("/login", ctx.Login)

	p := r.Use(middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	p.Post("/register/student", ctx.RegisterStudent)
	p.Get("/student/template", middlewares.RoleAllowed("ADMIN"), ctx.DownloadStudentTemplate)
	p.Get("/student/accounts", middlewares.RoleAllowed("ADMIN"), ctx.DownloadStudentAccountsByClass)
	p.Post("/register/student/import", middlewares.RoleAllowed("ADMIN"), ctx.ImportStudents)
	p.Post("/register/user-school", middlewares.RoleAllowed("ADMIN"), ctx.RegisterUserSchool)
	p.Get("/user-school/template", middlewares.RoleAllowed("ADMIN"), ctx.DownloadUserSchoolTeacherTemplate)
	p.Post("/register/user-school/import", middlewares.RoleAllowed("ADMIN"), ctx.ImportUserSchoolTeachers)
	p.Get("/user-school", middlewares.RoleAllowed("ADMIN"), ctx.GetUserSchoolList)
	p.Get("/teachers", ctx.GetSchoolTeacherList)
	p.Get("/teachers/student", middlewares.RoleAllowed("SISWA", "SARPRAS"), ctx.GetStudentTeacherList)
	p.Get("/schedule-options", middlewares.RoleAllowed("SISWA", "SARPRAS"), ctx.GetStudentScheduleOptions)
	p.Put("/user-school/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateUserSchool)
	p.Post("/user-school/:id/reset-password", middlewares.RoleAllowed("ADMIN"), ctx.ResetUserSchoolPassword)
	p.Delete("/user-school/:id", middlewares.RoleAllowed("ADMIN"), ctx.DeleteUserSchool)
	p.Get("/super-admin/admin-users", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.GetSuperAdminAdminUsers)
	p.Post("/super-admin/admin-users", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.CreateSuperAdminAdminUser)
	p.Put("/super-admin/admin-users/:id", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.UpdateSuperAdminAdminUser)
	p.Delete("/super-admin/admin-users/:id", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.DeleteSuperAdminAdminUser)
	p.Get("/profile", ctx.GetMyProfile)
	p.Put("/profile", ctx.UpdateMyProfile)
}

func registerSchool(api fiber.Router, ctx *controllers.AppContext) {
	current := api.Group("/school/current", middlewares.Auth(ctx.DB), middlewares.ExtractClaims(), middlewares.RoleAllowed("ADMIN"))
	current.Put("/", ctx.UpdateCurrentSchool)
	current.Put("/branding", ctx.UpdateCurrentSchoolBranding)

	r := api.Group("/school", middlewares.Auth(ctx.DB), middlewares.ExtractClaims(), middlewares.RoleAllowed("SUPER_ADMIN"))
	r.Post("/", ctx.CreateSchool)
	r.Get("/", ctx.GetSchools)
	r.Put("/:id", ctx.UpdateSchool)
	r.Put("/:id/modules", ctx.UpdateSchoolModules)
	r.Delete("/:id", ctx.DeleteSchool)
	r.Get("/:schoolId/billing", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.GetSchoolBillingSettings)
	r.Put("/:schoolId/billing", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.UpsertSchoolBillingSettings)
	r.Delete("/:schoolId/billing", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.DeleteSchoolBillingSettings)
	r.Post("/:schoolId/billing/invoices", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.CreateSchoolInvoice)
	r.Get("/:schoolId/billing/invoices", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.GetSchoolInvoices)
	r.Delete("/:schoolId/billing/invoices/:invoiceId", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.DeleteSchoolInvoice)
}

func registerClass(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/class", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	r.Post("/", ctx.CreateClass)
	r.Get("/", ctx.GetClasses)
	r.Put("/:id", ctx.UpdateClass)
	r.Delete("/:id", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.DeleteClass)
	r.Get("/my/homeroom", middlewares.RoleAllowed("GURU"), ctx.GetMyClass)
}

func registerStudent(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/student", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	r.Get("/", ctx.GetStudents)
	r.Get("/:id/detail", middlewares.RoleAllowed("ADMIN"), ctx.GetStudentDetail)
	r.Post("/:id/reset-password", middlewares.RoleAllowed("ADMIN"), ctx.ResetStudentPassword)
	r.Post("/promotions", middlewares.RoleAllowed("ADMIN"), ctx.PromoteStudents)
	r.Get("/my-class", middlewares.RoleAllowed("GURU"), ctx.GetMyClassStudents)
	r.Get("/:id/attendance", middlewares.RoleAllowed("GURU"), ctx.GetStudentAttendanceForTeacher)
	r.Get("/:id/receipt", middlewares.RoleAllowed("GURU"), ctx.GetStudentReceiptForTeacher)
	r.Get("/:id/class-history", middlewares.RoleAllowed("ADMIN", "GURU"), ctx.GetStudentClassHistory)
	r.Put("/:id", ctx.EditStudent)
	r.Delete("/:id", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.DeleteStudent)
}

func registerPublic(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/public")
	r.Get("/registration-options", ctx.GetPublicRegistrationOptions)
	r.Post("/student-registration", ctx.RegisterStudentPublic)
	r.Get("/check-username", ctx.CheckUsernameAvailability)
}

func registerReceipt(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/receipt", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	r.Post("/", ctx.CreateReceipt)
	r.Get("/", ctx.GetReceipt)
	r.Get("/:id", ctx.GetReceiptByID)
	r.Put("/:id", ctx.UpdateReceipt)
	r.Delete("/:id", ctx.DeleteReceipt)
}

func registerPrivateChat(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/private-chat", middlewares.Auth(ctx.DB), middlewares.ExtractClaims(), middlewares.RoleAllowed("ADMIN", "KOPERASI", "GURU", "SISWA"))
	r.Get("/summary", ctx.GetPrivateChatSummary)
	r.Get("/contacts", ctx.SearchPrivateChatContacts)
	r.Get("/:peerUserId/messages", ctx.GetPrivateChatMessages)
	r.Post("/:peerUserId/messages", ctx.CreatePrivateChatMessage)
	r.Put("/:peerUserId/messages/:messageId", ctx.UpdatePrivateChatMessage)
	r.Post("/:peerUserId/read", ctx.MarkPrivateChatAsRead)
}

func registerGuru(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	d.Get("/guru", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN", "GURU"), ctx.GetGuruDashboard)

	l := api.Group("/learning", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	l.Get("/subjects/teacher", middlewares.RoleAllowed("GURU"), ctx.GetTeacherSubjects)
	l.Get("/subjects/:subjectId/materials", middlewares.RoleAllowed("GURU", "SISWA"), ctx.GetSubjectMaterials)
	l.Post("/materials", middlewares.RoleAllowed("GURU"), ctx.CreateLearningMaterial)
	l.Put("/materials/:materialId", middlewares.RoleAllowed("GURU"), ctx.UpdateLearningMaterial)
	l.Delete("/materials/:materialId", middlewares.RoleAllowed("GURU"), ctx.DeleteLearningMaterial)
	l.Post("/subjects/:subjectId/materials/generate-ai-pptx", middlewares.RoleAllowed("GURU"), ctx.GenerateLearningMaterialPptWithAI)
	l.Post("/subjects/:subjectId/materials/publish-ai-pptx", middlewares.RoleAllowed("GURU"), ctx.PublishLearningMaterialPptWithAI)
	l.Put("/assignments/:assignmentId", middlewares.RoleAllowed("GURU"), ctx.UpdateLearningAssignmentByTeacher)
	l.Delete("/assignments/:assignmentId", middlewares.RoleAllowed("GURU"), ctx.DeleteLearningAssignmentByTeacher)
	l.Get("/assignments/:assignmentId/submissions", middlewares.RoleAllowed("GURU", "ADMIN"), ctx.GetAssignmentSubmissionsForTeacher)
	l.Get("/subjects/:subjectId/gradebook", middlewares.RoleAllowed("GURU"), ctx.GetSubjectGradebookForTeacher)
	l.Post("/submissions/:submissionId/grade", middlewares.RoleAllowed("GURU"), ctx.GradeLearningSubmission)
	l.Post("/question-bank", middlewares.RoleAllowed("GURU"), ctx.CreateLearningQuestionBankItem)
	l.Put("/question-bank/:id", middlewares.RoleAllowed("GURU"), ctx.UpdateLearningQuestionBankItem)
	l.Delete("/question-bank/:id", middlewares.RoleAllowed("GURU"), ctx.DeleteLearningQuestionBankItem)
	l.Get("/subjects/:subjectId/question-bank", middlewares.RoleAllowed("GURU", "ADMIN"), ctx.GetLearningQuestionBank)
	l.Get("/subjects/:subjectId/question-bank/template", middlewares.RoleAllowed("GURU"), ctx.DownloadLearningQuestionBankTemplate)
	l.Post("/subjects/:subjectId/question-bank/import", middlewares.RoleAllowed("GURU"), ctx.ImportLearningQuestionBankFromDocument)
	l.Post("/subjects/:subjectId/question-bank/generate-ai", middlewares.RoleAllowed("GURU"), ctx.GenerateLearningQuestionBankWithAI)
	l.Post("/subjects/:subjectId/question-bank/save-generated-ai", middlewares.RoleAllowed("GURU"), ctx.SaveGeneratedLearningQuestionBankItems)
	l.Post("/subjects/:subjectId/question-bank/bulk-delete", middlewares.RoleAllowed("GURU"), ctx.DeleteLearningQuestionBankItemsBulk)
	l.Get("/chat/summary", middlewares.RoleAllowed("GURU", "SISWA"), ctx.GetLearningChatSummary)
	l.Get("/subjects/:subjectId/chat", middlewares.RoleAllowed("GURU", "SISWA"), ctx.GetSubjectChatMessages)
	l.Get("/subjects/:subjectId/chat/online", middlewares.RoleAllowed("GURU", "SISWA"), ctx.GetSubjectOnlineUsers)
	l.Post("/subjects/:subjectId/chat", middlewares.RoleAllowed("GURU", "SISWA"), ctx.CreateSubjectChatMessage)
	l.Put("/subjects/:subjectId/chat/:messageId", middlewares.RoleAllowed("GURU", "SISWA"), ctx.UpdateSubjectChatMessage)
	l.Post("/subjects/:subjectId/chat/read", middlewares.RoleAllowed("GURU", "SISWA"), ctx.MarkSubjectChatAsRead)
	l.Post("/subjects/:subjectId/chat/typing", middlewares.RoleAllowed("GURU", "SISWA"), ctx.BroadcastSubjectTyping)
	l.Put("/subjects/:subjectId/chat-icon", middlewares.RoleAllowed("GURU"), ctx.UpdateLearningSubjectChatIconByTeacher)
	l.Post("/assignments/:assignmentId/exam-package", middlewares.RoleAllowed("GURU"), ctx.SubmitExamPackageByTeacher)
	l.Get("/assignments/:assignmentId/overview", middlewares.RoleAllowed("GURU", "ADMIN"), ctx.GetQuizAssignmentOverviewForTeacher)
	l.Get("/subjects/:subjectId/final-report", middlewares.RoleAllowed("GURU"), ctx.GetFinalGradeReportForTeacher)
}

func registerSiswa(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	d.Get("/siswa", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN", "GURU", "SISWA"), ctx.GetSiswaDashboard)

	at := api.Group("/attendance", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	at.Post("/", ctx.CheckIn)
	at.Post("/checkout", ctx.CheckOut)
	at.Get("/", ctx.GetListAttendance)

	l := api.Group("/learning", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	l.Get("/subjects/student", middlewares.RoleAllowed("SISWA"), ctx.GetStudentSubjects)
	l.Get("/grades/student", middlewares.RoleAllowed("SISWA"), ctx.GetStudentGrades)
	l.Post("/assignments/:assignmentId/start", middlewares.RoleAllowed("SISWA"), ctx.StartLearningQuizAttempt)
	l.Post("/assignments/:assignmentId/submit", middlewares.RoleAllowed("SISWA"), ctx.SubmitLearningAssignment)
	l.Post("/assignments/:assignmentId/violations", middlewares.RoleAllowed("SISWA"), ctx.RecordQuizViolation)
}

func registerInventory(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	d.Get("/sarpras", middlewares.RoleAllowed("ADMIN", "SARPRAS"), ctx.GetSarprasDashboard)

	r := api.Group("/inventory", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	r.Get("/overview", middlewares.RoleAllowed("ADMIN", "SARPRAS"), ctx.GetSarprasDashboard)
	r.Get("/items", middlewares.RoleAllowed("ADMIN", "SARPRAS", "SISWA"), ctx.GetInventoryItems)
	r.Post("/items", middlewares.RoleAllowed("ADMIN", "SARPRAS"), ctx.CreateInventoryItem)
	r.Put("/items/:id", middlewares.RoleAllowed("ADMIN", "SARPRAS"), ctx.UpdateInventoryItem)
	r.Delete("/items/:id", middlewares.RoleAllowed("ADMIN", "SARPRAS"), ctx.DeleteInventoryItem)
	r.Get("/loans", middlewares.RoleAllowed("ADMIN", "SARPRAS", "SISWA"), ctx.GetInventoryLoans)
	r.Get("/active-loans", middlewares.RoleAllowed("ADMIN", "SARPRAS", "SISWA"), ctx.GetInventoryActiveLoans)
	r.Post("/loans", middlewares.RoleAllowed("SISWA"), ctx.CreateInventoryLoan)
	r.Post("/loans/:id/return", middlewares.RoleAllowed("ADMIN", "SARPRAS", "SISWA"), ctx.ReturnInventoryLoan)
}

func registerKoperasi(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	d.Get("/koperasi", middlewares.RoleAllowed("ADMIN", "KOPERASI", "GURU", "SARPRAS", "SISWA"), ctx.GetKoperasiDashboard)

	r := api.Group("/koperasi", middlewares.Auth(ctx.DB), middlewares.ExtractClaims(), middlewares.RoleAllowed("ADMIN", "KOPERASI", "GURU", "SARPRAS", "SISWA"))
	r.Get("/dashboard", ctx.GetKoperasiDashboard)
	r.Get("/products", ctx.GetKoperasiProducts)
	r.Post("/products", middlewares.RoleAllowed("ADMIN", "KOPERASI"), ctx.CreateKoperasiProduct)
	r.Put("/products/:id", middlewares.RoleAllowed("ADMIN", "KOPERASI"), ctx.UpdateKoperasiProduct)
	r.Delete("/products/:id", middlewares.RoleAllowed("ADMIN", "KOPERASI"), ctx.DeleteKoperasiProduct)
	r.Get("/cart", ctx.GetKoperasiCart)
	r.Put("/cart/items", ctx.UpsertKoperasiCartItem)
	r.Delete("/cart/items/:productId", ctx.DeleteKoperasiCartItem)
	r.Delete("/cart", ctx.ClearKoperasiCart)
	r.Get("/orders", ctx.GetKoperasiOrders)
	r.Get("/orders/:id", ctx.GetKoperasiOrderByID)
	r.Post("/orders", ctx.CreateKoperasiOrder)
	r.Post("/orders/:id/simulate-payment", ctx.SimulateKoperasiSandboxPayment)
	r.Post("/orders/:id/reissue-payment", ctx.ReissueKoperasiPayment)
	r.Put("/orders/:id/status", ctx.UpdateKoperasiOrderStatus)
	r.Get("/reports/summary", middlewares.RoleAllowed("ADMIN", "KOPERASI"), ctx.GetKoperasiReportSummary)
}

func registerNotifications(api fiber.Router, ctx *controllers.AppContext) {
	api.Get("/notifications/vapid-public-key", ctx.GetVapidPublicKey)
	r := api.Group("/notifications", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	r.Post("/push-subscriptions", ctx.UpsertPushSubscription)
	r.Delete("/push-subscriptions", ctx.DeletePushSubscription)
}

func registerAdmin(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	d.Get("/superadmin", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.GetSuperAdminDashboard)
	d.Get("/admin", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.GetAdminDashboard)

	announcements := api.Group("/announcements", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	announcements.Get("/dashboard", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN", "KOPERASI", "SARPRAS", "GURU", "SISWA"), ctx.GetDashboardAnnouncements)

	adminAnnouncements := api.Group("/announcements", middlewares.Auth(ctx.DB), middlewares.ExtractClaims(), middlewares.RoleAllowed("ADMIN"))
	adminAnnouncements.Get("/", ctx.GetSchoolAnnouncements)
	adminAnnouncements.Post("/", ctx.CreateSchoolAnnouncement)
	adminAnnouncements.Put("/:id", ctx.UpdateSchoolAnnouncement)
	adminAnnouncements.Post("/:id/publish", ctx.PublishSchoolAnnouncement)
	adminAnnouncements.Post("/:id/toggle", ctx.ToggleSchoolAnnouncementStatus)
	adminAnnouncements.Delete("/:id", ctx.DeleteSchoolAnnouncement)

	a := api.Group("/admin-settings", middlewares.Auth(ctx.DB), middlewares.ExtractClaims(), middlewares.RoleAllowed("ADMIN"))
	a.Get("/summary", ctx.GetAdminSettingsSummary)
	a.Get("/public-registration-link", ctx.GetPublicStudentRegistrationLink)
	a.Post("/load-test", ctx.RunAdminLoadTest)
	a.Post("/load-test-login", ctx.RunAdminLoginLoadTest)
	a.Post("/reset", ctx.ResetAdminScope)

	api.Post("/billing/xendit/webhook", ctx.XenditWebhook)
	api.Post("/koperasi/xendit/webhook", ctx.XenditKoperasiWebhook)

	b := api.Group("/billing", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	b.Get("/current", middlewares.RoleAllowed("ADMIN"), ctx.GetCurrentSchoolBilling)
	b.Get("/current/invoices", middlewares.RoleAllowed("ADMIN"), ctx.GetCurrentSchoolInvoices)
	b.Post("/current/invoices/:invoiceId/pay", middlewares.RoleAllowed("ADMIN"), ctx.CreateMidtransPaymentForInvoice)
	b.Post("/current/invoices/:invoiceId/sync-xendit", middlewares.RoleAllowed("ADMIN"), ctx.SyncXenditInvoiceStatus)
	b.Post("/current/invoices/reference/:referenceId/sync-xendit", middlewares.RoleAllowed("ADMIN"), ctx.SyncXenditInvoiceStatusByReference)
	b.Post("/midtrans/notification", ctx.MidtransNotification)

	at := api.Group("/attendance", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	at.Post("/report/homeroom-email", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.SendHomeroomAttendanceReport)
	at.Post("/report/homeroom-whatsapp", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.SendHomeroomAttendanceWhatsAppReport)
}

func registerAcademic(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/academic-periods", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	r.Get("/", middlewares.RoleAllowed("ADMIN", "GURU"), ctx.GetAcademicPeriods)
	r.Post("/years", middlewares.RoleAllowed("ADMIN"), ctx.CreateAcademicYear)
	r.Put("/years/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateAcademicYear)
	r.Post("/years/:id/activate", middlewares.RoleAllowed("ADMIN"), ctx.ActivateAcademicYear)
	r.Post("/semesters", middlewares.RoleAllowed("ADMIN"), ctx.CreateSemester)
	r.Put("/semesters/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateSemester)
	r.Post("/semesters/:id/activate", middlewares.RoleAllowed("ADMIN"), ctx.ActivateSemester)
}

func registerLearningAdmin(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/learning", middlewares.Auth(ctx.DB), middlewares.ExtractClaims())
	r.Post("/system-chatbot", middlewares.RoleAllowed("ADMIN", "GURU"), ctx.AskSystemChatbot)
	r.Get("/curriculum/overview", middlewares.RoleAllowed("ADMIN"), ctx.GetCurriculumOverview)
	r.Post("/curriculum/subjects", middlewares.RoleAllowed("ADMIN"), ctx.CreateCurriculumSubject)
	r.Put("/curriculum/subjects/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateCurriculumSubject)
	r.Delete("/curriculum/subjects/:id", middlewares.RoleAllowed("ADMIN"), ctx.DeleteCurriculumSubject)
	r.Post("/curriculum/teacher-loads", middlewares.RoleAllowed("ADMIN"), ctx.CreateCurriculumTeacherLoad)
	r.Put("/curriculum/teacher-loads/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateCurriculumTeacherLoad)
	r.Delete("/curriculum/teacher-loads/:id", middlewares.RoleAllowed("ADMIN"), ctx.DeleteCurriculumTeacherLoad)
	r.Post("/curriculum/class-distributions", middlewares.RoleAllowed("ADMIN"), ctx.CreateCurriculumClassDistribution)
	r.Put("/curriculum/class-distributions/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateCurriculumClassDistribution)
	r.Delete("/curriculum/class-distributions/:id", middlewares.RoleAllowed("ADMIN"), ctx.DeleteCurriculumClassDistribution)
	r.Post("/curriculum/schedule-slots", middlewares.RoleAllowed("ADMIN"), ctx.CreateCurriculumScheduleSlot)
	r.Post("/curriculum/schedule-slots/bulk", middlewares.RoleAllowed("ADMIN"), ctx.BulkCreateCurriculumScheduleSlots)
	r.Delete("/curriculum/schedule-slots/bulk", middlewares.RoleAllowed("ADMIN"), ctx.BulkDeleteCurriculumScheduleSlots)
	r.Put("/curriculum/schedule-slots/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateCurriculumScheduleSlot)
	r.Delete("/curriculum/schedule-slots/:id", middlewares.RoleAllowed("ADMIN"), ctx.DeleteCurriculumScheduleSlot)
	r.Post("/curriculum/generate", middlewares.RoleAllowed("ADMIN"), ctx.GenerateCurriculumSchedule)
	r.Get("/subjects/admin", middlewares.RoleAllowed("ADMIN"), ctx.GetAdminSubjects)
	r.Post("/subjects", middlewares.RoleAllowed("ADMIN"), ctx.CreateLearningSubject)
	r.Put("/subjects/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateLearningSubject)
	r.Delete("/subjects/:id", middlewares.RoleAllowed("ADMIN"), ctx.DeleteLearningSubject)
	r.Get("/subjects/:subjectId/assignments", middlewares.RoleAllowed("ADMIN", "GURU", "SISWA"), ctx.GetSubjectAssignments)
	r.Post("/assignments", middlewares.RoleAllowed("GURU", "ADMIN"), ctx.CreateLearningAssignment)
	r.Put("/assignments/:assignmentId/exam-admin", middlewares.RoleAllowed("ADMIN"), ctx.UpdateExamRequestByAdmin)
	r.Delete("/assignments/:assignmentId/exam-admin", middlewares.RoleAllowed("ADMIN"), ctx.DeleteExamRequestByAdmin)
	r.Post("/assignments/:assignmentId/publish", middlewares.RoleAllowed("ADMIN"), ctx.PublishExamByAdmin)
	r.Post("/submissions/:submissionId/exam-access-code", middlewares.RoleAllowed("ADMIN"), ctx.GenerateStudentExamAccessCodeByAdmin)
}

// Saya sedang test CI CD
