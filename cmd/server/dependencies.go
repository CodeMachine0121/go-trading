package main

import (
	"log"
	"net/http"
	"os"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/assistant"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/clock"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/messaging"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerRoutes is the composition root; it returns the instances background jobs must share with
// the routes, such as the ingestion that remembers which markets are closed today.
func registerRoutes(
	engine *gin.Engine, database *gorm.DB, applicationConfig config.ApplicationConfig,
) (
	liveFollowApplications,
	*application.KCandleIngestionApplication,
	*application.KCandleContractIngestionApplication,
	*application.StrategyBotRunApplication,
	*application.AssistantConversationApplication,
	contractSeriesApplications,
) {
	engine.Use(middlewares.NewCorsMiddleware(applicationConfig.CorsAllowedOrigins).Handle)

	engine.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "Healthy"})
	})

	// Built first because every owned resource sits behind sign-in.
	userApplication := application.NewUserApplication(
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
			vo.AccountActivationPolicyVo{
				RequestMailbox: applicationConfig.AccountActivation.RequestMailbox,
				SubjectPrefix:  applicationConfig.AccountActivation.SubjectPrefix,
			},
			vo.SignInLockoutPolicyVo{
				FailureThreshold: applicationConfig.SignInLockout.FailureThreshold,
				LockoutDuration:  applicationConfig.SignInLockout.LockoutDuration,
			},
		),
	)

	// Market data is deliberately public; only what belongs to a person (scripts, strategies, bots,
	// the assistant) is mounted behind sign-in.
	requiresSignIn := middlewares.NewAuthenticationMiddleware(userApplication).Handle

	kCandleRepository := persistence.NewKCandleRepository(database)

	// 每個來源一個節奏，所有打到那個來源的 proxy 共用，因為額度是照來源算的。
	venuePacers := newVenuePacers(applicationConfig)

	kCandleService := service.NewKCandleService(
		kCandleRepository,
		persistence.NewTradingSymbolRepository(database),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.KCandleQueryMaxResults,
	)

	kCandleApplication := application.NewKCandleApplication(kCandleService)

	kCandleController := controller.NewKCandleController(kCandleApplication)

	engine.POST("/k-candles", kCandleController.CreateKCandle)
	engine.GET("/k-candles", kCandleController.GetKCandlesInRange)
	engine.GET("/k-candles/series", kCandleController.GetKCandleSeries)
	engine.GET("/k-candles/:symbol/:openTime", kCandleController.GetKCandle)
	engine.PUT("/k-candles/:symbol/:openTime", kCandleController.UpdateKCandle)
	engine.DELETE("/k-candles/:symbol/:openTime", kCandleController.DeleteKCandle)

	// 與背景工作共用同一份實例，否則「今天休市」會各記各的。
	kCandleIngestionService := service.NewKCandleIngestionService(
		kCandleRepository,
		persistence.NewKCandleHistorySyncRunRepository(database),
		persistence.NewTradingSymbolRepository(database),
		marketDataProxyFor(applicationConfig, venuePacers),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.Ingestion.RoundCandleCount,
		applicationConfig.Ingestion.BackfillLookback,
	)
	kCandleIngestionApplication := application.NewKCandleIngestionApplication(kCandleIngestionService)

	engine.POST("/k-candles/backfill",
		controller.NewKCandleBackfillController(kCandleIngestionApplication).CatchUpSymbol)

	// 歷史同步先回 202 與一筆輪次，因為長區間抓取要數千次請求，進度由下面的查詢路由取得。
	kCandleHistorySyncController := controller.NewKCandleHistorySyncController(
		kCandleIngestionApplication,
		applicationConfig.Ingestion.HistorySyncMaxLookbackDays,
	)
	engine.POST("/k-candles/history", kCandleHistorySyncController.StartSymbolHistorySync)
	engine.GET("/k-candles/history/:id", kCandleHistorySyncController.GetSymbolHistorySync)

	tradingSymbolApplication := application.NewTradingSymbolApplication(
		service.NewTradingSymbolService(
			persistence.NewTradingSymbolRepository(database),
			kCandleRepository,
			symbolLookupProxyFor(applicationConfig, venuePacers),
			clock.NewSystemClockProxy(),
			domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		),
		kCandleIngestionService,
	)

	tradingSymbolController := controller.NewTradingSymbolController(tradingSymbolApplication)

	engine.GET("/trading-symbols", tradingSymbolController.ListTradingSymbols)

	engine.POST("/watchlist", tradingSymbolController.AddToWatchlist)
	engine.DELETE("/watchlist/:symbol", tradingSymbolController.RemoveFromWatchlist)

	// 永續合約從來源到儲存都不與現貨共用：同一代號在兩個場所是不同商品，共用一張表會互相覆蓋。
	contractKCandleRepository := persistence.NewKCandleContractRepository(database)
	contractTradingSymbolRepository := persistence.NewContractTradingSymbolRepository(database)
	// Built before candle ingestion, which fills in position statistics after a history sync.
	contractPositionStatisticService := service.NewContractPositionStatisticService(
		persistence.NewContractPositionStatisticRepository(database),
		contractTradingSymbolRepository,
		marketdata.NewBinanceContractPositionStatisticProxy(
			applicationConfig.ContractIngestion.StatisticsBaseUrl,
			applicationConfig.ContractIngestion.RequestTimeout,
			venuePacers.cryptoContractStatistics,
		),
		marketdata.NewBinanceContractPositionStatisticArchiveProxy(
			applicationConfig.ContractIngestion.PositionStatisticArchiveBaseUrl,
			applicationConfig.ContractIngestion.RequestTimeout,
			venuePacers.cryptoContractArchive,
		),
		clock.NewSystemClockProxy(),
		applicationConfig.KCandleQueryMaxResults,
	)
	contractKCandleIngestionService := service.NewContractKCandleIngestionService(
		contractKCandleRepository,
		persistence.NewKCandleContractHistorySyncRunRepository(database),
		contractTradingSymbolRepository,
		marketdata.NewBinanceContractMarketDataProxy(
			applicationConfig.ContractIngestion.BaseUrl,
			applicationConfig.ContractIngestion.MarkPriceUrl,
			applicationConfig.ContractIngestion.IndexPriceUrl,
			applicationConfig.ContractIngestion.PremiumIndexUrl,
			applicationConfig.ContractIngestion.RequestTimeout,
			venuePacers.cryptoContract,
		),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.ContractIngestion.RoundCandleCount,
		applicationConfig.ContractIngestion.BackfillLookback,
		contractPositionStatisticService,
	)
	kCandleContractIngestionApplication := application.NewKCandleContractIngestionApplication(
		contractKCandleIngestionService)

	contractFundingRateService := service.NewContractFundingRateService(
		persistence.NewContractFundingRateSettlementRepository(database),
		contractTradingSymbolRepository,
		marketdata.NewBinanceContractFundingRateProxy(
			applicationConfig.ContractIngestion.FundingRateUrl,
			applicationConfig.ContractIngestion.RequestTimeout,
			venuePacers.cryptoContract,
		),
		clock.NewSystemClockProxy(),
		applicationConfig.KCandleQueryMaxResults,
	)
	// The tier ladder needs account credentials; without them the proxy reports so and callers go without.
	contractMaintenanceMarginTierService := service.NewContractMaintenanceMarginTierService(
		persistence.NewContractMaintenanceMarginTierRepository(database),
		contractTradingSymbolRepository,
		marketdata.NewBinanceContractMaintenanceMarginTierProxy(
			applicationConfig.ContractIngestion.MaintenanceMarginTierUrl,
			applicationConfig.ContractIngestion.AccountApiKey,
			applicationConfig.ContractIngestion.AccountApiSecret,
			applicationConfig.ContractIngestion.RequestTimeout,
			venuePacers.cryptoContract,
			clock.NewSystemClockProxy(),
		),
		clock.NewSystemClockProxy(),
	)

	// Shared with the contract bots, which quote the newest candle as the reference price.
	kCandleContractService := service.NewKCandleContractService(
		contractKCandleRepository,
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.KCandleQueryMaxResults,
	)

	kCandleContractController := controller.NewKCandleContractController(
		application.NewKCandleContractApplication(kCandleContractService))

	engine.POST("/contract-k-candles", kCandleContractController.CreateKCandleContract)
	engine.GET("/contract-k-candles", kCandleContractController.GetKCandleContractsInRange)
	engine.GET("/contract-k-candles/series", kCandleContractController.GetKCandleContractSeries)
	engine.POST("/contract-k-candles/backfill",
		controller.NewKCandleContractBackfillController(
			kCandleContractIngestionApplication).CatchUpSymbol)

	kCandleContractHistorySyncController := controller.NewKCandleContractHistorySyncController(
		kCandleContractIngestionApplication,
		applicationConfig.ContractIngestion.HistorySyncMaxLookbackDays,
	)
	// 必須在 :symbol/:openTime 之前註冊，否則 history 會被萬用路由吃掉。
	engine.POST("/contract-k-candles/history",
		kCandleContractHistorySyncController.StartSymbolHistorySync)
	engine.GET("/contract-k-candles/history/:id",
		kCandleContractHistorySyncController.GetSymbolHistorySync)
	engine.GET("/contract-k-candles/:symbol/:openTime",
		kCandleContractController.GetKCandleContract)
	engine.PUT("/contract-k-candles/:symbol/:openTime",
		kCandleContractController.UpdateKCandleContract)
	engine.DELETE("/contract-k-candles/:symbol/:openTime",
		kCandleContractController.DeleteKCandleContract)

	contractFundingRateApplication := application.NewContractFundingRateApplication(
		contractFundingRateService)
	contractPositionStatisticApplication := application.NewContractPositionStatisticApplication(
		contractPositionStatisticService)
	engine.GET("/contract-funding-rate-settlements",
		controller.NewContractFundingRateSettlementController(
			contractFundingRateApplication).GetSettlementsInRange)
	contractMaintenanceMarginTierApplication := application.NewContractMaintenanceMarginTierApplication(
		contractMaintenanceMarginTierService)
	engine.GET("/contract-maintenance-margin-tiers",
		controller.NewContractMaintenanceMarginTierController(
			contractMaintenanceMarginTierApplication).GetTiers)
	engine.GET("/contract-position-statistics",
		controller.NewContractPositionStatisticController(
			contractPositionStatisticApplication).GetStatisticsInRange)

	contractTradingSymbolApplication := application.NewContractTradingSymbolApplication(
		service.NewContractTradingSymbolService(
			contractTradingSymbolRepository,
			contractKCandleRepository,
			marketdata.NewBinanceContractSymbolLookupProxy(
				applicationConfig.ContractIngestion.SymbolCatalogUrl,
				applicationConfig.ContractIngestion.FundingInfoUrl,
				applicationConfig.ContractIngestion.RequestTimeout,
				venuePacers.cryptoContract,
			),
			clock.NewSystemClockProxy(),
		),
		contractKCandleIngestionService,
		contractFundingRateService,
		contractPositionStatisticService,
		contractMaintenanceMarginTierService,
	)
	contractTradingSymbolController := controller.NewContractTradingSymbolController(
		contractTradingSymbolApplication)

	engine.GET("/contract-trading-symbols",
		contractTradingSymbolController.ListContractTradingSymbols)
	engine.POST("/contract-watchlist", contractTradingSymbolController.AddToWatchlist)
	engine.DELETE("/contract-watchlist/:symbol",
		contractTradingSymbolController.RemoveFromWatchlist)

	// Every script run resolves its identifier here first, which lets a person run a published
	// algorithm without ever being handed its source.
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	publishedStrategyScriptRepository := persistence.NewPublishedStrategyScriptRepository(database)

	strategyScriptService := service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository)
	strategyScriptApplication := application.NewStrategyScriptApplication(strategyScriptService)

	strategyScriptController := controller.NewStrategyScriptController(strategyScriptApplication)

	engine.POST("/strategy-scripts", requiresSignIn, strategyScriptController.CreateStrategyScript)
	engine.GET("/strategy-scripts", requiresSignIn, strategyScriptController.ListAvailableStrategyScripts)
	engine.GET("/strategy-scripts/:id", requiresSignIn, strategyScriptController.GetStrategyScript)
	engine.PUT("/strategy-scripts/:id", requiresSignIn, strategyScriptController.UpdateStrategyScript)
	engine.DELETE("/strategy-scripts/:id", requiresSignIn, strategyScriptController.DeleteStrategyScript)

	strategyScriptMarketplaceController := controller.NewStrategyScriptMarketplaceController(
		application.NewStrategyScriptMarketplaceApplication(
			service.NewStrategyScriptMarketplaceService(
				strategyScriptRepository,
				publishedStrategyScriptRepository,
				persistence.NewStrategyScriptAdoptionRepository(database),
				clock.NewSystemClockProxy(),
			),
		),
	)

	engine.POST("/strategy-scripts/:id/publication", requiresSignIn, strategyScriptMarketplaceController.PublishStrategyScript)
	engine.DELETE("/strategy-scripts/:id/publication", requiresSignIn, strategyScriptMarketplaceController.WithdrawStrategyScript)
	engine.GET("/marketplace/strategy-scripts", requiresSignIn, strategyScriptMarketplaceController.BrowseMarketplace)
	engine.POST("/marketplace/strategy-scripts/:id/adoption", requiresSignIn, strategyScriptMarketplaceController.AdoptStrategyScript)
	engine.DELETE("/marketplace/strategy-scripts/:id/adoption", requiresSignIn, strategyScriptMarketplaceController.AbandonStrategyScript)

	// Each script runs in this binary re-launched as a memory-capped worker, so a runaway script
	// kills only its compartment.
	serverExecutable, executableError := os.Executable()
	if executableError != nil {
		log.Fatalf("failed to locate the server binary for script compartments: %v", executableError)
	}
	indicatorScriptIsolation := script.IndicatorScriptIsolation{
		WorkerCommand:    []string{serverExecutable, script.IndicatorScriptWorkerCommand},
		ExecutionTimeout: applicationConfig.IndicatorScriptTimeout,
		MemoryLimitBytes: applicationConfig.IndicatorScriptMemoryLimitBytes,
	}

	// Shared with strategy bots so both use the same script runner and timeouts.
	indicatorCalculationService := service.NewIndicatorCalculationService(
		kCandleRepository,
		persistence.NewTradingSymbolRepository(database),
		script.NewYaegiIndicatorScriptProxy(indicatorScriptIsolation),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.KCandleQueryMaxResults,
	)

	contractIndicatorCalculationService := service.NewContractIndicatorCalculationService(
		contractKCandleRepository,
		persistence.NewContractFundingRateSettlementRepository(database),
		persistence.NewContractPositionStatisticRepository(database),
		script.NewYaegiContractIndicatorScriptProxy(indicatorScriptIsolation),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.KCandleQueryMaxResults,
	)

	indicatorCalculationApplication := application.NewIndicatorCalculationApplication(
		strategyScriptService,
		indicatorCalculationService,
		contractIndicatorCalculationService,
	)

	indicatorCalculationController := controller.NewIndicatorCalculationController(indicatorCalculationApplication)
	engine.POST("/indicator-calculations", requiresSignIn, indicatorCalculationController.CalculateIndicator)
	engine.POST("/contract-indicator-calculations", requiresSignIn,
		indicatorCalculationController.CalculateContractIndicator)

	// A backtest stores nothing and has its own candle ceiling and whole-run time allowance.
	backtestService := service.NewBacktestService(
		kCandleRepository,
		script.NewYaegiIndicatorScriptProxy(indicatorScriptIsolation),
		clock.NewSystemClockProxy(),
		applicationConfig.BacktestMaxCandleCount,
		applicationConfig.BacktestTimeAllowance,
	)

	contractBacktestService := service.NewContractBacktestService(
		contractKCandleRepository,
		persistence.NewContractFundingRateSettlementRepository(database),
		persistence.NewContractPositionStatisticRepository(database),
		contractTradingSymbolRepository,
		persistence.NewContractMaintenanceMarginTierRepository(database),
		script.NewYaegiContractIndicatorScriptProxy(indicatorScriptIsolation),
		clock.NewSystemClockProxy(),
		applicationConfig.BacktestMaxCandleCount,
		applicationConfig.BacktestTimeAllowance,
	)

	backtestController := controller.NewBacktestController(
		application.NewBacktestApplication(strategyScriptService, backtestService, contractBacktestService))
	engine.POST("/backtests", requiresSignIn, backtestController.RunBacktest)
	engine.POST("/contract-backtests", requiresSignIn, backtestController.RunContractBacktest)

	userController := controller.NewUserController(userApplication)

	engine.POST("/users", userController.RegisterUser)
	engine.POST("/sessions", userController.SignIn)
	// POSTs rather than DELETE because the refresh token must travel in a body.
	engine.POST("/sessions/renewal", userController.RenewSession)
	engine.POST("/sessions/revocation", userController.RevokeSession)
	// Not behind requiresSignIn: it reads the token itself and would give the same rejection anyway.
	engine.GET("/users/me", userController.GetCurrentUser)
	// Behind requiresSignIn so a bad token is not confused with a wrong current password.
	engine.POST("/users/me/password", requiresSignIn, userController.ChangePassword)

	telegramDeliveryService := service.NewTelegramDeliveryService(
		persistence.NewTelegramDeliveryRepository(database),
		security.NewAesSecretSealProxy(applicationConfig.Secrets.SealKey),
		messaging.NewTelegramMessageDeliveryProxy(
			applicationConfig.Telegram.ApiBaseUrl,
			&http.Client{Timeout: applicationConfig.Telegram.RequestTimeout},
		),
	)

	telegramDeliveryController := controller.NewTelegramDeliveryController(
		application.NewTelegramDeliveryApplication(telegramDeliveryService),
	)

	engine.GET("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.GetDeliverySetting)
	// PUT because each person has at most one setting, so it is idempotent.
	engine.PUT("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.SaveDeliverySetting)
	engine.DELETE("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.RemoveDeliverySetting)
	engine.POST("/users/me/telegram-delivery/test-message",
		requiresSignIn, telegramDeliveryController.SendTestMessage)

	// Live follow only shortens the wait for viewers; the scheduled round still stores every closed candle.
	kCandleFollowService := service.NewKCandleFollowService(
		liveMarketDataProxyFor(applicationConfig, venuePacers),
		kCandleRepository,
		persistence.NewTradingSymbolRepository(database),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.LiveFollow.UpdateIntervalCeiling,
		applicationConfig.LiveFollow.QuietTimeout,
		applicationConfig.LiveFollow.MaximumRetryDelay,
	)

	kCandleFollowApplication := application.NewKCandleFollowApplication(kCandleFollowService)

	// Reuses the spot stream proxy at the contract address; nothing is stored because a contract candle
	// needs four readings that only the ingestion round collects.
	kCandleContractFollowApplication := application.NewKCandleContractFollowApplication(
		service.NewKCandleContractFollowService(
			marketdata.NewBinanceLiveMarketDataProxy(applicationConfig.LiveFollow.ContractMarketDataStreamUrl),
			contractTradingSymbolRepository,
			clock.NewSystemClockProxy(),
			applicationConfig.LiveFollow.UpdateIntervalCeiling,
			applicationConfig.LiveFollow.QuietTimeout,
			applicationConfig.LiveFollow.MaximumRetryDelay,
		))

	kCandleFollowController := controller.NewKCandleFollowController(
		kCandleFollowApplication, kCandleContractFollowApplication)
	engine.GET("/k-candles/live", kCandleFollowController.WatchKCandles)
	engine.GET("/contract-k-candles/live", kCandleFollowController.WatchKCandleContracts)

	// One service, two applications: managing bots is permission-checked per person, while runs are
	// driven by the scheduler with no person to ask.
	strategyBotService := service.NewStrategyBotService(
		persistence.NewStrategyBotRepository(database),
		persistence.NewStrategyBotRunRecordRepository(database),
		contractTradingSymbolRepository,
		persistence.NewContractMaintenanceMarginTierRepository(database),
		persistence.NewContractFundingRateSettlementRepository(database),
		clock.NewSystemClockProxy(),
	)

	tradingStrategyService := service.NewTradingStrategyService(
		persistence.NewTradingStrategyRepository(database),
	)

	tradingStrategyApplication := application.NewTradingStrategyApplication(
		tradingStrategyService,
		strategyScriptService,
		strategyBotService,
	)

	tradingStrategyController := controller.NewTradingStrategyController(tradingStrategyApplication)

	engine.POST("/trading-strategies", requiresSignIn, tradingStrategyController.CreateTradingStrategy)
	engine.GET("/trading-strategies", requiresSignIn, tradingStrategyController.ListTradingStrategies)
	engine.GET("/trading-strategies/:id", requiresSignIn, tradingStrategyController.GetTradingStrategy)
	engine.PUT("/trading-strategies/:id", requiresSignIn, tradingStrategyController.UpdateTradingStrategy)
	engine.DELETE("/trading-strategies/:id", requiresSignIn, tradingStrategyController.DeleteTradingStrategy)
	tradingStrategyBacktestApplication := application.NewTradingStrategyBacktestApplication(
		tradingStrategyService,
		strategyScriptService,
		backtestService,
		contractBacktestService,
	)

	tradingStrategyBacktestController := controller.NewTradingStrategyBacktestController(
		tradingStrategyBacktestApplication)
	engine.POST("/trading-strategies/:id/backtests", requiresSignIn,
		tradingStrategyBacktestController.RunTradingStrategyBacktest)
	engine.POST("/trading-strategies/:id/contract-backtests", requiresSignIn,
		tradingStrategyBacktestController.RunContractTradingStrategyBacktest)

	assistantConversationApplication := application.NewAssistantConversationApplication(
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
				strategyScriptApplication,
				tradingStrategyApplication,
				tradingStrategyBacktestApplication,
				applicationConfig.Assistant.CandleLimit,
			),
			clock.NewSystemClockProxy(),
			applicationConfig.Assistant.RecentMessageLimit,
			applicationConfig.Assistant.QueryLimit,
			applicationConfig.Assistant.DailyUsageAllowance,
			applicationConfig.Assistant.AnswerLengthLimit,
		),
	)

	assistantConversationController := controller.NewAssistantConversationController(
		assistantConversationApplication)

	// Behind sign-in because the assistant acts as the caller, so every script it saves has an owner.
	engine.POST("/chat", requiresSignIn, assistantConversationController.Ask)
	engine.GET("/chat/conversations", requiresSignIn, assistantConversationController.ListConversations)
	engine.GET("/chat/conversations/:id", requiresSignIn, assistantConversationController.GetConversation)

	strategyBotRunApplication := application.NewStrategyBotRunApplication(
		strategyBotService,
		tradingStrategyService,
		strategyScriptService,
		indicatorCalculationService,
		contractIndicatorCalculationService,
		telegramDeliveryService,
		kCandleService,
		kCandleContractService,
		clock.NewSystemClockProxy(),
		application.NewStrategyBotRoundGuard(),
		applicationConfig.StrategyBot.MaxConcurrentRounds,
		applicationConfig.StrategyBot.RoundTimeout,
	)

	strategyBotController := controller.NewStrategyBotController(
		application.NewStrategyBotApplication(
			strategyBotService,
			tradingStrategyService,
			telegramDeliveryService,
		),
		strategyBotRunApplication,
	)

	engine.POST("/strategy-bots", requiresSignIn, strategyBotController.CreateStrategyBot)
	engine.GET("/strategy-bots", requiresSignIn, strategyBotController.ListStrategyBots)
	engine.GET("/strategy-bots/:id", requiresSignIn, strategyBotController.GetStrategyBot)
	engine.PUT("/strategy-bots/:id", requiresSignIn, strategyBotController.UpdateStrategyBot)
	engine.DELETE("/strategy-bots/:id", requiresSignIn, strategyBotController.DeleteStrategyBot)
	// Power is an idempotent subresource; named /power rather than /run to avoid confusion with /runs.
	engine.POST("/strategy-bots/:id/power", requiresSignIn, strategyBotController.StartStrategyBot)
	engine.DELETE("/strategy-bots/:id/power", requiresSignIn, strategyBotController.StopStrategyBot)
	engine.GET("/strategy-bots/:id/runs", requiresSignIn, strategyBotController.ListRunRecords)
	// 立刻跑一輪，與排程那一輪走完全同一條路。
	engine.POST("/strategy-bots/:id/runs", requiresSignIn, strategyBotController.RunRoundNow)

	return liveFollowApplications{spot: kCandleFollowApplication, contract: kCandleContractFollowApplication},
		kCandleIngestionApplication,
		kCandleContractIngestionApplication, strategyBotRunApplication,
		assistantConversationApplication,
		contractSeriesApplications{
			fundingRate:       contractFundingRateApplication,
			positionStatistic: contractPositionStatisticApplication,
			tradingSymbol:     contractTradingSymbolApplication,
			maintenanceMargin: contractMaintenanceMarginTierApplication,
		}
}

type liveFollowApplications struct {
	spot     *application.KCandleFollowApplication
	contract *application.KCandleContractFollowApplication
}

func (liveFollowApplications liveFollowApplications) Stop() {
	liveFollowApplications.spot.Stop()
	liveFollowApplications.contract.Stop()
}

// contractSeriesApplications groups the contract use cases that run their own background rounds.
type contractSeriesApplications struct {
	fundingRate       *application.ContractFundingRateApplication
	positionStatistic *application.ContractPositionStatisticApplication
	tradingSymbol     *application.ContractTradingSymbolApplication
	maintenanceMargin *application.ContractMaintenanceMarginTierApplication
}

// assistantQueriesFor is the complete list of what the assistant may do; deleting and bot control are
// absent by design, and each query calls the same use case a person does.
func assistantQueriesFor(
	tradingSymbolApplication *application.TradingSymbolApplication,
	kCandleApplication *application.KCandleApplication,
	indicatorCalculationApplication *application.IndicatorCalculationApplication,
	strategyScriptApplication *application.StrategyScriptApplication,
	tradingStrategyApplication *application.TradingStrategyApplication,
	tradingStrategyBacktestApplication *application.TradingStrategyBacktestApplication,
	candleLimit int,
) []domaininterface.IAssistantQuery {
	return []domaininterface.IAssistantQuery{
		assistantqueries.NewTradingSymbolListAssistantQuery(tradingSymbolApplication),
		assistantqueries.NewKCandleSeriesAssistantQuery(kCandleApplication, candleLimit),
		assistantqueries.NewKCandleRangeAssistantQuery(kCandleApplication, candleLimit),
		assistantqueries.NewIndicatorCalculationAssistantQuery(indicatorCalculationApplication),
		assistantqueries.NewStrategyScriptListAssistantQuery(strategyScriptApplication),
		assistantqueries.NewStrategyScriptGetAssistantQuery(strategyScriptApplication),
		assistantqueries.NewStrategyScriptCreateAssistantQuery(strategyScriptApplication),
		assistantqueries.NewStrategyScriptUpdateAssistantQuery(strategyScriptApplication),
		assistantqueries.NewTradingStrategyListAssistantQuery(tradingStrategyApplication),
		assistantqueries.NewTradingStrategyGetAssistantQuery(tradingStrategyApplication),
		assistantqueries.NewTradingStrategyCreateAssistantQuery(tradingStrategyApplication),
		assistantqueries.NewTradingStrategyUpdateAssistantQuery(tradingStrategyApplication),
		assistantqueries.NewTradingStrategyBacktestAssistantQuery(tradingStrategyBacktestApplication),
	}
}

func backgroundJobsFor(
	applicationConfig config.ApplicationConfig,
	kCandleFollowApplication *application.KCandleFollowApplication,
	kCandleIngestionApplication *application.KCandleIngestionApplication,
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication,
	strategyBotRunApplication *application.StrategyBotRunApplication,
	contractSeries contractSeriesApplications,
) []domaininterface.IBackgroundJob {
	if !applicationConfig.BackgroundJobsEnabled {
		return []domaininterface.IBackgroundJob{}
	}

	kCandleIngestionJob := job.NewKCandleIngestionJob(
		kCandleIngestionApplication, job.KCandleIngestionInterval)

	// Separate from the spot round so one unresponsive venue cannot hold up the other.
	contractKCandleIngestionJob := job.NewContractKCandleIngestionJob(
		kCandleContractIngestionApplication, job.KCandleIngestionInterval)

	// Its own job so a stalled ingestion round cannot delay a market that has just opened.
	liveFollowRosterJob := job.NewLiveFollowRosterJob(
		kCandleFollowApplication, job.LiveFollowRosterInterval)

	// One scan over stored bot state rather than a goroutine per bot, so a restart loses at most one interval.
	strategyBotScanJob := job.NewStrategyBotScanJob(
		strategyBotRunApplication, applicationConfig.StrategyBot.ScanInterval)

	backgroundJobs := []domaininterface.IBackgroundJob{
		kCandleIngestionJob, contractKCandleIngestionJob, liveFollowRosterJob, strategyBotScanJob,
	}

	// Each contract series is its own job so a slow one cannot hold up the others; each can be switched off alone.
	contractIngestion := applicationConfig.ContractIngestion
	if contractIngestion.FundingRateIngestionInterval > 0 {
		backgroundJobs = append(backgroundJobs, job.NewContractFundingRateIngestionJob(
			contractSeries.fundingRate, contractIngestion.FundingRateIngestionInterval))
	}
	if contractIngestion.PositionStatisticIngestionInterval > 0 {
		backgroundJobs = append(backgroundJobs, job.NewContractPositionStatisticIngestionJob(
			contractSeries.positionStatistic, contractIngestion.PositionStatisticIngestionInterval))
	}
	if contractIngestion.TradingSpecificationRefreshInterval > 0 {
		backgroundJobs = append(backgroundJobs, job.NewContractTradingSpecificationRefreshJob(
			contractSeries.tradingSymbol, contractIngestion.TradingSpecificationRefreshInterval))
	}

	// Only with account credentials; without them the round could ask nothing.
	if contractIngestion.MaintenanceMarginTierRefreshInterval > 0 && contractIngestion.HasAccountCredentials() {
		backgroundJobs = append(backgroundJobs, job.NewContractMaintenanceMarginTierRefreshJob(
			contractSeries.maintenanceMargin, contractIngestion.MaintenanceMarginTierRefreshInterval))
	}

	return backgroundJobs
}

// venuePacers holds one pacer per venue allowance, shared by every proxy that spends it, because the
// venue counts requests per host rather than per kind of question.
type venuePacers struct {
	crypto marketdata.RequestPacer
	// cryptoContract is separate from spot because the two venues count allowances apart.
	cryptoContract marketdata.RequestPacer
	// cryptoContractStatistics is the venue's separate allowance for position statistics.
	cryptoContractStatistics marketdata.RequestPacer
	// cryptoContractArchive is a separate file host with its own allowance.
	cryptoContractArchive marketdata.RequestPacer
	taiwanStock           marketdata.RequestPacer
}

func newVenuePacers(applicationConfig config.ApplicationConfig) venuePacers {
	return venuePacers{
		crypto: marketdata.NewRequestPacer(
			applicationConfig.Ingestion.MarketDataRequestsPerMinute),
		cryptoContract: marketdata.NewRequestPacer(
			applicationConfig.ContractIngestion.RequestsPerMinute),
		cryptoContractStatistics: marketdata.NewRequestPacer(
			applicationConfig.ContractIngestion.StatisticsRequestsPerMinute),
		cryptoContractArchive: marketdata.NewRequestPacer(
			applicationConfig.ContractIngestion.PositionStatisticArchiveRequestsPerMinute),
		taiwanStock: marketdata.NewRequestPacer(
			applicationConfig.TaiwanStock.RequestsPerMinute),
	}
}

func marketDataProxyFor(
	applicationConfig config.ApplicationConfig, venuePacers venuePacers,
) domaininterface.IMarketDataProxy {
	return marketdata.NewMarketRoutedMarketDataProxy(
		map[vo.MarketVo]domaininterface.IMarketDataProxy{
			vo.MarketCrypto: marketdata.NewBinanceMarketDataProxy(
				applicationConfig.Ingestion.MarketDataBaseUrl,
				applicationConfig.Ingestion.MarketDataRequestTimeout,
				venuePacers.crypto,
			),
			vo.MarketTaiwanStock: marketdata.NewFugleMarketDataProxy(
				applicationConfig.TaiwanStock.IntradayCandlesUrl,
				applicationConfig.TaiwanStock.HistoricalCandlesUrl,
				applicationConfig.TaiwanStock.ApiKey,
				// 給市場本身而不是時段，讓它能跳過窗口中間的休市日。
				domains.NewMarketCatalogDomain(applicationConfig.MarketRules).
					MarketOf(string(vo.MarketTaiwanStock)),
				clock.NewSystemClockProxy(),
				applicationConfig.TaiwanStock.RequestTimeout,
				venuePacers.taiwanStock,
			),
		})
}

func liveMarketDataProxyFor(
	applicationConfig config.ApplicationConfig, venuePacers venuePacers,
) domaininterface.ILiveMarketDataProxy {
	return marketdata.NewMarketRoutedLiveMarketDataProxy(
		map[vo.MarketVo]domaininterface.ILiveMarketDataProxy{
			vo.MarketCrypto: marketdata.NewBinanceLiveMarketDataProxy(
				applicationConfig.LiveFollow.MarketDataStreamUrl),
			// Same venue and pacer as stored history, so live and stored figures agree and share one allowance.
			vo.MarketTaiwanStock: marketdata.NewFugleIntradayLiveMarketDataProxy(
				applicationConfig.TaiwanStock.IntradayCandlesUrl,
				applicationConfig.TaiwanStock.ApiKey,
				applicationConfig.TaiwanStock.LiveCandleInterval,
				applicationConfig.TaiwanStock.RequestsPerMinute,
				applicationConfig.LiveFollow.QuietTimeout,
				applicationConfig.TaiwanStock.RequestTimeout,
				venuePacers.taiwanStock),
		})
}

func symbolLookupProxyFor(
	applicationConfig config.ApplicationConfig, venuePacers venuePacers,
) domaininterface.ISymbolLookupProxy {
	return marketdata.NewMarketRoutedSymbolLookupProxy(
		map[vo.MarketVo]domaininterface.ISymbolLookupProxy{
			vo.MarketCrypto: marketdata.NewBinanceSymbolLookupProxy(
				applicationConfig.Ingestion.SymbolCatalogUrl,
				applicationConfig.Ingestion.MarketDataRequestTimeout,
				venuePacers.crypto),
			vo.MarketTaiwanStock: marketdata.NewFugleSymbolLookupProxy(
				applicationConfig.TaiwanStock.TickerUrl,
				applicationConfig.TaiwanStock.ApiKey,
				applicationConfig.TaiwanStock.RequestTimeout,
				venuePacers.taiwanStock),
		})
}
