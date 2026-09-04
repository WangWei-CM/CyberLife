package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cyberlife/server/internal/admin"
	"cyberlife/server/internal/auth"
	"cyberlife/server/internal/config"
	nowservice "cyberlife/server/internal/now"
)

type Server struct { cfg config.Config; auth *auth.Service; admin *admin.Service; now *nowservice.Service }
func New(cfg config.Config, authService *auth.Service, adminService *admin.Service, nowService *nowservice.Service) *Server { return &Server{cfg:cfg,auth:authService,admin:adminService,now:nowService} }

func (s *Server) Router() *gin.Engine {
	r:=gin.New(); r.Use(gin.Logger(),gin.Recovery(),s.cors())
	r.GET("/health/live",func(c *gin.Context){c.JSON(http.StatusOK,gin.H{"status":"ok"})})
	r.POST("/api/v1/auth/key-login",s.keyLogin);r.POST("/api/v1/admin/auth/login",s.adminLogin);r.POST("/api/v1/auth/logout",s.logout)
	protected:=r.Group("/api/v1");protected.Use(s.requireActor())
	protected.GET("/auth/me",s.me)
	writer:=protected.Group("/now");writer.Use(s.requireWriter())
	writer.GET("",s.nowToday);writer.GET("/mood-tags",s.moodTags);writer.POST("/mood-tags",s.addMoodTag);writer.POST("/moods",s.addMood);writer.POST("/body",s.addBody);writer.PUT("/diary",s.saveDiary);writer.POST("/tasks",s.addTask);writer.POST("/tasks/:id/done",s.setTaskDone)
	adminGroup:=protected.Group("/admin");adminGroup.Use(s.requireAdmin())
	adminGroup.GET("/writers",s.listWriters);adminGroup.POST("/writers",s.createWriter);adminGroup.GET("/writers/:lifeID/reader-keys",s.listReaderKeys);adminGroup.POST("/writers/:lifeID/reader-keys",s.createReaderKey);adminGroup.POST("/reader-keys/:id/revoke",s.revokeReaderKey)
	return r
}
func (s *Server) cors() gin.HandlerFunc{return func(c *gin.Context){c.Header("Access-Control-Allow-Origin","http://127.0.0.1:5173");c.Header("Access-Control-Allow-Credentials","true");c.Header("Access-Control-Allow-Headers","Content-Type, Idempotency-Key");c.Header("Access-Control-Allow-Methods","GET,POST,OPTIONS");if c.Request.Method==http.MethodOptions{c.Status(http.StatusNoContent);c.Abort();return};c.Next()}}
func (s *Server) keyLogin(c *gin.Context){var request struct{Key string `json:"key"`};if !bind(c,&request){return};token,actor,err:=s.auth.KeyLogin(c.Request.Context(),request.Key,c.Request.UserAgent());if err!=nil{fail(c,http.StatusUnauthorized,"invalid_credentials","密钥无效或已失效");return};s.setCookie(c,token);c.JSON(http.StatusOK,gin.H{"actor":actor})}
func (s *Server) adminLogin(c *gin.Context){var request struct{Password string `json:"password"`};if !bind(c,&request){return};token,actor,err:=s.auth.AdminLogin(c.Request.Context(),request.Password,c.Request.UserAgent());if err!=nil{fail(c,http.StatusUnauthorized,"invalid_credentials","管理员凭证无效");return};s.setCookie(c,token);c.JSON(http.StatusOK,gin.H{"actor":actor})}
func (s *Server) logout(c *gin.Context){_ = s.auth.Logout(c.Request.Context(),s.token(c));c.SetCookie(s.cfg.SessionCookieName,"",-1,"/","",s.cfg.SecureCookies,true);c.Status(http.StatusNoContent)}
func (s *Server) me(c *gin.Context){actor:=c.MustGet("actor").(auth.Actor);c.JSON(http.StatusOK,gin.H{"actor":actor,"capabilities":capabilities(actor)})}
func (s *Server) listWriters(c *gin.Context){items,err:=s.admin.ListWriters(c.Request.Context());if err!=nil{internal(c,err);return};c.JSON(http.StatusOK,gin.H{"items":items})}
func (s *Server) createWriter(c *gin.Context){var request struct{Nickname string `json:"nickname"`};if !bind(c,&request){return};item,key,err:=s.admin.CreateWriter(c.Request.Context(),strings.TrimSpace(request.Nickname));if err!=nil{fail(c,http.StatusBadRequest,"validation_failed",err.Error());return};c.JSON(http.StatusCreated,gin.H{"writer":item,"master_key":key})}
func (s *Server) listReaderKeys(c *gin.Context){items,err:=s.admin.ListReaderKeys(c.Request.Context(),c.Param("lifeID"));if err!=nil{internal(c,err);return};c.JSON(http.StatusOK,gin.H{"items":items})}
func (s *Server) createReaderKey(c *gin.Context){var request struct{Nickname string `json:"nickname"`;Note string `json:"note"`;ExpiresAt *string `json:"expires_at"`};if !bind(c,&request){return};var expires *time.Time;if request.ExpiresAt!=nil{parsed,err:=time.Parse(time.RFC3339,*request.ExpiresAt);if err!=nil{fail(c,http.StatusBadRequest,"validation_failed","expires_at 必须是 RFC3339 时间");return};expires=&parsed};item,key,err:=s.admin.CreateReaderKey(c.Request.Context(),c.Param("lifeID"),strings.TrimSpace(request.Nickname),strings.TrimSpace(request.Note),expires);if err!=nil{fail(c,http.StatusBadRequest,"validation_failed",err.Error());return};c.JSON(http.StatusCreated,gin.H{"reader_key":item,"key":key})}
func (s *Server) revokeReaderKey(c *gin.Context){if err:=s.admin.RevokeReaderKey(c.Request.Context(),c.Param("id"));err!=nil{fail(c,http.StatusNotFound,"not_found","阅读密钥不存在或已作废");return};c.Status(http.StatusNoContent)}
func (s *Server) nowToday(c *gin.Context){a:=c.MustGet("actor").(auth.Actor);d,m,b,t,e:=s.now.Today(c.Request.Context(),a.LifeID);if e!=nil{internal(c,e);return};c.JSON(200,gin.H{"diary":d,"moods":m,"bodies":b,"tasks":t})}
func (s *Server) moodTags(c *gin.Context){a:=c.MustGet("actor").(auth.Actor);x,e:=s.now.Tags(c.Request.Context(),a.LifeID);if e!=nil{internal(c,e);return};c.JSON(200,gin.H{"items":x})}
func (s *Server) addMoodTag(c *gin.Context){var r struct{Name string `json:"name"`;Emoji string `json:"emoji"`;Value int `json:"value"`};if !bind(c,&r){return};a:=c.MustGet("actor").(auth.Actor);x,e:=s.now.AddTag(c.Request.Context(),a.LifeID,strings.TrimSpace(r.Name),strings.TrimSpace(r.Emoji),r.Value);if e!=nil{fail(c,400,"validation_failed",e.Error());return};c.JSON(201,x)}
func (s *Server) addMood(c *gin.Context){var r struct{Note string `json:"note"`;TagIDs []string `json:"tag_ids"`};if !bind(c,&r){return};a:=c.MustGet("actor").(auth.Actor);x,e:=s.now.AddMood(c.Request.Context(),a.LifeID,strings.TrimSpace(r.Note),r.TagIDs);if e!=nil{fail(c,400,"validation_failed",e.Error());return};c.JSON(201,x)}
func (s *Server) addBody(c *gin.Context){var r struct{Score int `json:"score"`;Note string `json:"note"`};if !bind(c,&r){return};a:=c.MustGet("actor").(auth.Actor);x,e:=s.now.AddBody(c.Request.Context(),a.LifeID,r.Score,strings.TrimSpace(r.Note));if e!=nil{fail(c,400,"validation_failed",e.Error());return};c.JSON(201,x)}
func (s *Server) saveDiary(c *gin.Context){var r struct{Content string `json:"content"`};if !bind(c,&r){return};a:=c.MustGet("actor").(auth.Actor);x,e:=s.now.SaveDiary(c.Request.Context(),a.LifeID,r.Content);if e!=nil{internal(c,e);return};c.JSON(200,x)}
func (s *Server) addTask(c *gin.Context){var r struct{Title string `json:"title"`;Description string `json:"description"`;Priority string `json:"priority"`};if !bind(c,&r){return};a:=c.MustGet("actor").(auth.Actor);x,e:=s.now.AddTask(c.Request.Context(),a.LifeID,strings.TrimSpace(r.Title),strings.TrimSpace(r.Description),r.Priority);if e!=nil{fail(c,400,"validation_failed",e.Error());return};c.JSON(201,x)}
func (s *Server) setTaskDone(c *gin.Context){var r struct{Done bool `json:"done"`};if !bind(c,&r){return};a:=c.MustGet("actor").(auth.Actor);x,e:=s.now.SetTaskDone(c.Request.Context(),a.LifeID,c.Param("id"),r.Done);if e!=nil{fail(c,404,"not_found",e.Error());return};c.JSON(200,x)}
func (s *Server) requireActor()gin.HandlerFunc{return func(c *gin.Context){actor,err:=s.auth.Authenticate(c.Request.Context(),s.token(c));if err!=nil{fail(c,http.StatusUnauthorized,"unauthorized","请先登录");c.Abort();return};c.Set("actor",actor);c.Next()}}
func (s *Server) requireAdmin()gin.HandlerFunc{return func(c *gin.Context){if c.MustGet("actor").(auth.Actor).Type!="admin"{fail(c,http.StatusForbidden,"forbidden","需要管理员权限");c.Abort();return};c.Next()}}
func (s *Server) requireWriter()gin.HandlerFunc{return func(c *gin.Context){if c.MustGet("actor").(auth.Actor).Type!="writer"{fail(c,http.StatusForbidden,"forbidden","当前操作仅限书写者");c.Abort();return};c.Next()}}
func (s *Server) token(c *gin.Context)string{if value,err:=c.Cookie(s.cfg.SessionCookieName);err==nil{return value};return strings.TrimPrefix(c.GetHeader("Authorization"),"Bearer ")}
func (s *Server) setCookie(c *gin.Context,token string){c.SetCookie(s.cfg.SessionCookieName,token,60*60*24*3650,"/","",s.cfg.SecureCookies,true)}
func bind(c *gin.Context,value any)bool{if err:=c.ShouldBindJSON(value);err!=nil{fail(c,http.StatusBadRequest,"validation_failed","请求 JSON 无效");return false};return true}
func capabilities(actor auth.Actor)[]string{if actor.Type=="admin"{return []string{"admin:manage_writers","admin:manage_reader_keys"}};if actor.Type=="writer"{return []string{"content:write","keys:manage"}};return []string{"content:read","comments:write"}}
func fail(c *gin.Context,status int,code,message string){c.JSON(status,gin.H{"error":gin.H{"code":code,"message":message}})}
func internal(c *gin.Context,err error){_ = errors.Unwrap(err);fail(c,http.StatusInternalServerError,"internal_error","服务暂时不可用")}
