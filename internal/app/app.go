package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"status-links/internal/config"
	"status-links/internal/handlers"
	"status-links/internal/services"
	"status-links/internal/storage"
	"syscall"
	"time"
)

type App struct {
	cfg      *config.Config
	server   *http.Server
	services *Services
	storages *Storages
}

type Services struct {
	LinksService services.LinkProcessor
}

type Storages struct {
	temp     storage.TempStorage
	reliable storage.ReliableStorage
}

func NewApp(cfg *config.Config) *App {

	app := &App{
		cfg: cfg,
	}

	app.initStorages()
	app.initServices()
	app.initHTTP()

	return app
}

func (a *App) initStorages() {
	a.storages = &Storages{
		temp: storage.NewTempStorage(),
		reliable: storage.NewReliableStorage(
			a.cfg.NameFileAllTasks,
			a.cfg.NameFileProcessTasksLinks,
			a.cfg.NameFileProcessTasksNums,
		),
	}
}

func (a *App) initServices() {
	a.services = &Services{
		LinksService: services.NewLinksService(a.storages.temp, a.storages.reliable),
	}
}

func (a *App) initHTTP() {
	handler, err := handlers.NewHandler(a.services.LinksService)
	if err != nil {
		slog.Error("Failed to create handler", "error", err)
		os.Exit(1)
	}

	router := a.setupRoutes(handler)

	a.server = &http.Server{
		Addr:         ":" + a.cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
}

func (a *App) setupRoutes(handler *handlers.Handler) http.Handler {
	mux := http.NewServeMux()

	// Static files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// API routes
	apiRoutes := map[string]http.HandlerFunc{
		"/api/loadUnfinishedWork": handler.LoadUnfinishedWork,
		"/api/saveNewUrls":        handler.SaveNewUrls,
		"/api/loadUrls":           handler.LoadUrls,
	}

	for path, handlerFunc := range apiRoutes {
		mux.HandleFunc(path, handlerFunc)
	}

	return mux
}

func (a *App) Run() {
	go a.startServer()
	a.waitForShutdown()
}

func (a *App) startServer() {
	slog.Info("Server starting", "port", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func (a *App) waitForShutdown() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("Shutting down server gracefully...")
	a.shutdown()
}

func (a *App) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}
	a.services.LinksService.WaitForCompletion()
	slog.Info("Server stopped")
}
