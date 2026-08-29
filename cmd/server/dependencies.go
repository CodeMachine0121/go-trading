package main

import (
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/clock"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerRoutes is the composition root: it wires every concrete type and mounts the routes.
func registerRoutes(engine *gin.Engine, database *gorm.DB, applicationConfig config.ApplicationConfig) {
	engine.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "Healthy"})
	})

	kCandleRepository := persistence.NewKCandleRepository(database)

	kCandleController := controller.NewKCandleController(
		application.NewKCandleApplication(
			service.NewKCandleService(
				kCandleRepository,
				clock.NewSystemClockProxy(),
				applicationConfig.KCandleQueryMaxResults,
			),
		),
	)

	engine.POST("/k-candles", kCandleController.CreateKCandle)
	engine.GET("/k-candles", kCandleController.GetKCandlesInRange)
	engine.GET("/k-candles/:symbol/:openTime", kCandleController.GetKCandle)
	engine.PUT("/k-candles/:symbol/:openTime", kCandleController.UpdateKCandle)
	engine.DELETE("/k-candles/:symbol/:openTime", kCandleController.DeleteKCandle)

	indicatorCalculationController := controller.NewIndicatorCalculationController(
		application.NewIndicatorCalculationApplication(
			service.NewIndicatorCalculationService(
				kCandleRepository,
				script.NewYaegiIndicatorScriptProxy(applicationConfig.IndicatorScriptTimeout),
				applicationConfig.KCandleQueryMaxResults,
			),
		),
	)

	engine.POST("/indicator-calculations", indicatorCalculationController.CalculateIndicator)
}
