package app

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/example/one-template-go-api/internal/config"
	apihttp "github.com/example/one-template-go-api/internal/http"
	"github.com/example/one-template-go-api/internal/http/handlers"
	"github.com/example/one-template-go-api/internal/platform/jwt"
	"github.com/example/one-template-go-api/internal/platform/postgres"
	"github.com/example/one-template-go-api/internal/repository"
	"github.com/example/one-template-go-api/internal/service"
)

type App struct {
	cfg    config.Config
	log    *zap.Logger
	db     *gorm.DB
	router *apihttp.Router
}

func New(cfg config.Config, log *zap.Logger) (*App, error) {
	db, err := postgres.Open(cfg.DatabaseURL, log)
	if err != nil {
		return nil, err
	}

	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpiresIn)
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	authService := service.NewAuthService(userRepo, jwtManager)

	handlerSet := handlers.Set{
		App:    handlers.NewAppHandler(cfg),
		Health: handlers.NewHealthHandler(db),
		Auth:   handlers.NewAuthHandler(authService),
		Users:  handlers.NewUserHandler(userService),
	}

	return &App{
		cfg: cfg,
		log: log,
		db:  db,
		router: apihttp.NewRouter(apihttp.Dependencies{
			Config:  cfg,
			Logger:  log,
			JWT:     jwtManager,
			Handler: handlerSet,
		}),
	}, nil
}

func (a *App) Router() *apihttp.Router {
	return a.router
}
