package main

import (
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/assistant"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/clock"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerRoutes is the composition root: it wires every concrete type and mounts
// the routes. It hands back how to end the live follows, because they are the one
// thing here that outlives the request that started it.
func registerRoutes(
	engine *gin.Engine, database *gorm.DB, applicationConfig config.ApplicationConfig,
) func() {
	engine.Use(controller.NewCorsMiddleware(applicationConfig.CorsAllowedOrigins).Handle)

	engine.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "Healthy"})
	})

	kCandleRepository := persistence.NewKCandleRepository(database)

	kCandleApplication := application.NewKCandleApplication(
		service.NewKCandleService(
			kCandleRepository,
			clock.NewSystemClockProxy(),
			applicationConfig.KCandleQueryMaxResults,
		),
	)

	kCandleController := controller.NewKCandleController(kCandleApplication)

	engine.POST("/k-candles", kCandleController.CreateKCandle)
	engine.GET("/k-candles", kCandleController.GetKCandlesInRange)
	engine.GET("/k-candles/series", kCandleController.GetKCandleSeries)
	engine.GET("/k-candles/:symbol/:openTime", kCandleController.GetKCandle)
	engine.PUT("/k-candles/:symbol/:openTime", kCandleController.UpdateKCandle)
	engine.DELETE("/k-candles/:symbol/:openTime", kCandleController.DeleteKCandle)

	// 交易標的是另一個資源（系統認得哪幾個市場），不是某一根 K 線，所以有自己的 controller 與路徑。
	tradingSymbolApplication := application.NewTradingSymbolApplication(
		service.NewTradingSymbolService(
			persistence.NewTradingSymbolRepository(database),
			kCandleRepository,
		),
	)

	engine.GET("/trading-symbols", controller.NewTradingSymbolController(
		tradingSymbolApplication).ListTradingSymbols)

	indicatorCalculationApplication := application.NewIndicatorCalculationApplication(
		service.NewIndicatorCalculationService(
			kCandleRepository,
			script.NewYaegiIndicatorScriptProxy(applicationConfig.IndicatorScriptTimeout),
			clock.NewSystemClockProxy(),
			applicationConfig.KCandleQueryMaxResults,
		),
	)

	engine.POST("/indicator-calculations", controller.NewIndicatorCalculationController(
		indicatorCalculationApplication).CalculateIndicator)

	// Replaying a strategy is its own use case rather than a mode of calculating an
	// indicator: it asks a different question of the same script, and it stores
	// nothing — which is why it is given no repository beyond the one it reads from.
	//
	// It shares the read ceiling with every other read, deliberately: a replay is
	// still one look at the market, and giving it a ceiling of its own would leave two
	// numbers to keep in step.
	engine.POST("/backtests", controller.NewBacktestController(
		application.NewBacktestApplication(
			service.NewBacktestService(
				kCandleRepository,
				script.NewYaegiIndicatorScriptProxy(applicationConfig.IndicatorScriptTimeout),
				clock.NewSystemClockProxy(),
				applicationConfig.KCandleQueryMaxResults,
			),
		)).RunBacktest)

	// A saved strategy is its own resource: it holds an algorithm and nothing else —
	// how coarse the K candles are, how many of them and up to when describe one run
	// and travel with the calculation instead. It reads no K candles, so it is given
	// no K candle repository.
	strategyApplication := application.NewStrategyApplication(
		service.NewStrategyService(persistence.NewStrategyRepository(database)),
	)

	strategyController := controller.NewStrategyController(strategyApplication)

	engine.POST("/strategies", strategyController.CreateStrategy)
	engine.GET("/strategies", strategyController.ListStrategies)
	engine.GET("/strategies/:id", strategyController.GetStrategy)
	engine.PUT("/strategies/:id", strategyController.UpdateStrategy)
	engine.DELETE("/strategies/:id", strategyController.DeleteStrategy)

	assistantConversationController := controller.NewAssistantConversationController(
		application.NewAssistantConversationApplication(
			service.NewAssistantConversationService(
				persistence.NewConversationRepository(database),
				assistant.NewClaudeAssistantProxy(
					applicationConfig.Assistant.ApiKey,
					applicationConfig.Assistant.Model,
					applicationConfig.Assistant.Effort,
					applicationConfig.Assistant.BaseUrl,
					applicationConfig.Assistant.ResponseTimeout,
				),
				assistantQueriesFor(
					tradingSymbolApplication,
					kCandleApplication,
					indicatorCalculationApplication,
					strategyApplication,
					applicationConfig.Assistant.CandleLimit,
				),
				clock.NewSystemClockProxy(),
				applicationConfig.Assistant.RecentMessageLimit,
				applicationConfig.Assistant.QueryLimit,
				applicationConfig.Assistant.DailyUsageAllowance,
				applicationConfig.Assistant.AnswerLengthLimit,
			),
		),
	)

	engine.POST("/chat", assistantConversationController.Ask)
	engine.GET("/chat/conversations", assistantConversationController.ListConversations)
	engine.GET("/chat/conversations/:id", assistantConversationController.GetConversation)

	// Recognising a person is the one thing here that has nothing to do with the
	// market, and it is wired from two capabilities that are pure cryptography:
	// turning a password into something storable, and turning an identity into
	// something signed. Both are behind interfaces, so replacing either — bcrypt for
	// something newer, one shared key for a key pair — is a new implementation and
	// one changed line here.
	//
	// Two of these routes are open to anybody who can reach this address, and that is
	// deliberate rather than overlooked: a system holding no users has nobody who
	// could be allowed to create the first one. Only "who am I" needs a proof, and it
	// is where this door is proved to actually shut. The market routes below are
	// still open to anyone — see the sign-in design notes for why they are a slice of
	// their own.
	userController := controller.NewUserController(
		application.NewUserApplication(
			service.NewUserService(
				persistence.NewUserRepository(database),
				persistence.NewSessionRepository(database),
				security.NewBcryptPasswordProofProxy(),
				security.NewJwtAccessTokenProxy(
					applicationConfig.Authentication.AccessTokenSigningKey),
				security.NewRandomRefreshTokenProxy(),
				clock.NewSystemClockProxy(),
				vo.SessionLifetimesVo{
					AccessToken:  applicationConfig.Authentication.AccessTokenLifetime,
					RefreshToken: applicationConfig.Authentication.RefreshTokenLifetime,
				},
			),
		),
	)

	engine.POST("/users", userController.RegisterUser)
	engine.POST("/sessions", userController.SignIn)
	// Renewing and ending are POSTs rather than one body-carrying DELETE: which
	// session is meant is named by the renewal proof, and a proof can only travel in
	// a body.
	engine.POST("/sessions/renewal", userController.RenewSession)
	engine.POST("/sessions/revocation", userController.RevokeSession)
	engine.GET("/users/me", userController.GetCurrentUser)

	// Following a market live is an addition, not a replacement: the five-minute
	// round keeps running, and it is what fills in every candle that closed while
	// nobody was looking. This path only shortens the wait for whoever is looking.
	kCandleFollowService := service.NewKCandleFollowService(
		marketdata.NewBinanceLiveMarketDataProxy(applicationConfig.LiveFollow.MarketDataStreamUrl),
		kCandleRepository,
		clock.NewSystemClockProxy(),
		applicationConfig.LiveFollow.UpdateIntervalCeiling,
		applicationConfig.LiveFollow.QuietTimeout,
		applicationConfig.LiveFollow.MaximumRetryDelay,
	)

	engine.GET("/k-candles/live", controller.NewKCandleFollowController(
		application.NewKCandleFollowApplication(kCandleFollowService),
	).WatchKCandles)

	return kCandleFollowService.Stop
}

// assistantQueriesFor is everything the assistant is allowed to do.
//
// It is assembled here and only here, which is what makes "it cannot delete a
// strategy" a fact about the system rather than a check somebody could remove: there
// is no deleting capability to reach for, and no K candle writing one either. Adding
// a capability is adding one line to this list.
//
// Each capability calls the very same use case a person calls, so no rule is relaxed
// for the assistant and none had to be written twice.
func assistantQueriesFor(
	tradingSymbolApplication *application.TradingSymbolApplication,
	kCandleApplication *application.KCandleApplication,
	indicatorCalculationApplication *application.IndicatorCalculationApplication,
	strategyApplication *application.StrategyApplication,
	candleLimit int,
) []domaininterface.IAssistantQuery {
	return []domaininterface.IAssistantQuery{
		application.NewTradingSymbolListAssistantQuery(tradingSymbolApplication),
		application.NewKCandleSeriesAssistantQuery(kCandleApplication, candleLimit),
		application.NewKCandleRangeAssistantQuery(kCandleApplication, candleLimit),
		application.NewIndicatorCalculationAssistantQuery(indicatorCalculationApplication, strategyApplication),
		application.NewStrategyListAssistantQuery(strategyApplication),
		application.NewStrategyGetAssistantQuery(strategyApplication),
		application.NewStrategyCreateAssistantQuery(strategyApplication),
		application.NewStrategyUpdateAssistantQuery(strategyApplication),
	}
}

// backgroundJobsFor assembles the work the system does on its own. Switching
// background jobs off leaves nothing to start; the ingestion job itself decides what
// an empty watchlist means, which is nothing to fetch.
func backgroundJobsFor(
	database *gorm.DB,
	applicationConfig config.ApplicationConfig,
) []domaininterface.IBackgroundJob {
	if !applicationConfig.BackgroundJobsEnabled {
		return []domaininterface.IBackgroundJob{}
	}

	kCandleIngestionJob := job.NewKCandleIngestionJob(
		application.NewKCandleIngestionApplication(
			service.NewKCandleIngestionService(
				persistence.NewKCandleRepository(database),
				marketdata.NewBinanceMarketDataProxy(
					applicationConfig.Ingestion.MarketDataBaseUrl,
					applicationConfig.Ingestion.MarketDataRequestTimeout,
				),
				clock.NewSystemClockProxy(),
				applicationConfig.Ingestion.RoundCandleCount,
				applicationConfig.Ingestion.BackfillLookback,
			),
		),
		applicationConfig.Ingestion.Symbols,
		job.KCandleIngestionInterval,
	)

	return []domaininterface.IBackgroundJob{kCandleIngestionJob}
}
