package routes

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/controllers"
	"lms/middlewares"
	"lms/realtime"
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
	registerGuru(api, ctx)
	registerSiswa(api, ctx)
}

func registerAuth(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/auth")
	r.Post("/register", ctx.RegisterUser)
	r.Post("/login", ctx.Login)

	p := r.Use(middlewares.Auth(), middlewares.ExtractClaims())
	p.Post("/register/student", ctx.RegisterStudent)
	p.Post("/register/user-school", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.RegisterUserSchool)
	p.Get("/user-school", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.GetUserSchoolList)
	p.Put("/user-school/:id", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.UpdateUserSchool)
	p.Delete("/user-school/:id", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.DeleteUserSchool)
	p.Get("/profile", ctx.GetMyProfile)
	p.Put("/profile", ctx.UpdateMyProfile)
}

func registerSchool(api fiber.Router, ctx *controllers.AppContext) {
	api.Post("/school", ctx.CreateSchool)
}

func registerClass(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/class", middlewares.Auth(), middlewares.ExtractClaims())
	r.Post("/", ctx.CreateClass)
	r.Get("/", ctx.GetClasses)
	r.Put("/:id", ctx.UpdateClass)
	r.Delete("/:id", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.DeleteClass)
	r.Get("/my/homeroom", middlewares.RoleAllowed("GURU"), ctx.GetMyClass)
}

func registerStudent(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/student", middlewares.Auth(), middlewares.ExtractClaims())
	r.Get("/", ctx.GetStudents)
	r.Get("/my-class", middlewares.RoleAllowed("GURU"), ctx.GetMyClassStudents)
	r.Get("/:id/attendance", middlewares.RoleAllowed("GURU"), ctx.GetStudentAttendanceForTeacher)
	r.Get("/:id/receipt", middlewares.RoleAllowed("GURU"), ctx.GetStudentReceiptForTeacher)
	r.Put("/:id", ctx.EditStudent)
	r.Delete("/:id", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.DeleteStudent)
}

func registerPublic(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/public")
	r.Get("/registration-options", ctx.GetPublicRegistrationOptions)
	r.Post("/student-registration", ctx.RegisterStudentPublic)
}

func registerReceipt(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/receipt", middlewares.Auth(), middlewares.ExtractClaims())
	r.Post("/", ctx.CreateReceipt)
	r.Get("/", ctx.GetReceipt)
	r.Get("/:id", ctx.GetReceiptByID)
	r.Put("/:id", ctx.UpdateReceipt)
	r.Delete("/:id", ctx.DeleteReceipt)
}

func registerGuru(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(), middlewares.ExtractClaims())
	d.Get("/guru", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN", "GURU"), ctx.GetGuruDashboard)

	l := api.Group("/learning", middlewares.Auth(), middlewares.ExtractClaims())
	l.Get("/subjects/teacher", middlewares.RoleAllowed("GURU"), ctx.GetTeacherSubjects)
	l.Get("/subjects/:subjectId/materials", middlewares.RoleAllowed("GURU", "SISWA"), ctx.GetSubjectMaterials)
	l.Post("/materials", middlewares.RoleAllowed("GURU"), ctx.CreateLearningMaterial)
	l.Put("/materials/:materialId", middlewares.RoleAllowed("GURU"), ctx.UpdateLearningMaterial)
	l.Delete("/materials/:materialId", middlewares.RoleAllowed("GURU"), ctx.DeleteLearningMaterial)
	l.Post("/subjects/:subjectId/materials/generate-ai-pptx", middlewares.RoleAllowed("GURU"), ctx.GenerateLearningMaterialPptWithAI)
	l.Post("/subjects/:subjectId/materials/publish-ai-pptx", middlewares.RoleAllowed("GURU"), ctx.PublishLearningMaterialPptWithAI)
	l.Put("/assignments/:assignmentId", middlewares.RoleAllowed("GURU"), ctx.UpdateLearningAssignmentByTeacher)
	l.Delete("/assignments/:assignmentId", middlewares.RoleAllowed("GURU"), ctx.DeleteLearningAssignmentByTeacher)
	l.Get("/assignments/:assignmentId/submissions", middlewares.RoleAllowed("GURU"), ctx.GetAssignmentSubmissionsForTeacher)
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
	l.Post("/subjects/:subjectId/chat/read", middlewares.RoleAllowed("GURU", "SISWA"), ctx.MarkSubjectChatAsRead)
	l.Post("/subjects/:subjectId/chat/typing", middlewares.RoleAllowed("GURU", "SISWA"), ctx.BroadcastSubjectTyping)
	l.Put("/subjects/:subjectId/chat-icon", middlewares.RoleAllowed("GURU"), ctx.UpdateLearningSubjectChatIconByTeacher)
	l.Post("/assignments/:assignmentId/exam-package", middlewares.RoleAllowed("GURU"), ctx.SubmitExamPackageByTeacher)
	l.Get("/assignments/:assignmentId/overview", middlewares.RoleAllowed("GURU"), ctx.GetQuizAssignmentOverviewForTeacher)
	l.Get("/subjects/:subjectId/final-report", middlewares.RoleAllowed("GURU"), ctx.GetFinalGradeReportForTeacher)
}

func registerSiswa(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(), middlewares.ExtractClaims())
	d.Get("/siswa", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN", "GURU", "SISWA"), ctx.GetSiswaDashboard)

	at := api.Group("/attendance", middlewares.Auth(), middlewares.ExtractClaims())
	at.Post("/", ctx.CheckIn)
	at.Post("/checkout", ctx.CheckOut)
	at.Get("/", ctx.GetListAttendance)

	l := api.Group("/learning", middlewares.Auth(), middlewares.ExtractClaims())
	l.Get("/subjects/student", middlewares.RoleAllowed("SISWA"), ctx.GetStudentSubjects)
	l.Get("/grades/student", middlewares.RoleAllowed("SISWA"), ctx.GetStudentGrades)
	l.Post("/assignments/:assignmentId/start", middlewares.RoleAllowed("SISWA"), ctx.StartLearningQuizAttempt)
	l.Post("/assignments/:assignmentId/submit", middlewares.RoleAllowed("SISWA"), ctx.SubmitLearningAssignment)
	l.Post("/assignments/:assignmentId/violations", middlewares.RoleAllowed("SISWA"), ctx.RecordQuizViolation)
}

func registerAdmin(api fiber.Router, ctx *controllers.AppContext) {
	d := api.Group("/dashboard", middlewares.Auth(), middlewares.ExtractClaims())
	d.Get("/superadmin", middlewares.RoleAllowed("SUPER_ADMIN"), ctx.GetSuperAdminDashboard)
	d.Get("/admin", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.GetAdminDashboard)

	a := api.Group("/admin-settings", middlewares.Auth(), middlewares.ExtractClaims(), middlewares.RoleAllowed("ADMIN"))
	a.Get("/summary", ctx.GetAdminSettingsSummary)
	a.Get("/public-registration-link", ctx.GetPublicStudentRegistrationLink)
	a.Post("/reset", ctx.ResetAdminScope)

	at := api.Group("/attendance", middlewares.Auth(), middlewares.ExtractClaims())
	at.Post("/report/homeroom-email", middlewares.RoleAllowed("SUPER_ADMIN", "ADMIN"), ctx.SendHomeroomAttendanceReport)
}

func registerAcademic(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/academic-periods", middlewares.Auth(), middlewares.ExtractClaims())
	r.Get("/", middlewares.RoleAllowed("ADMIN", "GURU"), ctx.GetAcademicPeriods)
	r.Post("/years", middlewares.RoleAllowed("ADMIN"), ctx.CreateAcademicYear)
	r.Put("/years/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateAcademicYear)
	r.Post("/years/:id/activate", middlewares.RoleAllowed("ADMIN"), ctx.ActivateAcademicYear)
	r.Post("/semesters", middlewares.RoleAllowed("ADMIN"), ctx.CreateSemester)
	r.Put("/semesters/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateSemester)
	r.Post("/semesters/:id/activate", middlewares.RoleAllowed("ADMIN"), ctx.ActivateSemester)
}

func registerLearningAdmin(api fiber.Router, ctx *controllers.AppContext) {
	r := api.Group("/learning", middlewares.Auth(), middlewares.ExtractClaims())
	r.Get("/subjects/admin", middlewares.RoleAllowed("ADMIN"), ctx.GetAdminSubjects)
	r.Post("/subjects", middlewares.RoleAllowed("ADMIN"), ctx.CreateLearningSubject)
	r.Put("/subjects/:id", middlewares.RoleAllowed("ADMIN"), ctx.UpdateLearningSubject)
	r.Delete("/subjects/:id", middlewares.RoleAllowed("ADMIN"), ctx.DeleteLearningSubject)
	r.Get("/subjects/:subjectId/assignments", middlewares.RoleAllowed("ADMIN", "GURU", "SISWA"), ctx.GetSubjectAssignments)
	r.Post("/assignments", middlewares.RoleAllowed("GURU", "ADMIN"), ctx.CreateLearningAssignment)
	r.Put("/assignments/:assignmentId/exam-admin", middlewares.RoleAllowed("ADMIN"), ctx.UpdateExamRequestByAdmin)
	r.Delete("/assignments/:assignmentId/exam-admin", middlewares.RoleAllowed("ADMIN"), ctx.DeleteExamRequestByAdmin)
	r.Post("/assignments/:assignmentId/publish", middlewares.RoleAllowed("ADMIN"), ctx.PublishExamByAdmin)
}
