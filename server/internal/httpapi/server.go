package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cyberlife/server/internal/acl"
	"cyberlife/server/internal/admin"
	"cyberlife/server/internal/auth"
	"cyberlife/server/internal/config"
	"cyberlife/server/internal/future"
	"cyberlife/server/internal/history"
	"cyberlife/server/internal/interaction"
	"cyberlife/server/internal/music"
	"cyberlife/server/internal/notification"
	nowservice "cyberlife/server/internal/now"
)

type Server struct {
	cfg          config.Config
	auth         *auth.Service
	admin        *admin.Service
	now          *nowservice.Service
	music        *music.Service
	acl          *acl.Service
	interaction  *interaction.Service
	history      *history.Service
	future       *future.Service
	notification *notification.Service
}

func New(cfg config.Config, authService *auth.Service, adminService *admin.Service, nowService *nowservice.Service, musicService *music.Service, aclService *acl.Service, interactionService *interaction.Service, historyService *history.Service, futureService *future.Service, notificationService *notification.Service) *Server {
	return &Server{cfg: cfg, auth: authService, admin: adminService, now: nowService, music: musicService, acl: aclService, interaction: interactionService, history: historyService, future: futureService, notification: notificationService}
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), s.cors())
	r.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	webDir, _ := filepath.Abs(s.cfg.WebDir)
	r.Static("/assets", filepath.Join(webDir, "assets"))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			fail(c, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		http.ServeFile(c.Writer, c.Request, filepath.Join(webDir, "index.html"))
	})
	r.POST("/api/v1/auth/key-login", s.keyLogin)
	r.POST("/api/v1/admin/auth/login", s.adminLogin)
	r.POST("/api/v1/auth/logout", s.logout)
	protected := r.Group("/api/v1")
	protected.Use(s.requireActor())
	protected.GET("/auth/me", s.me)
	protected.GET("/today", s.visibleToday)
	protected.GET("/history", s.historyRange)
	protected.GET("/notifications", s.listNotifications)
	protected.POST("/notifications/:id/read", s.markNotificationRead)
	protected.GET("/plans", s.listPlans)
	protected.GET("/plans/:id/cover", s.downloadPlanImage("cover"))
	protected.GET("/plans/:id/icon", s.downloadPlanImage("icon"))
	protected.GET("/plans/:id/files/:fileID", s.downloadPlanFile)
	protected.GET("/calendar", s.futureCalendar)
	protected.POST("/comments", s.addVisibleComment)
	protected.GET("/comments", s.listVisibleComments)
	protected.GET("/milestones", s.listVisibleMilestones)
	protected.GET("/attachments/:id", s.downloadAttachment)
	protected.GET("/music/tracks/:id", s.downloadMusicTrack)
	writer := protected.Group("/now")
	writer.Use(s.requireWriter())
	writer.GET("", s.nowToday)
	writer.POST("/plans", s.createPlan)
	writer.PUT("/plans/order", s.reorderPlans)
	writer.POST("/plans/:id/progress", s.setPlanProgress)
	writer.PUT("/plans/:id", s.updatePlan)
	writer.POST("/plans/:id/cover", s.uploadPlanImage("cover"))
	writer.POST("/plans/:id/icon", s.uploadPlanImage("icon"))
	writer.POST("/plans/:id/files", s.uploadPlanFile)
	writer.DELETE("/plans/:id/files/:fileID", s.deletePlanFile)
	writer.GET("/mood-tags", s.moodTags)
	writer.POST("/mood-tags", s.addMoodTag)
	writer.GET("/reader-keys", s.writerReaderKeys)
	writer.GET("/presets", s.listPresets)
	writer.POST("/presets", s.createPreset)
	writer.PUT("/presets/:id/rules", s.replacePresetRules)
	writer.POST("/comments", s.addComment)
	writer.POST("/milestones", s.addMilestone)
	writer.POST("/moods", s.addMood)
	writer.POST("/body", s.addBody)
	writer.POST("/diary/attachments", s.uploadAttachment)
	writer.GET("/music/playlists", s.listMusicPlaylists)
	writer.PUT("/music/playlists/:page", s.replaceMusicPlaylist)
	writer.DELETE("/music/playlists/:page", s.deleteMusicPlaylist)
	writer.POST("/music/playlists/:page/tracks", s.uploadMusicTrack)
	writer.DELETE("/music/tracks/:id", s.deleteMusicTrack)
	writer.PUT("/diary/attachments/:id/access", s.setAttachmentAccess)
	writer.PUT("/diary/draft", s.saveDraft)
	writer.PUT("/diary", s.saveDiary)
	writer.PUT("/diary/access", s.setDiaryAccess)
	writer.POST("/tasks", s.addTask)
	writer.POST("/tasks/:id/done", s.setTaskDone)
	writer.PUT("/tasks/:id/access", s.setTaskAccess)
	adminGroup := protected.Group("/admin")
	adminGroup.Use(s.requireAdmin())
	adminGroup.GET("/writers", s.listWriters)
	adminGroup.POST("/writers", s.createWriter)
	adminGroup.GET("/writers/:lifeID/reader-keys", s.listReaderKeys)
	adminGroup.POST("/writers/:lifeID/reader-keys", s.createReaderKey)
	adminGroup.POST("/reader-keys/:id/revoke", s.revokeReaderKey)
	return r
}
func (s *Server) cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://127.0.0.1:5173")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}
func (s *Server) keyLogin(c *gin.Context) {
	var request struct {
		Key string `json:"key"`
	}
	if !bind(c, &request) {
		return
	}
	token, actor, err := s.auth.KeyLogin(c.Request.Context(), request.Key, c.Request.UserAgent())
	if err != nil {
		fail(c, http.StatusUnauthorized, "invalid_credentials", "密钥无效或已失效")
		return
	}
	s.setCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"actor": actor})
}
func (s *Server) adminLogin(c *gin.Context) {
	var request struct {
		Password string `json:"password"`
	}
	if !bind(c, &request) {
		return
	}
	token, actor, err := s.auth.AdminLogin(c.Request.Context(), request.Password, c.Request.UserAgent())
	if err != nil {
		fail(c, http.StatusUnauthorized, "invalid_credentials", "管理员凭证无效")
		return
	}
	s.setCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"actor": actor})
}
func (s *Server) logout(c *gin.Context) {
	_ = s.auth.Logout(c.Request.Context(), s.token(c))
	c.SetCookie(s.cfg.SessionCookieName, "", -1, "/", "", s.cfg.SecureCookies, true)
	c.Status(http.StatusNoContent)
}
func (s *Server) me(c *gin.Context) {
	actor := c.MustGet("actor").(auth.Actor)
	c.JSON(http.StatusOK, gin.H{"actor": actor, "capabilities": capabilities(actor)})
}
func (s *Server) listWriters(c *gin.Context) {
	items, err := s.admin.ListWriters(c.Request.Context())
	if err != nil {
		internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (s *Server) createWriter(c *gin.Context) {
	var request struct {
		Nickname string `json:"nickname"`
	}
	if !bind(c, &request) {
		return
	}
	item, key, err := s.admin.CreateWriter(c.Request.Context(), strings.TrimSpace(request.Nickname))
	if err != nil {
		fail(c, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"writer": item, "master_key": key})
}
func (s *Server) listReaderKeys(c *gin.Context) {
	items, err := s.admin.ListReaderKeys(c.Request.Context(), c.Param("lifeID"))
	if err != nil {
		internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (s *Server) createReaderKey(c *gin.Context) {
	var request struct {
		Nickname  string  `json:"nickname"`
		Note      string  `json:"note"`
		ExpiresAt *string `json:"expires_at"`
	}
	if !bind(c, &request) {
		return
	}
	var expires *time.Time
	if request.ExpiresAt != nil {
		parsed, err := time.Parse(time.RFC3339, *request.ExpiresAt)
		if err != nil {
			fail(c, http.StatusBadRequest, "validation_failed", "expires_at 必须是 RFC3339 时间")
			return
		}
		expires = &parsed
	}
	item, key, err := s.admin.CreateReaderKey(c.Request.Context(), c.Param("lifeID"), strings.TrimSpace(request.Nickname), strings.TrimSpace(request.Note), expires)
	if err != nil {
		fail(c, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"reader_key": item, "key": key})
}
func (s *Server) revokeReaderKey(c *gin.Context) {
	if err := s.admin.RevokeReaderKey(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, http.StatusNotFound, "not_found", "阅读密钥不存在或已作废")
		return
	}
	c.Status(http.StatusNoContent)
}
// planReadable applies the ACL to a plan. The end date is the anchor date: a plan that is still running
// after a reader key was issued stays visible to that reader.
func (s *Server) planReadable(ctx context.Context, a auth.Actor, plan future.Plan) (bool, error) {
	return s.acl.CanRead(ctx, a, acl.Resource{LifeID: a.LifeID, Date: plan.EndDate, PresetID: plan.PresetID, Secret: plan.Secret})
}
func (s *Server) listPlans(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	if a.Type != "writer" && a.Type != "reader" {
		fail(c, 403, "forbidden", "当前身份不能读取规划")
		return
	}
	x, e := s.future.ListPlans(c.Request.Context(), a.LifeID)
	if e != nil {
		internal(c, e)
		return
	}
	visible := []future.Plan{}
	for _, plan := range x {
		ok, e := s.planReadable(c.Request.Context(), a, plan)
		if e != nil {
			internal(c, e)
			return
		}
		if ok {
			visible = append(visible, plan)
		}
	}
	c.JSON(200, gin.H{"items": visible})
}
func (s *Server) uploadPlanImage(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, (5<<20)+1024)
		file, header, e := c.Request.FormFile("file")
		if e != nil {
			fail(c, 400, "file_rejected", "请选择小于 5MB 的图片")
			return
		}
		defer file.Close()
		a := c.MustGet("actor").(auth.Actor)
		x, e := s.future.SetImage(c.Request.Context(), a.LifeID, c.Param("id"), kind, header.Filename, header.Header.Get("Content-Type"), header.Size, file)
		if e != nil {
			fail(c, 400, "file_rejected", e.Error())
			return
		}
		c.JSON(200, x)
	}
}
func (s *Server) uploadPlanFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, (20<<20)+1024)
	file, header, e := c.Request.FormFile("file")
	if e != nil {
		fail(c, 400, "file_rejected", "请选择小于 20MB 的文件")
		return
	}
	defer file.Close()
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.future.AddFile(c.Request.Context(), a.LifeID, c.Param("id"), header.Filename, header.Header.Get("Content-Type"), header.Size, file)
	if e != nil {
		fail(c, 400, "file_rejected", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) deletePlanFile(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	if e := s.future.DeleteFile(c.Request.Context(), a.LifeID, c.Param("id"), c.Param("fileID")); e != nil {
		fail(c, 404, "not_found", e.Error())
		return
	}
	c.Status(204)
}
func (s *Server) downloadPlanImage(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		a := c.MustGet("actor").(auth.Actor)
		plan, path, contentType, e := s.future.ImageForRead(c.Request.Context(), a.LifeID, c.Param("id"), kind)
		if e != nil {
			fail(c, 404, "not_found", "图片不存在")
			return
		}
		ok, e := s.planReadable(c.Request.Context(), a, plan)
		if e != nil {
			internal(c, e)
			return
		}
		if !ok {
			fail(c, 404, "not_found", "图片不存在")
			return
		}
		file, e := os.Open(path)
		if e != nil {
			fail(c, 404, "not_found", "图片不存在")
			return
		}
		defer file.Close()
		stat, e := file.Stat()
		if e != nil {
			internal(c, e)
			return
		}
		c.Header("Content-Type", contentType)
		http.ServeContent(c.Writer, c.Request, filepath.Base(path), stat.ModTime(), file)
	}
}
func (s *Server) downloadPlanFile(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	item, plan, path, e := s.future.FileForRead(c.Request.Context(), a.LifeID, c.Param("id"), c.Param("fileID"))
	if e != nil {
		fail(c, 404, "not_found", "文件不存在")
		return
	}
	ok, e := s.planReadable(c.Request.Context(), a, plan)
	if e != nil {
		internal(c, e)
		return
	}
	if !ok {
		fail(c, 404, "not_found", "文件不存在")
		return
	}
	file, e := os.Open(path)
	if e != nil {
		fail(c, 404, "not_found", "文件不存在")
		return
	}
	defer file.Close()
	stat, e := file.Stat()
	if e != nil {
		internal(c, e)
		return
	}
	c.Header("Content-Type", item.MimeType)
	c.Header("Content-Disposition", `attachment; filename="`+item.OriginalName+`"`)
	http.ServeContent(c.Writer, c.Request, item.OriginalName, stat.ModTime(), file)
}
func (s *Server) createPlan(c *gin.Context) {
	var r struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Intro     string `json:"intro"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.future.CreatePlan(c.Request.Context(), a.LifeID, strings.TrimSpace(r.Name), r.StartDate, r.EndDate, r.Intro)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) updatePlan(c *gin.Context) {
	var r struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Intro     string `json:"intro"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.future.UpdatePlan(c.Request.Context(), a.LifeID, c.Param("id"), strings.TrimSpace(r.Name), r.StartDate, r.EndDate, r.Intro)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(200, x)
}
func (s *Server) reorderPlans(c *gin.Context) {
	var r struct {
		IDs []string `json:"ids"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	if e := s.future.ReorderPlans(c.Request.Context(), a.LifeID, r.IDs); e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.Status(204)
}
func (s *Server) setPlanProgress(c *gin.Context) {
	var r struct {
		Date    string  `json:"date"`
		Percent float64 `json:"percent"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.future.SetProgress(c.Request.Context(), a.LifeID, c.Param("id"), r.Date, r.Percent)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(200, x)
}
func (s *Server) futureCalendar(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	if a.Type != "writer" && a.Type != "reader" {
		fail(c, 403, "forbidden", "当前身份不能读取日历")
		return
	}
	x, e := s.future.Calendar(c.Request.Context(), a.LifeID, c.Query("from"), c.Query("to"))
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	visible := []future.Task{}
	for _, task := range x {
		ok, e := s.acl.CanRead(c.Request.Context(), a, acl.Resource{LifeID: a.LifeID, Date: task.Date, PresetID: task.PresetID, Secret: task.Secret})
		if e != nil {
			internal(c, e)
			return
		}
		if ok {
			visible = append(visible, task)
		}
	}
	c.JSON(200, gin.H{"items": visible})
}
func (s *Server) listNotifications(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	if a.Type == "admin" {
		fail(c, 403, "forbidden", "管理员不使用人生通知中心")
		return
	}
	s.sweepNotifications(c.Request.Context(), a)
	x, e := s.notification.List(c.Request.Context(), a.LifeID, a.ID)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, gin.H{"items": x})
}
func (s *Server) markNotificationRead(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	if e := s.notification.MarkRead(c.Request.Context(), a.LifeID, c.Param("id")); e != nil {
		fail(c, 404, "not_found", "通知不存在")
		return
	}
	c.Status(204)
}
func (s *Server) historyRange(c *gin.Context) {
	from, e := time.Parse("2006-01-02", c.Query("from"))
	if e != nil {
		fail(c, 400, "validation_failed", "from 必须为 YYYY-MM-DD")
		return
	}
	to, e := time.Parse("2006-01-02", c.Query("to"))
	if e != nil {
		fail(c, 400, "validation_failed", "to 必须为 YYYY-MM-DD")
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.history.Range(c.Request.Context(), a, from, to)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(200, x)
}
func (s *Server) downloadAttachment(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	x, entryDate, path, e := s.now.AttachmentForRead(c.Request.Context(), a.LifeID, c.Param("id"))
	if e != nil {
		fail(c, 404, "not_found", "附件不存在")
		return
	}
	ok, e := s.acl.CanRead(c.Request.Context(), a, acl.Resource{LifeID: a.LifeID, Date: entryDate, PresetID: x.PresetID, Secret: x.Secret})
	if e != nil {
		internal(c, e)
		return
	}
	if !ok {
		fail(c, 404, "not_found", "附件不存在")
		return
	}
	file, e := os.Open(path)
	if e != nil {
		fail(c, 404, "not_found", "附件不存在")
		return
	}
	defer file.Close()
	c.Header("Content-Type", x.MimeType)
	c.Header("Content-Disposition", `attachment; filename="`+x.OriginalName+`"`)
	http.ServeContent(c.Writer, c.Request, file.Name(), time.Now(), file)
}
func (s *Server) writerReaderKeys(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.admin.ListReaderKeys(c.Request.Context(), a.LifeID)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, gin.H{"items": x})
}
func (s *Server) listPresets(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.acl.ListPresets(c.Request.Context(), a.LifeID)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, gin.H{"items": x})
}
func (s *Server) createPreset(c *gin.Context) {
	var r struct {
		Name  string     `json:"name"`
		Rules []acl.Rule `json:"rules"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.acl.CreatePreset(c.Request.Context(), a.LifeID, strings.TrimSpace(r.Name), r.Rules)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) replacePresetRules(c *gin.Context) {
	var r struct {
		Rules []acl.Rule `json:"rules"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	if e := s.acl.ReplacePresetRules(c.Request.Context(), a.LifeID, c.Param("id"), r.Rules); e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.Status(204)
}
func (s *Server) addVisibleComment(c *gin.Context) {
	var r struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Content    string `json:"content"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	target, e := s.interaction.TargetAccess(c.Request.Context(), a.LifeID, r.TargetType, r.TargetID)
	if e != nil || !target.Commentable {
		fail(c, 404, "not_found", "目标不存在或不可评论")
		return
	}
	ok, e := s.acl.CanRead(c.Request.Context(), a, acl.Resource{LifeID: a.LifeID, Date: target.Date, PresetID: target.PresetID, Secret: target.Secret})
	if e != nil {
		internal(c, e)
		return
	}
	if !ok {
		fail(c, 404, "not_found", "目标不存在或不可评论")
		return
	}
	x, target, e := s.interaction.AddComment(c.Request.Context(), a.LifeID, a.ID, r.TargetType, r.TargetID, strings.TrimSpace(r.Content))
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	s.notifyComment(c.Request.Context(), a, target, x)
	c.JSON(201, x)
}
// notifyComment tells the writer about a reader's comment; the message carries the day so the client
// can open it on the past page.
func (s *Server) notifyComment(ctx context.Context, a auth.Actor, target interaction.TargetAccess, comment interaction.Comment) {
	if a.Type != "reader" {
		return
	}
	writer, e := s.notification.WriterOf(ctx, a.LifeID)
	if e != nil || writer == "" {
		return
	}
	name := a.Nickname
	if name == "" {
		name = "阅读者"
	}
	kind := "日记"
	if target.TargetType == "task" {
		kind = "日程"
	}
	snippet := []rune(comment.Content)
	if len(snippet) > 40 {
		snippet = append(snippet[:40], '…')
	}
	_ = s.notification.Enqueue(ctx, a.LifeID, writer, "comment", target.TargetID, target.Date, fmt.Sprintf("%s 评论了 %s 的%s：%s", name, target.Date, kind, string(snippet)))
}
// sweepNotifications generates due reminders lazily on every inbox read: plans ending within three
// days and reader keys expiring within seven days. EnsureOnce keeps them from duplicating.
func (s *Server) sweepNotifications(ctx context.Context, a auth.Actor) {
	zone := time.FixedZone("CST", 28800)
	today, e := time.ParseInLocation("2006-01-02", acl.Today(), zone)
	if e != nil {
		return
	}
	if a.Type == "writer" {
		if plans, e := s.future.ListPlans(ctx, a.LifeID); e == nil {
			for _, plan := range plans {
				end, e := time.ParseInLocation("2006-01-02", plan.EndDate, zone)
				if e != nil || plan.Progress >= 100 {
					continue
				}
				days := int(end.Sub(today).Hours() / 24)
				if days < 0 || days > 3 {
					continue
				}
				text := fmt.Sprintf("规划「%s」今天到期", plan.Name)
				if days > 0 {
					text = fmt.Sprintf("规划「%s」将在 %d 天后到期", plan.Name, days)
				}
				_ = s.notification.EnsureOnce(ctx, a.LifeID, a.ID, "plan_due", plan.ID, plan.EndDate, text)
			}
		}
	}
	keys, e := s.admin.ListReaderKeys(ctx, a.LifeID)
	if e != nil {
		return
	}
	for _, key := range keys {
		if key.ExpiresAt == nil || key.RevokedAt != nil {
			continue
		}
		if a.Type == "reader" && key.ID != a.ID {
			continue
		}
		expiry, e := time.Parse(time.RFC3339Nano, *key.ExpiresAt)
		if e != nil {
			continue
		}
		days := int(expiry.Sub(time.Now()).Hours() / 24)
		if days < 0 || days > 7 {
			continue
		}
		date := expiry.In(zone).Format("2006-01-02")
		_ = s.notification.EnsureOnce(ctx, a.LifeID, a.ID, "key_expired", key.ID, date, fmt.Sprintf("阅读密钥「%s」将于 %s 到期", key.Nickname, date))
	}
}
func (s *Server) listVisibleComments(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	targetType, targetID := c.Query("target_type"), c.Query("target_id")
	target, e := s.interaction.TargetAccess(c.Request.Context(), a.LifeID, targetType, targetID)
	if e != nil || !target.Commentable {
		fail(c, 404, "not_found", "目标不存在或不可评论")
		return
	}
	ok, e := s.acl.CanRead(c.Request.Context(), a, acl.Resource{LifeID: a.LifeID, Date: target.Date, PresetID: target.PresetID, Secret: target.Secret})
	if e != nil {
		internal(c, e)
		return
	}
	if !ok {
		fail(c, 404, "not_found", "目标不存在或不可评论")
		return
	}
	x, e := s.interaction.ListComments(c.Request.Context(), a.LifeID, targetType, targetID)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, gin.H{"items": x})
}
func (s *Server) listVisibleMilestones(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	targetType, targetID := c.Query("target_type"), c.Query("target_id")
	target, e := s.interaction.TargetAccess(c.Request.Context(), a.LifeID, targetType, targetID)
	if e != nil {
		fail(c, 404, "not_found", "目标不存在")
		return
	}
	ok, e := s.acl.CanRead(c.Request.Context(), a, acl.Resource{LifeID: a.LifeID, Date: target.Date, PresetID: target.PresetID, Secret: target.Secret})
	if e != nil {
		internal(c, e)
		return
	}
	if !ok {
		fail(c, 404, "not_found", "目标不存在")
		return
	}
	items, e := s.interaction.ListMilestones(c.Request.Context(), a.LifeID, targetType, targetID)
	if e != nil {
		internal(c, e)
		return
	}
	visible := []interaction.Milestone{}
	for _, x := range items {
		ok, e := s.acl.CanRead(c.Request.Context(), a, acl.Resource{LifeID: a.LifeID, Date: target.Date, PresetID: x.PresetID, Secret: x.Secret})
		if e != nil {
			internal(c, e)
			return
		}
		if ok {
			visible = append(visible, x)
		}
	}
	c.JSON(200, gin.H{"items": visible})
}
func (s *Server) addComment(c *gin.Context) {
	var r struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Content    string `json:"content"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, _, e := s.interaction.AddComment(c.Request.Context(), a.LifeID, a.ID, r.TargetType, r.TargetID, strings.TrimSpace(r.Content))
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) addMilestone(c *gin.Context) {
	var r struct {
		TargetType  string `json:"target_type"`
		TargetID    string `json:"target_id"`
		Description string `json:"description"`
		Detail      string `json:"detail"`
		PresetID    string `json:"preset_id"`
		Secret      bool   `json:"secret"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.interaction.AddMilestone(c.Request.Context(), a.LifeID, r.TargetType, r.TargetID, strings.TrimSpace(r.Description), strings.TrimSpace(r.Detail), r.PresetID, r.Secret)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) nowToday(c *gin.Context) { s.visibleToday(c) }
func (s *Server) visibleToday(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	d, m, b, t, e := s.now.Today(c.Request.Context(), a.LifeID)
	if e != nil {
		internal(c, e)
		return
	}
	if a.Type == "writer" {
		secretDiary, e := s.now.SecretDiary(c.Request.Context(), a.LifeID)
		if e != nil {
			internal(c, e)
			return
		}
		c.JSON(200, gin.H{"diary": d, "secretDiary": secretDiary, "moods": m, "bodies": b, "tasks": t})
		return
	}
	if a.Type != "reader" {
		fail(c, http.StatusForbidden, "forbidden", "当前身份不能读取内容")
		return
	}
	ctx := c.Request.Context()
	if ok, e := s.acl.CanRead(ctx, a, acl.Resource{LifeID: a.LifeID, Date: d.EntryDate, PresetID: d.PresetID, Secret: d.Secret}); e != nil {
		internal(c, e)
		return
	} else if !ok {
		d = nowservice.Diary{EntryDate: d.EntryDate}
	}
	visibleMoods := []nowservice.MoodRecord{}
	for _, x := range m {
		if ok, e := s.acl.CanRead(ctx, a, acl.Resource{LifeID: a.LifeID, Date: x.RecordedDate, Secret: x.Secret}); e != nil {
			internal(c, e)
			return
		} else if ok {
			visibleMoods = append(visibleMoods, x)
		}
	}
	visibleBodies := []nowservice.BodyRecord{}
	for _, x := range b {
		if ok, e := s.acl.CanRead(ctx, a, acl.Resource{LifeID: a.LifeID, Date: x.RecordedDate, Secret: x.Secret}); e != nil {
			internal(c, e)
			return
		} else if ok {
			visibleBodies = append(visibleBodies, x)
		}
	}
	visibleTasks := []nowservice.Task{}
	for _, x := range t {
		if ok, e := s.acl.CanRead(ctx, a, acl.Resource{LifeID: a.LifeID, Date: x.TaskDate, PresetID: x.PresetID, Secret: x.Secret}); e != nil {
			internal(c, e)
			return
		} else if ok {
			visibleTasks = append(visibleTasks, x)
		}
	}
	c.JSON(200, gin.H{"diary": d, "moods": visibleMoods, "bodies": visibleBodies, "tasks": visibleTasks})
}
func (s *Server) moodTags(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.Tags(c.Request.Context(), a.LifeID)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, gin.H{"items": x})
}
func (s *Server) addMoodTag(c *gin.Context) {
	var r struct {
		Name  string `json:"name"`
		Emoji string `json:"emoji"`
		Value int    `json:"value"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.AddTag(c.Request.Context(), a.LifeID, strings.TrimSpace(r.Name), strings.TrimSpace(r.Emoji), r.Value)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) addMood(c *gin.Context) {
	var r struct {
		Note   string   `json:"note"`
		TagIDs []string `json:"tag_ids"`
		Secret bool     `json:"secret"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.AddMood(c.Request.Context(), a.LifeID, strings.TrimSpace(r.Note), r.TagIDs, r.Secret)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) addBody(c *gin.Context) {
	var r struct {
		Score  int    `json:"score"`
		Note   string `json:"note"`
		Secret bool   `json:"secret"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.AddBody(c.Request.Context(), a.LifeID, r.Score, strings.TrimSpace(r.Note), r.Secret)
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) listMusicPlaylists(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	items, err := s.music.List(c.Request.Context(), a.LifeID)
	if err != nil {
		internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) replaceMusicPlaylist(c *gin.Context) {
	var request struct {
		Name           string `json:"name"`
		Mode           string `json:"mode"`
		Volume         int    `json:"volume"`
		DefaultEnabled *bool  `json:"default_enabled"`
	}
	if !bind(c, &request) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	item, err := s.music.Replace(c.Request.Context(), a.LifeID, c.Param("page"), request.Name, request.Mode, request.Volume, request.DefaultEnabled)
	if err != nil {
		fail(c, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) deleteMusicPlaylist(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	if err := s.music.DeletePlaylist(c.Request.Context(), a.LifeID, c.Param("page")); err != nil {
		fail(c, http.StatusNotFound, "not_found", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) uploadMusicTrack(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, (50<<20)+1024)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "file_rejected", "请选择小于 50MB 的音频文件")
		return
	}
	defer file.Close()
	a := c.MustGet("actor").(auth.Actor)
	track, err := s.music.AddTrack(c.Request.Context(), a.LifeID, c.Param("page"), header.Filename, header.Header.Get("Content-Type"), header.Size, file)
	if err != nil {
		fail(c, http.StatusBadRequest, "file_rejected", err.Error())
		return
	}
	c.JSON(http.StatusCreated, track)
}

func (s *Server) deleteMusicTrack(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	if err := s.music.DeleteTrack(c.Request.Context(), a.LifeID, c.Param("id")); err != nil {
		fail(c, http.StatusNotFound, "not_found", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) downloadMusicTrack(c *gin.Context) {
	a := c.MustGet("actor").(auth.Actor)
	track, path, err := s.music.TrackForRead(c.Request.Context(), a.LifeID, c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "not_found", err.Error())
		return
	}
	file, err := os.Open(path)
	if err != nil {
		fail(c, http.StatusNotFound, "not_found", "音频文件不存在")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		internal(c, err)
		return
	}
	c.Header("Content-Type", track.MimeType)
	c.Header("Content-Disposition", `inline; filename="`+filepath.Base(track.Title)+`"`)
	http.ServeContent(c.Writer, c.Request, track.Title, stat.ModTime(), file)
}

func (s *Server) uploadAttachment(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20<<20+1024)
	file, header, e := c.Request.FormFile("file")
	if e != nil {
		fail(c, 400, "file_rejected", "请选择小于 20MB 的附件")
		return
	}
	defer file.Close()
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.SaveAttachment(c.Request.Context(), a.LifeID, header.Filename, header.Header.Get("Content-Type"), header.Size, file)
	if e != nil {
		fail(c, 400, "file_rejected", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) setAttachmentAccess(c *gin.Context) {
	var r struct {
		PresetID string `json:"preset_id"`
		Secret   bool   `json:"secret"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.SetAttachmentAccess(c.Request.Context(), a.LifeID, c.Param("id"), r.PresetID, r.Secret)
	if e != nil {
		fail(c, 404, "not_found", e.Error())
		return
	}
	c.JSON(200, x)
}
func (s *Server) saveDraft(c *gin.Context) {
	var r struct {
		Content string `json:"content"`
		Secret  bool   `json:"secret"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.SaveDraft(c.Request.Context(), a.LifeID, r.Content, r.Secret)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, x)
}
func (s *Server) saveDiary(c *gin.Context) {
	var r struct {
		Content string `json:"content"`
		Secret  bool   `json:"secret"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.SaveDiary(c.Request.Context(), a.LifeID, r.Content, r.Secret)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, x)
}
func (s *Server) setDiaryAccess(c *gin.Context) {
	var r struct {
		PresetID    string `json:"preset_id"`
		Secret      bool   `json:"secret"`
		Commentable bool   `json:"commentable"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.SetDiaryAccess(c.Request.Context(), a.LifeID, r.PresetID, r.Secret, r.Commentable)
	if e != nil {
		internal(c, e)
		return
	}
	c.JSON(200, x)
}
func (s *Server) addTask(c *gin.Context) {
	var r struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		Date        string `json:"date"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.AddTask(c.Request.Context(), a.LifeID, strings.TrimSpace(r.Title), strings.TrimSpace(r.Description), r.Priority, strings.TrimSpace(r.Date))
	if e != nil {
		fail(c, 400, "validation_failed", e.Error())
		return
	}
	c.JSON(201, x)
}
func (s *Server) setTaskAccess(c *gin.Context) {
	var r struct {
		PresetID    string `json:"preset_id"`
		Secret      bool   `json:"secret"`
		Commentable bool   `json:"commentable"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.SetTaskAccess(c.Request.Context(), a.LifeID, c.Param("id"), r.PresetID, r.Secret, r.Commentable)
	if e != nil {
		fail(c, 404, "not_found", e.Error())
		return
	}
	c.JSON(200, x)
}
func (s *Server) setTaskDone(c *gin.Context) {
	var r struct {
		Done bool   `json:"done"`
		Date string `json:"date"`
	}
	if !bind(c, &r) {
		return
	}
	a := c.MustGet("actor").(auth.Actor)
	x, e := s.now.SetTaskDone(c.Request.Context(), a.LifeID, c.Param("id"), r.Done, strings.TrimSpace(r.Date))
	if e != nil {
		fail(c, 404, "not_found", e.Error())
		return
	}
	c.JSON(200, x)
}
func (s *Server) requireActor() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, err := s.auth.Authenticate(c.Request.Context(), s.token(c))
		if err != nil {
			fail(c, http.StatusUnauthorized, "unauthorized", "请先登录")
			c.Abort()
			return
		}
		c.Set("actor", actor)
		c.Next()
	}
}
func (s *Server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.MustGet("actor").(auth.Actor).Type != "admin" {
			fail(c, http.StatusForbidden, "forbidden", "需要管理员权限")
			c.Abort()
			return
		}
		c.Next()
	}
}
func (s *Server) requireWriter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.MustGet("actor").(auth.Actor).Type != "writer" {
			fail(c, http.StatusForbidden, "forbidden", "当前操作仅限书写者")
			c.Abort()
			return
		}
		c.Next()
	}
}
func (s *Server) token(c *gin.Context) string {
	if value, err := c.Cookie(s.cfg.SessionCookieName); err == nil {
		return value
	}
	return strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
}
func (s *Server) setCookie(c *gin.Context, token string) {
	c.SetCookie(s.cfg.SessionCookieName, token, 60*60*24*3650, "/", "", s.cfg.SecureCookies, true)
}
func bind(c *gin.Context, value any) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		fail(c, http.StatusBadRequest, "validation_failed", "请求 JSON 无效")
		return false
	}
	return true
}
func capabilities(actor auth.Actor) []string {
	if actor.Type == "admin" {
		return []string{"admin:manage_writers", "admin:manage_reader_keys"}
	}
	if actor.Type == "writer" {
		return []string{"content:write", "keys:manage"}
	}
	return []string{"content:read", "comments:write"}
}
func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
func internal(c *gin.Context, err error) {
	_ = errors.Unwrap(err)
	fail(c, http.StatusInternalServerError, "internal_error", "服务暂时不可用")
}
