package http

import (
	"github.com/example/one-template-go-api/internal/config"
	"github.com/example/one-template-go-api/internal/http/middleware"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"net/http"
)

type Router struct{ *gin.Engine }

func NewRouter(cfg config.Config, log *zap.Logger) *Router {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recovery(log), middleware.Logger(log), middleware.CORS(cfg))
	engine.GET("/", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"name": cfg.AppName, "version": "0.1.0"}) })
	engine.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	engine.GET("/api/docs", func(c *gin.Context) { c.Redirect(http.StatusTemporaryRedirect, "/api/docs/index.html") })
	engine.StaticFile("/api/openapi.yaml", "api/openapi.yaml")
	engine.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/openapi.yaml")))
	return &Router{Engine: engine}
}
