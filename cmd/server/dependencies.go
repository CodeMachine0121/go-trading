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
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/messaging"
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
) (
	*application.KCandleFollowApplication,
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

	// The door. Everything mounted through it carries an identified user; everything
	// mounted beside it is open to anybody who can reach this address.
	//
	// What is open is deliberate rather than overlooked. Creating the first user and
	// signing in cannot require being signed in. The market — candles, symbols, the
	// watchlist — is nobody's property: putting a door on it would only make a chart
	// blank for a visitor, and there is nothing behind it to protect. What is closed
	// is everything that belongs to a person: their strategy scripts, the marketplace,
	// running one, and the assistant that acts as them.
	requiresSignIn := middlewares.NewAuthenticationMiddleware(userApplication).Handle

	kCandleRepository := persistence.NewKCandleRepository(database)

	// Built once and shared, because a strategy bot quoting a reference price is
	// asking the same question of the market as the chart is. Two instances would be
	// two read ceilings, and the one a bot used would be the one nobody tuned.
	// 每個來源一個節奏，所有打到那個來源的 proxy 共用——額度是照來源算的，
	// 不是照問什麼問題算的。
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

	// 抓取在這裡組起來而不是在背景工作那邊：加入觀察清單要立刻補齊那一檔，手動補齊也是
	// 一條路由，兩者都不該等背景工作被打開才存在——而它們必須跟背景工作共用同一份，
	// 否則「今天休市」會各記各的。
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

	// 補缺口與同步一段歷史是兩條路，因為它們對「要回溯多久」的答案相反：
	// 前者由系統決定（那句話寫在它的請求物件上當理由），後者由要求的人說。
	// 在前者身上加一個選填欄位，會讓那句話變成半真的。
	//
	// 它先回 202 與一筆輪次，而不是等抓完才回：四年的一分鐘 K 線是幾千次照節奏發出的
	// 請求，沒有哪一條連線值得開那麼久。抓取由活得比這個請求久的東西推動，
	// 所以下面這條查詢路由才是它真正的答案。
	kCandleHistorySyncController := controller.NewKCandleHistorySyncController(
		kCandleIngestionApplication,
		applicationConfig.Ingestion.HistorySyncMaxLookbackDays,
	)
	engine.POST("/k-candles/history", kCandleHistorySyncController.StartSymbolHistorySync)
	engine.GET("/k-candles/history/:id", kCandleHistorySyncController.GetSymbolHistorySync)

	// 交易標的是另一個資源（系統認得哪幾個市場），不是某一根 K 線，所以有自己的 controller 與路徑。
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

	// 觀察清單是自己的資源（系統打算持續追蹤哪幾個市場），與「系統認得哪幾個」是兩件事，
	// 所以有自己的路徑。
	engine.POST("/watchlist", tradingSymbolController.AddToWatchlist)
	engine.DELETE("/watchlist/:symbol", tradingSymbolController.RemoveFromWatchlist)

	// 永續合約自成一條路徑,從來源到儲存都不與現貨共用。
	//
	// 兩件事逼出這個決定。一是同一個代號在兩個場所是兩種不同的商品,而 K 線以
	// 「交易標的 ＋ 起始時間」唯一——共用一張表,每分鐘那一輪會安靜地互相覆蓋。
	// 二是合約 K 線帶著標記價格,而現貨**沒有這個概念**:那不是某個市場不提供的
	// 一項數字,是一件在現貨那邊不存在的事。
	//
	// 它與抓取共用同一個 service 實例,理由與現貨那邊一字不差:加入合約追蹤名單要
	// 立刻補齊那一檔,手動補齊也是一條路由,兩者都不該等背景工作被打開才存在。
	contractKCandleRepository := persistence.NewKCandleContractRepository(database)
	contractTradingSymbolRepository := persistence.NewContractTradingSymbolRepository(database)
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
	)
	kCandleContractIngestionApplication := application.NewKCandleContractIngestionApplication(
		contractKCandleIngestionService)

	// Funding rate settlements and position statistics are caught up by the same
	// service instances the rounds use, for the reason the candles are: joining the
	// watchlist catches a contract up at once, and that must not wait for background
	// work to be switched on.
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
	contractPositionStatisticService := service.NewContractPositionStatisticService(
		persistence.NewContractPositionStatisticRepository(database),
		contractTradingSymbolRepository,
		marketdata.NewBinanceContractPositionStatisticProxy(
			applicationConfig.ContractIngestion.StatisticsBaseUrl,
			applicationConfig.ContractIngestion.RequestTimeout,
			venuePacers.cryptoContractStatistics,
		),
		clock.NewSystemClockProxy(),
		applicationConfig.KCandleQueryMaxResults,
	)

	kCandleContractController := controller.NewKCandleContractController(
		application.NewKCandleContractApplication(service.NewKCandleContractService(
			contractKCandleRepository,
			clock.NewSystemClockProxy(),
			applicationConfig.KCandleQueryMaxResults,
		)))

	engine.POST("/contract-k-candles", kCandleContractController.CreateKCandleContract)
	engine.GET("/contract-k-candles", kCandleContractController.GetKCandleContractsInRange)
	engine.POST("/contract-k-candles/backfill",
		controller.NewKCandleContractBackfillController(
			kCandleContractIngestionApplication).CatchUpSymbol)

	kCandleContractHistorySyncController := controller.NewKCandleContractHistorySyncController(
		kCandleContractIngestionApplication,
		applicationConfig.ContractIngestion.HistorySyncMaxLookbackDays,
	)
	// 這兩條掛在 :symbol/:openTime 之前,因為 history 與一個代號在路由樹上是同一層,
	// 先註冊具體的那一條才不會被萬用的那一條吃掉。
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
	)
	contractTradingSymbolController := controller.NewContractTradingSymbolController(
		contractTradingSymbolApplication)

	engine.GET("/contract-trading-symbols",
		contractTradingSymbolController.ListContractTradingSymbols)
	engine.POST("/contract-watchlist", contractTradingSymbolController.AddToWatchlist)
	engine.DELETE("/contract-watchlist/:symbol",
		contractTradingSymbolController.RemoveFromWatchlist)

	// A saved strategy script is its own resource: it holds an algorithm, who it belongs
	// to, and nothing else — how coarse the K candles are, how many of them and up
	// to when describe one run and travel with the calculation instead. It reads no
	// K candles, so it is given no K candle repository.
	//
	// It is built before the two use cases that run a strategy script, because both of them
	// resolve an identifier through it first. That resolution is the only way a
	// script leaves storage, and it is what lets one person run another's published
	// algorithm without ever being handed it.
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

	// The marketplace is the same rows read a different way, and a separate resource
	// because it answers a different question: not "what is mine" but "what is out
	// there". Publishing hangs off the strategy script's own path because it is something
	// done to a strategy script; browsing and adopting hang off the marketplace because
	// they are things done to the shelf.
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

	// Built once and shared, because a strategy bot asks exactly the same question
	// of it as somebody sitting at the screen does. Two instances would be two
	// script runners with two timeouts, and the one a bot used would be the one
	// nobody ever tuned.
	indicatorCalculationService := service.NewIndicatorCalculationService(
		kCandleRepository,
		persistence.NewTradingSymbolRepository(database),
		script.NewYaegiIndicatorScriptProxy(applicationConfig.IndicatorScriptTimeout),
		clock.NewSystemClockProxy(),
		domains.NewMarketCatalogDomain(applicationConfig.MarketRules),
		applicationConfig.KCandleQueryMaxResults,
	)

	indicatorCalculationApplication := application.NewIndicatorCalculationApplication(
		strategyScriptService,
		indicatorCalculationService,
	)

	engine.POST("/indicator-calculations", requiresSignIn, controller.NewIndicatorCalculationController(
		indicatorCalculationApplication).CalculateIndicator)

	// Replaying a strategy script is its own use case rather than a mode of calculating an
	// indicator: it asks a different question of the same script, and it stores
	// nothing — which is why it is given no repository beyond the one it reads from.
	//
	// It shares the read ceiling with every other read, deliberately: a replay is
	// still one look at the market, and giving it a ceiling of its own would leave two
	// numbers to keep in step.
	// Built once and shared by both kinds of replay. Two of these would be two read
	// ceilings and two clocks, and the day they drifted apart the same stretch of
	// market would replay differently depending on which subject was named.
	backtestService := service.NewBacktestService(
		kCandleRepository,
		script.NewYaegiIndicatorScriptProxy(applicationConfig.IndicatorScriptTimeout),
		clock.NewSystemClockProxy(),
		applicationConfig.KCandleQueryMaxResults,
	)

	engine.POST("/backtests", requiresSignIn, controller.NewBacktestController(
		application.NewBacktestApplication(strategyScriptService, backtestService)).RunBacktest)

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
	// "Who am I" reads the proof itself rather than sitting behind the door, and
	// that is not an oversight: the door answers a rejected proof with the same
	// sentence this route would, so putting one in front of the other would only
	// mean reading the header twice to reach the same answer.
	engine.GET("/users/me", userController.GetCurrentUser)
	// Changing a password does sit behind the door, unlike "who am I" above. The
	// difference is what the two do with a rejected proof: reading who you are is
	// the same refusal either way, whereas this one has a second refusal of its own
	// ("that is not your current password") that must not be confused with the
	// first — and the door is what keeps them apart.
	engine.POST("/users/me/password", requiresSignIn, userController.ChangePassword)

	// Where this system speaks to somebody. It is the first thing here that talks
	// without being asked, so it is wired from two capabilities named for what they
	// do rather than for who does them: locking a secret away, and delivering a
	// message. Telegram is today's only carrier; a second one is a second
	// implementation and one changed line here.
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
	// PUT rather than POST: there is at most one of these per person, and sending
	// it twice leaves the same single setting behind.
	engine.PUT("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.SaveDeliverySetting)
	engine.DELETE("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.RemoveDeliverySetting)
	engine.POST("/users/me/telegram-delivery/test-message",
		requiresSignIn, telegramDeliveryController.SendTestMessage)

	// Following a market live is an addition, not a replacement: the scheduled
	// round keeps running, and it is what fills in every candle that closed while
	// nobody was looking. This path only shortens the wait for whoever is looking.
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

	engine.GET("/k-candles/live", controller.NewKCandleFollowController(
		kCandleFollowApplication).WatchKCandles)

	// Standing bots: the first thing here that both decides something and says it
	// without anybody asking. They are wired last because they lean on almost
	// everything above — the strategy script gates, the script runner, the candles and the
	// way out to Telegram — and add only one store of their own.
	//
	// The run side and the managing side share one service but are two
	// applications, because they answer to different callers. One is a person
	// pressing a button and is refused when they may not; the other is a clock, and
	// has no person whose permission could be asked.
	strategyBotService := service.NewStrategyBotService(
		persistence.NewStrategyBotRepository(database),
		persistence.NewStrategyBotRunRecordRepository(database),
		clock.NewSystemClockProxy(),
	)

	// The rules are their own thing, built before the bots that follow them and
	// shared by every one of them. Nothing here knows how often anything wakes up.
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
	)

	// 重演是對某一份交易策略做的事，所以掛在它底下——與機器人的輪次同一個形狀。
	engine.POST("/trading-strategies/:id/backtests", requiresSignIn,
		controller.NewTradingStrategyBacktestController(
			tradingStrategyBacktestApplication).RunTradingStrategyBacktest)

	// The assistant is wired after the trading strategies rather than before,
	// because it is now handed them: it assembles a set of rules out of the scripts
	// it writes and replays it to see what it would have done. Everything it reaches
	// for has to exist by the time this list is built.
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

	// The assistant acts as whoever asked it, so it is behind the door like anything
	// else that touches a strategy script. Without that, a strategy script it saved would belong
	// to nobody, and "every strategy script has an owner" would have its one exception.
	engine.POST("/chat", requiresSignIn, assistantConversationController.Ask)
	engine.GET("/chat/conversations", requiresSignIn, assistantConversationController.ListConversations)
	engine.GET("/chat/conversations/:id", requiresSignIn, assistantConversationController.GetConversation)

	strategyBotRunApplication := application.NewStrategyBotRunApplication(
		strategyBotService,
		tradingStrategyService,
		strategyScriptService,
		indicatorCalculationService,
		telegramDeliveryService,
		kCandleService,
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
	// Being on is a subresource that either exists or does not, rather than two
	// verbs. Pressing either button twice is then harmless because of the shape,
	// not because something remembered to allow it.
	//
	// It is /power and not /run because a round is /runs, and two paths that differ
	// by one letter while meaning completely different things is a mistake waiting
	// to be made — by a reader, by a caller, and by whoever edits this next.
	engine.POST("/strategy-bots/:id/power", requiresSignIn, strategyBotController.StartStrategyBot)
	engine.DELETE("/strategy-bots/:id/power", requiresSignIn, strategyBotController.StopStrategyBot)
	// 一台機器人跑過哪幾輪，是它自己的一份東西，所以掛在它底下而不是另開一條路徑。
	engine.GET("/strategy-bots/:id/runs", requiresSignIn, strategyBotController.ListRunRecords)
	// 立刻跑一輪。它與排程跑的那一輪走完全同一條路——不然「按下去看到的」
	// 與「它自己跑出來的」就是兩件事，而那正是這顆按鈕要用來排除的東西。
	engine.POST("/strategy-bots/:id/runs", requiresSignIn, strategyBotController.RunRoundNow)

	return kCandleFollowApplication, kCandleIngestionApplication,
		kCandleContractIngestionApplication, strategyBotRunApplication,
		assistantConversationApplication,
		contractSeriesApplications{
			fundingRate:       contractFundingRateApplication,
			positionStatistic: contractPositionStatisticApplication,
			tradingSymbol:     contractTradingSymbolApplication,
		}
}

// contractSeriesApplications are the contract use cases that have background rounds
// of their own beside the candles. They travel together because they are handed to
// the jobs together, and a list of three more positional returns would be three more
// places to put one in the wrong slot.
type contractSeriesApplications struct {
	fundingRate       *application.ContractFundingRateApplication
	positionStatistic *application.ContractPositionStatisticApplication
	tradingSymbol     *application.ContractTradingSymbolApplication
}

// assistantQueriesFor is everything the assistant is allowed to do.
//
// It is assembled here and only here, which is what makes "it cannot delete a
// strategy script" — or a trading strategy, or touch a bot — a fact about the system
// rather than a check somebody could remove: there is no deleting capability to reach
// for, and no K candle writing one either. Adding a capability is adding one line to
// this list.
//
// Each capability calls the very same use case a person calls, so no rule is relaxed
// for the assistant and none had to be written twice.
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

// backgroundJobsFor assembles the work the system does on its own. Switching
// background jobs off leaves nothing to start; an empty watchlist means every round
// has nothing to fetch, which is a state rather than a failure.
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

	// The contract venue gets a round of its own rather than more work inside the spot
	// one. The two spend different allowances, and one refusing to answer must not
	// hold the other up — which is exactly what sharing a round would do.
	contractKCandleIngestionJob := job.NewContractKCandleIngestionJob(
		kCandleContractIngestionApplication, job.KCandleIngestionInterval)

	// Handing out a market's live places is its own job rather than another step of
	// a round: a round held up by a source that will not answer would otherwise hold
	// up a market that has just opened.
	liveFollowRosterJob := job.NewLiveFollowRosterJob(
		kCandleFollowApplication, job.LiveFollowRosterInterval)

	// One scan for every standing bot, rather than one goroutine per bot. What a bot
	// is doing lives in the store, so this job asks rather than remembers — and a
	// restart costs one scan interval instead of switching every bot off without
	// telling anybody.
	strategyBotScanJob := job.NewStrategyBotScanJob(
		strategyBotRunApplication, applicationConfig.StrategyBot.ScanInterval)

	backgroundJobs := []domaininterface.IBackgroundJob{
		kCandleIngestionJob, contractKCandleIngestionJob, liveFollowRosterJob, strategyBotScanJob,
	}

	// Each contract series keeps its own time, so each is a job of its own rather
	// than more work inside the candle round: a venue slow to answer about funding
	// rates must not hold up the candles, and the other way round. Any of them can be
	// switched off alone.
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

	return backgroundJobs
}

// marketDataProxyFor is where every market's candle source is named, and the only
// place they are all named together.
//
// Recognising a third market is one more entry in each of these three, plus its
// rules in the settings. Nothing else in the system changes: everything above these
// asks for a window of candles and never learns which venue answered.
// venuePacers is one pacer per venue, shared by every proxy that spends that venue's
// allowance.
//
// **The allowance is counted per venue, not per kind of question.** Asking whether a
// symbol is listed and asking for a day of candles both spend it, so a pacer each
// would let the two of them together go at twice the rate either was allowed — and
// the one being paced is the one doing thousands of requests in a row.
type venuePacers struct {
	crypto marketdata.RequestPacer
	// cryptoContract is the perpetual contract venue's own. It is **not** the spot
	// one: the two count their allowances separately, so sharing a pacer would spend
	// half of each. Every proxy that reaches the contract venue takes this one, so a
	// second contract series added later joins the same budget rather than opening a
	// second one beside it.
	cryptoContract marketdata.RequestPacer
	// cryptoContractStatistics is the contract venue's allowance for its position
	// statistics, which it counts apart from the one above. It is the one exception
	// to "one venue, one budget", and it is the venue's exception, not this system's.
	cryptoContractStatistics marketdata.RequestPacer
	// taiwanStock is the market data plan's allowance, which the live quotes no longer
	// spend: they come from the exchange itself, and its pace is the poll interval
	// rather than an allowance shared with anybody.
	taiwanStock marketdata.RequestPacer
}

func newVenuePacers(applicationConfig config.ApplicationConfig) venuePacers {
	return venuePacers{
		crypto: marketdata.NewRequestPacer(
			applicationConfig.Ingestion.MarketDataRequestsPerMinute),
		cryptoContract: marketdata.NewRequestPacer(
			applicationConfig.ContractIngestion.RequestsPerMinute),
		cryptoContractStatistics: marketdata.NewRequestPacer(
			applicationConfig.ContractIngestion.StatisticsRequestsPerMinute),
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
				// 這個來源是一天打一次，所以它要問得到「哪幾天這個市場根本不開」——
				// 把窗口收進交易時段只動得到頭尾，中間那些休市日還留在裡面。
				// 給的是市場本身而不是時段，那個問題才不會在這裡被回答第二次。
				domains.NewMarketCatalogDomain(applicationConfig.MarketRules).
					MarketOf(string(vo.MarketTaiwanStock)),
				clock.NewSystemClockProxy(),
				applicationConfig.TaiwanStock.RequestTimeout,
				venuePacers.taiwanStock,
			),
		})
}

// liveMarketDataProxyFor is where every market's live feed is named.
func liveMarketDataProxyFor(
	applicationConfig config.ApplicationConfig, venuePacers venuePacers,
) domaininterface.ILiveMarketDataProxy {
	return marketdata.NewMarketRoutedLiveMarketDataProxy(
		map[vo.MarketVo]domaininterface.ILiveMarketDataProxy{
			vo.MarketCrypto: marketdata.NewBinanceLiveMarketDataProxy(
				applicationConfig.LiveFollow.MarketDataStreamUrl),
			// Live comes from the same venue as the stored history, so a live update
			// and the candle it lands beside cannot disagree about a figure. It also
			// shares that venue's one allowance, which is why it takes the same pacer.
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

// symbolLookupProxyFor is where every market is asked whether it has heard of a code.
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
