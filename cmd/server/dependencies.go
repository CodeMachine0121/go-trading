package main

import (
	"net/http"

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
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerRoutes is the composition root: it wires every concrete type and mounts
// the routes. It hands back the two things a background job and a route both reach
// for, so that there is exactly one of each: the live follows, which outlive the
// request that started them, and the ingestion, which remembers which markets it has
// decided are shut today — a memory that would be two different memories if the job
// and the routes each built their own.
func registerRoutes(
	engine *gin.Engine, database *gorm.DB, applicationConfig config.ApplicationConfig,
) (*application.KCandleFollowApplication, *application.KCandleIngestionApplication) {
	engine.Use(middlewares.NewCorsMiddleware(applicationConfig.CorsAllowedOrigins).Handle)

	engine.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "Healthy"})
	})

	// Recognising a person is built first because everything that belongs to
	// somebody is built behind it. It is wired from two capabilities that are pure
	// cryptography: turning a password into something storable, and turning an
	// identity into something signed. Both are behind interfaces, so replacing
	// either — bcrypt for something newer, one shared key for a key pair — is a new
	// implementation and one changed line here.
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
		),
	)

	// The door. Everything mounted through it carries an identified user; everything
	// mounted beside it is open to anybody who can reach this address.
	//
	// What is open is deliberate rather than overlooked. Creating the first user and
	// signing in cannot require being signed in. The market — candles, symbols, the
	// watchlist — is nobody's property: putting a door on it would only make a chart
	// blank for a visitor, and there is nothing behind it to protect. What is closed
	// is everything that belongs to a person: their strategies, the marketplace,
	// running one, and the assistant that acts as them.
	requiresSignIn := middlewares.NewAuthenticationMiddleware(userApplication).Handle

	kCandleRepository := persistence.NewKCandleRepository(database)

	kCandleApplication := application.NewKCandleApplication(
		service.NewKCandleService(
			kCandleRepository,
			persistence.NewTradingSymbolRepository(database),
			clock.NewSystemClockProxy(),
			domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
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

	// 抓取在這裡組起來而不是在背景工作那邊：加入觀察清單要立刻補齊那一檔，手動補齊也是
	// 一條路由，兩者都不該等背景工作被打開才存在——而它們必須跟背景工作共用同一份，
	// 否則「今天休市」會各記各的。
	kCandleIngestionService := service.NewKCandleIngestionService(
		kCandleRepository,
		persistence.NewTradingSymbolRepository(database),
		marketDataProxyFor(applicationConfig),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.Ingestion.RoundCandleCount,
		applicationConfig.Ingestion.BackfillLookback,
	)
	kCandleIngestionApplication := application.NewKCandleIngestionApplication(kCandleIngestionService)

	engine.POST("/k-candles/backfill",
		controller.NewKCandleBackfillController(kCandleIngestionApplication).CatchUpSymbol)

	// 交易標的是另一個資源（系統認得哪幾個市場），不是某一根 K 線，所以有自己的 controller 與路徑。
	tradingSymbolApplication := application.NewTradingSymbolApplication(
		service.NewTradingSymbolService(
			persistence.NewTradingSymbolRepository(database),
			kCandleRepository,
			symbolLookupProxyFor(applicationConfig),
			clock.NewSystemClockProxy(),
			domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		),
		kCandleIngestionService,
	)

	tradingSymbolController := controller.NewTradingSymbolController(tradingSymbolApplication)

	engine.GET("/trading-symbols", tradingSymbolController.ListTradingSymbols)

	// 觀察清單是自己的資源（系統打算持續追蹤哪幾個市場），與「系統認得哪幾個」是兩件事，
	// 所以有自己的路徑。
	engine.POST("/watchlist", tradingSymbolController.AddToWatchlist)
	engine.DELETE("/watchlist/:symbol", tradingSymbolController.RemoveFromWatchlist)

	// A saved strategy is its own resource: it holds an algorithm, who it belongs
	// to, and nothing else — how coarse the K candles are, how many of them and up
	// to when describe one run and travel with the calculation instead. It reads no
	// K candles, so it is given no K candle repository.
	//
	// It is built before the two use cases that run a strategy, because both of them
	// resolve an identifier through it first. That resolution is the only way a
	// script leaves storage, and it is what lets one person run another's published
	// algorithm without ever being handed it.
	strategyRepository := persistence.NewStrategyRepository(database)
	publishedStrategyRepository := persistence.NewPublishedStrategyRepository(database)

	strategyService := service.NewStrategyService(strategyRepository, publishedStrategyRepository)
	strategyApplication := application.NewStrategyApplication(strategyService)

	strategyController := controller.NewStrategyController(strategyApplication)

	engine.POST("/strategies", requiresSignIn, strategyController.CreateStrategy)
	engine.GET("/strategies", requiresSignIn, strategyController.ListAvailableStrategies)
	engine.GET("/strategies/:id", requiresSignIn, strategyController.GetStrategy)
	engine.PUT("/strategies/:id", requiresSignIn, strategyController.UpdateStrategy)
	engine.DELETE("/strategies/:id", requiresSignIn, strategyController.DeleteStrategy)

	// The marketplace is the same rows read a different way, and a separate resource
	// because it answers a different question: not "what is mine" but "what is out
	// there". Publishing hangs off the strategy's own path because it is something
	// done to a strategy; browsing and adopting hang off the marketplace because
	// they are things done to the shelf.
	strategyMarketplaceController := controller.NewStrategyMarketplaceController(
		application.NewStrategyMarketplaceApplication(
			service.NewStrategyMarketplaceService(
				strategyRepository,
				publishedStrategyRepository,
				persistence.NewStrategyAdoptionRepository(database),
				clock.NewSystemClockProxy(),
			),
		),
	)

	engine.POST("/strategies/:id/publication", requiresSignIn, strategyMarketplaceController.PublishStrategy)
	engine.DELETE("/strategies/:id/publication", requiresSignIn, strategyMarketplaceController.WithdrawStrategy)
	engine.GET("/marketplace/strategies", requiresSignIn, strategyMarketplaceController.BrowseMarketplace)
	engine.POST("/marketplace/strategies/:id/adoption", requiresSignIn, strategyMarketplaceController.AdoptStrategy)
	engine.DELETE("/marketplace/strategies/:id/adoption", requiresSignIn, strategyMarketplaceController.AbandonStrategy)

	indicatorCalculationApplication := application.NewIndicatorCalculationApplication(
		strategyService,
		service.NewIndicatorCalculationService(
			kCandleRepository,
			persistence.NewTradingSymbolRepository(database),
			script.NewYaegiIndicatorScriptProxy(applicationConfig.IndicatorScriptTimeout),
			clock.NewSystemClockProxy(),
			domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
			applicationConfig.KCandleQueryMaxResults,
		),
	)

	engine.POST("/indicator-calculations", requiresSignIn, controller.NewIndicatorCalculationController(
		indicatorCalculationApplication).CalculateIndicator)

	// Replaying a strategy is its own use case rather than a mode of calculating an
	// indicator: it asks a different question of the same script, and it stores
	// nothing — which is why it is given no repository beyond the one it reads from.
	//
	// It shares the read ceiling with every other read, deliberately: a replay is
	// still one look at the market, and giving it a ceiling of its own would leave two
	// numbers to keep in step.
	engine.POST("/backtests", requiresSignIn, controller.NewBacktestController(
		application.NewBacktestApplication(
			strategyService,
			service.NewBacktestService(
				kCandleRepository,
				script.NewYaegiIndicatorScriptProxy(applicationConfig.IndicatorScriptTimeout),
				clock.NewSystemClockProxy(),
				applicationConfig.KCandleQueryMaxResults,
			),
		)).RunBacktest)

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

	// The assistant acts as whoever asked it, so it is behind the door like anything
	// else that touches a strategy. Without that, a strategy it saved would belong
	// to nobody, and "every strategy has an owner" would have its one exception.
	engine.POST("/chat", requiresSignIn, assistantConversationController.Ask)
	engine.GET("/chat/conversations", requiresSignIn, assistantConversationController.ListConversations)
	engine.GET("/chat/conversations/:id", requiresSignIn, assistantConversationController.GetConversation)

	// Creating a user and signing in are open, and have to be: a system holding no
	// users has nobody who could be allowed to create the first one. "Who am I" is
	// the one route here that reads the proof through the same door as everything
	// else.
	userController := controller.NewUserController(userApplication)

	engine.POST("/users", userController.RegisterUser)
	engine.POST("/sessions", userController.SignIn)
	// Renewing and ending are POSTs rather than one body-carrying DELETE: which
	// session is meant is named by the renewal proof, and a proof can only travel in
	// a body.
	engine.POST("/sessions/renewal", userController.RenewSession)
	engine.POST("/sessions/revocation", userController.RevokeSession)
	engine.GET("/users/me", requiresSignIn, userController.GetCurrentUser)

	// Following a market live is an addition, not a replacement: the scheduled
	// round keeps running, and it is what fills in every candle that closed while
	// nobody was looking. This path only shortens the wait for whoever is looking.
	kCandleFollowService := service.NewKCandleFollowService(
		liveMarketDataProxyFor(applicationConfig),
		kCandleRepository,
		persistence.NewTradingSymbolRepository(database),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.LiveFollow.UpdateIntervalCeiling,
		applicationConfig.LiveFollow.QuietTimeout,
		applicationConfig.LiveFollow.MaximumRetryDelay,
	)

	kCandleFollowApplication := application.NewKCandleFollowApplication(kCandleFollowService)

	engine.GET("/k-candles/live", controller.NewKCandleFollowController(
		kCandleFollowApplication).WatchKCandles)

	return kCandleFollowApplication, kCandleIngestionApplication
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
		assistantqueries.NewTradingSymbolListAssistantQuery(tradingSymbolApplication),
		assistantqueries.NewKCandleSeriesAssistantQuery(kCandleApplication, candleLimit),
		assistantqueries.NewKCandleRangeAssistantQuery(kCandleApplication, candleLimit),
		assistantqueries.NewIndicatorCalculationAssistantQuery(indicatorCalculationApplication),
		assistantqueries.NewStrategyListAssistantQuery(strategyApplication),
		assistantqueries.NewStrategyGetAssistantQuery(strategyApplication),
		assistantqueries.NewStrategyCreateAssistantQuery(strategyApplication),
		assistantqueries.NewStrategyUpdateAssistantQuery(strategyApplication),
	}
}

// backgroundJobsFor assembles the work the system does on its own. Switching
// background jobs off leaves nothing to start; an empty watchlist means every round
// has nothing to fetch, which is a state rather than a failure.
func backgroundJobsFor(
	applicationConfig config.ApplicationConfig,
	kCandleFollowApplication *application.KCandleFollowApplication,
	kCandleIngestionApplication *application.KCandleIngestionApplication,
) []domaininterface.IBackgroundJob {
	if !applicationConfig.BackgroundJobsEnabled {
		return []domaininterface.IBackgroundJob{}
	}

	kCandleIngestionJob := job.NewKCandleIngestionJob(
		kCandleIngestionApplication, job.KCandleIngestionInterval)

	// Handing out a market's live places is its own job rather than another step of
	// a round: a round held up by a source that will not answer would otherwise hold
	// up a market that has just opened.
	liveFollowRosterJob := job.NewLiveFollowRosterJob(
		kCandleFollowApplication, job.LiveFollowRosterInterval)

	return []domaininterface.IBackgroundJob{kCandleIngestionJob, liveFollowRosterJob}
}

// marketDataProxyFor is where every market's candle source is named, and the only
// place they are all named together.
//
// Recognising a third market is one more entry in each of these three, plus its
// rules in the settings. Nothing else in the system changes: everything above these
// asks for a window of candles and never learns which venue answered.
func marketDataProxyFor(
	applicationConfig config.ApplicationConfig,
) domaininterface.IMarketDataProxy {
	return marketdata.NewMarketRoutedMarketDataProxy(
		map[vo.MarketVo]domaininterface.IMarketDataProxy{
			vo.MarketCrypto: marketdata.NewBinanceMarketDataProxy(
				applicationConfig.Ingestion.MarketDataBaseUrl,
				applicationConfig.Ingestion.MarketDataRequestTimeout,
			),
			vo.MarketTaiwanStock: marketdata.NewFugleMarketDataProxy(
				applicationConfig.TaiwanStock.IntradayCandlesUrl,
				applicationConfig.TaiwanStock.HistoricalCandlesUrl,
				applicationConfig.TaiwanStock.ApiKey,
				applicationConfig.TaiwanStock.TimeZone,
				clock.NewSystemClockProxy(),
				applicationConfig.TaiwanStock.RequestTimeout,
			),
		})
}

// liveMarketDataProxyFor is where every market's live feed is named.
func liveMarketDataProxyFor(
	applicationConfig config.ApplicationConfig,
) domaininterface.ILiveMarketDataProxy {
	return marketdata.NewMarketRoutedLiveMarketDataProxy(
		map[vo.MarketVo]domaininterface.ILiveMarketDataProxy{
			vo.MarketCrypto: marketdata.NewBinanceLiveMarketDataProxy(
				applicationConfig.LiveFollow.MarketDataStreamUrl),
			vo.MarketTaiwanStock: marketdata.NewFugleLiveMarketDataProxy(
				applicationConfig.TaiwanStock.StreamUrl,
				applicationConfig.TaiwanStock.ApiKey,
				applicationConfig.TaiwanStock.RequestTimeout),
		})
}

// symbolLookupProxyFor is where every market is asked whether it has heard of a code.
func symbolLookupProxyFor(
	applicationConfig config.ApplicationConfig,
) domaininterface.ISymbolLookupProxy {
	return marketdata.NewMarketRoutedSymbolLookupProxy(
		map[vo.MarketVo]domaininterface.ISymbolLookupProxy{
			vo.MarketCrypto: marketdata.NewBinanceSymbolLookupProxy(
				applicationConfig.Ingestion.SymbolCatalogUrl,
				applicationConfig.Ingestion.MarketDataRequestTimeout),
			vo.MarketTaiwanStock: marketdata.NewFugleSymbolLookupProxy(
				applicationConfig.TaiwanStock.TickerUrl,
				applicationConfig.TaiwanStock.ApiKey,
				applicationConfig.TaiwanStock.RequestTimeout),
		})
}
