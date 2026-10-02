package app

import (
	"github.com/example/one-template-go-api/internal/config"
	apihttp "github.com/example/one-template-go-api/internal/http"
	"go.uber.org/zap"
)

type App struct{ router *apihttp.Router }

func New(cfg config.Config, log *zap.Logger) *App {
	return &App{router: apihttp.NewRouter(cfg, log)}
}

func (a *App) Router() *apihttp.Router { return a.router }
