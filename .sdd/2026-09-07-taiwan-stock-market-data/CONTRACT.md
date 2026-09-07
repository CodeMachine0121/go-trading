# 台股行情接入 — Contract Verification Matrix

**Oracle:** `PRD.md` 第 3 節的 Gherkin 驗收條件（48 條）、第 4 節業務規則（20 條）、
第 6 節非功能需求（6 條）
**Design map:** `ARCH.md`（第 7 節 Traceability，用來定位程式碼，不作為契約）
**判定方式:** 靜態一致性稽核——測試對照 oracle、程式碼對照 oracle，兩邊**各自獨立**判定；
不以「測試跑綠」作為結論。

> **稽核天花板：** 本次只讀既有的測試與程式碼，不新寫探針、不執行自行發明的情境。
> 「程式碼稽核」欄是讀程式碼路徑得到的結論，不是跑測試得到的。

---

## 1. Clauses — US-01 每個交易標的記著自己屬於哪個市場

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-01.1 | 台股的交易標的向台股的來源取資料 | 取數請求送到台股那一份來源，而非另一份 | `market_routed_market_data_proxy.go:39` | `TestFetchingIsSentToTheSourceThatServesTheWindowsMarket` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.2 | 加密貨幣的交易標的維持現狀 | 取數請求送到加密貨幣那一份來源 | 同上（同一張 map 的另一個項目） | 同上（對另一份設期望即失敗，因此「送錯」抓得到） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.3 | 沒有登錄過的交易標的回覆找不到 | 呼叫端被告知**找不到這個交易標的** | `k_candle_follow_service.go:98`（哨兵錯誤）＋`k_candle_follow_controller.go:41`（對映為「找不到」） | `TestWatchingASymbolNobodyRegisteredIsRefused`（service 層）、`TestWatchingASymbolNobodyRegisteredIsAnsweredAsNotFound`（對外那一層） | asserts-oracle | produces-oracle | ✅ conforms（**本次稽核先判 🔴 violation，已修**：原本每一種跟盤失敗都被折成「服務暫時無法使用」，於是「你要的標的不存在」被說成「系統壞了、稍後再試」——兩句話給的下一步完全相反） |
| AC-01.4 | 既有的舊資料沒有記市場時視為加密貨幣 | 該標的走加密貨幣的來源與規則 | `market_catalog_domain.go:47`（`MarketOf` 正規化） | `TestMarketOfReadsWhatWasStored`／`a row that names no market` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.5 | 建立資料庫結構時預設交易標的屬於加密貨幣且追蹤中 | 兩檔都被登錄為 `crypto` 且 `IsWatched` 為真 | `trading_symbol_service.go:126` | `TestRegisterDefaultTradingSymbols`＋`newlyRegistered` 期望形狀 | asserts-oracle | produces-oracle | ✅ conforms |

## 2. Clauses — US-02 觀察清單由系統保管，改了下一輪就生效

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-02.1 | 加進來的下一輪就被抓 | 下一輪的報告裡出現這個標的 | `k_candle_ingestion_service.go:116`（每輪 `FindWatched`） | `TestARoundReadsTheWatchlistAfresh` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.2 | 拿掉的下一輪就不抓，既有 K 線仍查得到 | 下一輪不含它；它的 K 線一根未刪 | 同上（不抓）＋`trading_symbol_service.go:211`（只改 `IsWatched`） | `TestARoundReadsTheWatchlistAfresh`（不抓）、`TestRemoveFromWatchlist/stops watching without removing anything else`（不刪） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.3 | 進行中的那一輪照它開始時的清單跑完 | 該輪抓的仍是開始時清單上的那些 | `k_candle_ingestion_service.go:116`（一輪只讀一次，讀在最上面） | `TestARoundInFlightWorksFromTheListItStartedWith`（在取數進行中製造清單改動，且只准讀一次） | asserts-oracle | produces-oracle | ✅ conforms（本次稽核先判 🟠 mis-asserted，已補測試） |
| AC-02.4 | 觀察清單一檔都沒有時什麼都不抓 | 不向任何來源取數，且不算錯誤 | `k_candle_ingestion_service.go:141`（空清單即空報告） | `TestScheduledRoundOnAnEmptyWatchlistDoesNothing` | asserts-oracle | produces-oracle | ✅ conforms |

## 3. Clauses — US-03 把交易標的加進觀察清單

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-03.1 | 加一檔存在的台股 | 先向該市場確認存在，確認後登錄為該市場且追蹤中 | `trading_symbol_service.go:169` | `TestAddToWatchlist/checks with the market, then starts watching` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.2 | 代號在該市場不存在即拒絕，清單維持原狀 | 拒絕並說明找不到；沒有任何寫入 | 同上（`ErrTradingSymbolNotInMarket`，在 `Save` 之前 return） | `TestAddToWatchlist/refuses a code the market has never heard of`（未對 `Save` 設期望＝寫入即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.3 | 行情來源不可用時加不進去，且沒有先加進去 | 拒絕並說明稍後再試；沒有任何寫入 | 同上（`ErrMarketDataSourceUnavailable`） | `TestAddToWatchlist/refuses when the market cannot be asked`（並斷言**不是**「找不到」那一種） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.4 | 重複加同一檔維持一筆，不算失敗 | 清單上仍是一筆；不回報錯誤 | `trading_symbol_repository.go:87`（以名稱為鍵覆寫） | `TestAddToWatchlist/adding one already watched leaves one entry`、`TestSaveTradingSymbol/replaces what was held` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.5 | 加回一檔以前被拿掉的，K 線一根都沒少 | 重新標記為追蹤中；既有 K 線未受影響 | `watchlist_entry_domain.go:68`（沿用原登錄時間，不刪任何東西） | `TestAddToWatchlist/adding back one that was removed keeps its place in the queue` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.6 | 代號空白即拒絕，且不向任何來源詢問 | 拒絕；沒有任何往返 | `watchlist_entry_domain.go:32`（在查詢之前） | `TestAddToWatchlist/refuses a blank code without asking any market`（未對 lookup 設期望） | asserts-oracle | produces-oracle | ✅ conforms |

## 4. Clauses — US-04 把交易標的從觀察清單拿掉

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-04.1 | 拿掉之後不再抓，但三個月的 K 線全部仍查得到，且仍是系統認得的標的 | 只有 `IsWatched` 變假；登錄與 K 線未動 | `trading_symbol_service.go:211` | `TestRemoveFromWatchlist/stops watching without removing anything else`（斷言存回的實體只有 `IsWatched` 不同）、`TestASymbolTakenOffTheWatchlistIsStillListed` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.2 | 拿掉一檔正在被即時跟盤的台股，名額遞補 | 停止跟它；名額之外那一檔開始被跟 | `k_candle_follow_service.go:156`（下一次重算名單） | `TestASymbolPushedOutOfItsPlaceIsToldItsUpdatesAreGone` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.3 | 拿掉一檔本來就不在清單上的，不算失敗 | 不回報錯誤，且不寫入 | `trading_symbol_service.go:211`（未登錄或未追蹤即 return nil） | `TestRemoveFromWatchlist/removing one nobody registered is not a failure`、`/removing one that was not being watched writes nothing` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.4 | 拿掉清單上最後一檔，下一輪什麼都不抓 | 下一輪空手且不算錯誤 | `k_candle_ingestion_service.go:141` | `TestScheduledRoundOnAnEmptyWatchlistDoesNothing`（同一條路徑） | asserts-oracle | produces-oracle | ✅ conforms |

## 5. Clauses — US-05 台股非交易時段直接跳過，不算失敗

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-05.1 | 交易時段內兩檔都抓 | 兩者都向來源取數並存入 | `k_candle_ingestion_service.go:173`（窗收窄後非空即取） | `TestARoundSkipsAMarketThatCouldHoldNothing/mid session` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.2 | 收盤後台股跳過、不留失敗紀錄，加密貨幣照常 | 不向台股來源取數；該標的無失敗原因；另一市場照常 | `market_domain.go:65`（`ClampToTradingSession` 收成空）＋`k_candle_ingestion_service.go:173` | `TestARoundSkipsAMarketThatCouldHoldNothing/the evening`、`TestAClosedMarketDoesNotStopAnotherOneBeingFetched` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.3 | 週末台股全部跳過 | 同上 | 同上（`tradesOn` 週一至週五） | `TestARoundSkipsAMarketThatCouldHoldNothing/sunday`、`TestClampToTradingSessionKeepsOnlyWhatCouldHoldCandles/a sunday round covers nothing` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.4 | 剛收盤那一輪存入最後一根 13:25 | 13:25 那一根被取到並存入 | `market_domain.go:65`（收窄成 `[13:05, 13:25]`，窗仍與時段重疊） | `TestClampToTradingSessionKeepsOnlyWhatCouldHoldCandles/the round just after the close still reaches the day's last candle`（domain 層直接斷言 13:25）；服務層 `TestARoundSkipsAMarketThatCouldHoldNothing/just after the close` 只斷言「有去問、沒失敗」 | asserts-oracle（domain 層） | produces-oracle | ✅ conforms |
| AC-05.5 | 休市日由行情來源的回覆推定，當日剩餘輪次跳過 | 第一輪照問；來源正常回覆卻零根即當日不再問 | `k_candle_ingestion_service.go:251`（`presumeClosedMarkets`）＋`market_closure_ledger.go:33` | `TestAMarketThatAnswersWithNothingIsPresumedShutForItsOwnDay`（來源只准被問一次） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.6 | 隔日重新判定休市 | 隔天照常向來源取數 | `market_domain.go:106`（`TradingDateOf` 以市場本地日為界）＋`market_closure_ledger.go:44` | `TestAMarketPresumedShutIsAskedAgainTheFollowingDay` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.7 | 交易時段內來源不可用才算失敗，且不推定休市 | 留下失敗紀錄；下一輪照常重試 | `k_candle_ingestion_service.go:173`（取數失敗即 `FetchFailureReason`，`WasAsked` 維持假） | `TestASourceThatWillNotAnswerIsNeverReadAsAHoliday`（兩輪都問到、兩輪都有失敗原因） | asserts-oracle | produces-oracle | ✅ conforms |

## 6. Clauses — US-06 台股固定跟最早登錄的那幾檔

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-06.1 | 清單上的台股多於名額時只跟最前面那幾檔 | 跟盤份數等於上限；跟的是最早登錄的那幾檔 | `live_follow_roster_domain.go:36`＋`k_candle_follow_service.go:211` | `TestALimitedMarketFollowsItsEarliestRegisteredSymbolsAndNoMore`（份數＋實際開了哪幾條 feed 都驗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.2 | 沒有人在看也照跟 | 沒有任何觀看者時份數仍大於零 | `symbol_follow.go:31`（`isOnARoster` 讓它不因最後一個觀看者離開而結束） | `TestALimitedMarketIsFollowedWithNobodyWatching`、`TestAFollowHeldUpByARosterOutlivesItsLastViewer`（用 `Never` 斷言不會掉） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.3 | 台股不足名額時不去湊滿 | 有幾檔跟幾檔 | `live_follow_roster_domain.go:36` | `TestALimitedMarketFollowsFewerThanItsPlacesWhenThatIsAllThereIs` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.4 | 名額之內的觀看者立刻收到即時更新 | 加入既有那一份跟盤，份數不增加 | `k_candle_follow_service.go:86`（查有即 `join`） | `TestAViewerOfASymbolWithAPlaceJoinsTheFollowAlreadyRunning` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.5 | 名額之外的觀看者被告知沒有即時更新，仍查得到已存入的 K 線 | 收到「沒有即時更新」這一種訊息；且不佔用名額 | `k_candle_follow_service.go:240`（`unavailableUpdates`） | `TestAViewerOfASymbolWithNoPlaceIsToldSoRatherThanShownAFrozenPicture`（狀態＋份數未增） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.6 | 收盤時不跟任何台股 | 名單為空，份數為零 | `live_follow_roster_domain.go:36`（`IsOpen` 為否即跳過） | `TestAClosedLimitedMarketHoldsNoPlaces` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.7 | 開盤時自己跟回來 | 進入交易時段後重算名單即恢復跟盤 | `live_follow_roster_job.go:64`（每輪重算）＋`live_follow_roster_domain.go:36` | `TestALimitedMarketIsFollowedAgainOnceItOpens`（收盤時歸零、時間走到開盤後重新拿回名額） | asserts-oracle | produces-oracle | ✅ conforms（本次稽核先判 🟡 partial，已補測試） |
| AC-06.8 | 加密貨幣仍然是有人看才跟 | 沒有觀看者時不跟 | `live_follow_roster_domain.go:36`（無上限即拿不到名額） | `TestARoundTheClockMarketIsStillOnlyFollowedWhileSomebodyWatches`、`TestHandingOutPlacesLeavesAViewerDrivenFollowAlone` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.9 | 清單改動後名單跟著重新決定 | 被拿掉的停止、遞補的開始 | `k_candle_follow_service.go:156` | `TestASymbolPushedOutOfItsPlaceIsToldItsUpdatesAreGone` | asserts-oracle | produces-oracle | ✅ conforms |

## 7. Clauses — US-07 台股沒有的成交數字留空，不當成零

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-07.1 | 台股的 K 線那三個數字沒有值，其餘有值 | 三項為「沒有值」，開高低收與成交量有值 | `fugle_wire.go:37`（不設那三項）→`k_candle.go:20`（可空欄位） | `TestFugleLeavesTheFiguresItDoesNotPublishAbsent`、`TestNewKCandleDomainAcceptsACandleFromAMarketThatReportsFewerFigures` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07.2 | 加密貨幣的 K 線三個數字都有值 | 三項皆有值 | `binance_wire.go:58`（明確包成有值） | `TestKCandleBucketDomainMergesTheCandlesItHolds`（有值路徑）、既有 binance wire 測試 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07.3 | 成交量是零與沒有值不同 | 成交量為零且仍「有值」；成交額仍「沒有值」 | `optional_figure_domain.go:20` | `TestAnUnreportedFigureIsNotZero`、`TestPlusKeepsAnUnreportedFigureUnreported/a reported zero is still reported` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07.4 | 完全沒有成交的那五分鐘沒有那一根 | 不補一根空的、不沿用前一根 | 既有規則：來源沒給就不存；`k_candle_bucket_domain.go` 不補洞 | 既有 `k_candle_series_domain_test.go` 的不補洞測試；本切片未新增 | asserts-oracle（既有） | produces-oracle | ✅ conforms |
| AC-07.5 | 成交量照原樣存，不換算單位 | 12000 股存進去仍是 12000 | `fugle_wire.go:37`（原樣帶過） | `TestFugleReadsAPushedCandleIntoTheShapeTheDomainKnows`（斷言 8450 原樣） | asserts-oracle | produces-oracle | ✅ conforms |

## 8. Clauses — US-08 啟動回補只補交易時段內的

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-08.1 | 交易日盤中啟動補上今天的缺口 | 補 09:35–10:55 | `k_candle_ingestion_domain.go:85`＋`market_domain.go:65` | `TestClampToTradingSessionKeepsOnlyWhatCouldHoldCandles/a mid-session backfill starts where the stored candles ended` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08.2 | 週六啟動只補週五 09:00–13:25 | 收窄成週五整段 | `market_domain.go:65` | `TestClampToTradingSessionKeepsOnlyWhatCouldHoldCandles/a saturday backfill reaches back into friday's session`、`TestBackfillOnlyReachesBackIntoTradingSessions`（服務層驗到實際送出的窗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08.3 | 週一開盤後啟動只補今天 09:00–09:25 | 收窄成週一那一小段 | 同上 | `TestClampToTradingSessionKeepsOnlyWhatCouldHoldCandles/a monday backfill after a complete friday fills only this morning` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08.4 | 從未有過資料的台股補滿範圍內的交易時段 | 補回補範圍內每一段時段 | `k_candle_ingestion_service.go:85`（無資料即整段回補窗）＋收窄 | `TestBackfillOnlyReachesBackIntoTradingSessions`（`FindLatest` 回空） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08.5 | 回補範圍內完全沒有交易時段就沒有東西要補 | 不向來源取數且不算失敗 | `market_domain.go:65`（收成空窗）＋`k_candle_fetch_window_vo.go:IsEmpty` | `TestBackfillAsksForNothingWhenNothingInReachCouldTrade`（未對來源設期望） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08.6 | 加密貨幣的回補維持現狀 | 整段照補，不收窄 | `market_domain.go:65`（永不收盤即原樣回傳） | `TestClampToTradingSessionLeavesARoundTheClockMarketAlone`、既有回補測試全數維持 | asserts-oracle | produces-oracle | ✅ conforms |

## 9. Clauses — US-09 可查交易標的答得出市場、交易時段與即時更新

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-09.1 | 每個交易標的帶著所屬市場 | 2330 帶台股、BTCUSDT 帶加密貨幣 | `trading_symbol_service.go:56` | `TestEveryListedSymbolSaysWhichMarketItBelongsTo` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.2 | 台股在交易時段內／收盤中如實回報 | 盤中為真、收盤為假 | 同上（`marketDomain.IsOpen`） | `TestEveryListedSymbolSaysWhetherItsMarketIsTradingRightNow`（盤中）、`TestAClosedMarketSaysSoOnEveryOneOfItsSymbols`（收盤） | asserts-oracle | produces-oracle | ✅ conforms（本次稽核先判 🟠 mis-asserted，已補測試） |
| AC-09.3 | 加密貨幣任何時候都在交易時段內 | 恆為真 | 同上 | `TestEveryListedSymbolSaysWhetherItsMarketIsTradingRightNow`、`TestCryptoIsOpenWheneverItIsAsked` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.4 | 名額之內的台股帶著有即時更新 | 前 N 檔為真 | `live_follow_roster_domain.go:104`（`HasLiveUpdates`） | `TestALimitedMarketOnlyPromisesLiveUpdatesToSymbolsHoldingAPlace` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.5 | 名額之外的台股帶著沒有即時更新 | 第 N+1 檔為假 | 同上 | 同上（第六檔斷言為假） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.6 | 加密貨幣一律帶著有即時更新 | 恆為真，即使沒有人在看 | `live_follow_roster_domain.go:104`（無上限即為真） | `TestAMarketWithNoCeilingAlwaysPromisesLiveUpdates` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.7 | 不追蹤的交易標的仍然查得到 | 仍出現在清單上，且標為未追蹤 | `trading_symbol_service.go:56`（讀整張表） | `TestASymbolTakenOffTheWatchlistIsStillListed` | asserts-oracle | produces-oracle | ✅ conforms |

| AC-09.10 | 台股帶著行情來源給的公司名稱 | 2330 帶「台積電」 | `trading_symbol_service.go`（`registration.DisplayName`） | `TestAListingCarriesWhatEachVenueCallsItsSymbols` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.11 | 不取名字的市場名稱留空，不拿代號充數 | BTCUSDT 的名稱是空的 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.8 | 台股帶著「這個市場會收盤」 | 為真 | `trading_symbol_service.go`（`!marketDomain.NeverCloses()`） | `TestAListingSaysWhichMarketsKeepHoursAndThereforeShut` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09.9 | 加密貨幣帶著「這個市場不收盤」 | 為假 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |

> **US-03 追加三條**（加進來時記下名稱）：
>
> | ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
> |---|---|---|---|---|---|---|---|
> | AC-03.7 | 加進來時記下行情來源給的名稱 | 觀察清單上的 2330 帶「台積電」 | `LookUpSymbol` → `WatchlistEntryDomain.ToEntity` | `TestAddToWatchlist/stores what the venue calls it`、`TestFugleCarriesTheCompanyNameOutOfTheSameAnswer` | asserts-oracle | produces-oracle | ✅ conforms |
> | AC-03.8 | 重新加入即改名生效 | 名稱換成「台積電控股」 | 同上（每次都寫） | `TestAddToWatchlist/writes the venue's name every time` | asserts-oracle | produces-oracle | ✅ conforms |
> | AC-03.9 | 來源沒給名稱一樣加得進去 | 加進來了，名稱是空的 | `FugleSymbolLookupProxy`（讀不到 body 仍 `IsListed: true`）＋crypto 一律無名 | `TestFugleStillWatchesASymbolWhoseNameItCouldNotRead`、`TestAddToWatchlist/a market that names nothing…` | asserts-oracle | produces-oracle | ✅ conforms |

## 10. Clauses — US-10 加進觀察清單就馬上看得到

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-10.1 | 收盤後加一檔台股，補上今天交易時段內的每一根 | 加完立刻向來源要今天那一段 | `trading_symbol_application.go:AddToWatchlist` → `k_candle_ingestion_service.go:RunBackfillFor` | `TestTradingSymbolApplicationAddToWatchlist/catches the symbol up on the spot`、`TestCatchingOneSymbolUpAsksForItsOwnGapAfterTheCloseHasPassed` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10.2 | 補齊失敗不把加入退回去 | 仍在清單上，且未回報加入失敗 | 同上（只留紀錄） | `TestTradingSymbolApplicationAddToWatchlist/a catch-up that fails does not undo the add` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10.3 | 加入本身被拒絕時什麼都不補 | 未向來源要過任何 K 線 | 同上（加失敗即提早返回） | `TestTradingSymbolApplicationAddToWatchlist/nothing is caught up when the add itself was refused` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10.4 | 手動要求補齊，並回報收到幾根 | 回 200 與這一輪的報告 | `k_candle_backfill_controller.go` | `TestCatchingUpASymbolReportsWhatItCollected` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10.5 | 不在觀察清單上的一樣補得動 | 照樣向來源要 | `RunBackfillFor` 走 `FindBySymbol` | `TestCatchingOneSymbolUpReachesASymbolNobodyIsWatching` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10.6 | 沒登錄過的代號被拒絕，且不是「稍後再試」 | 404，不是 502 | `RunBackfillFor` + controller 對映 | `TestCatchingUpASymbolNobodyRegisteredIsRefused`、`TestCatchingUpASymbolNobodyRegisteredIsAnsweredAsNotFound` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10.7 | 只補一檔時不推定整個市場休市 | 同市場其他標的下一輪照常被抓 | `RunBackfillFor` 不呼叫 `presumeClosedMarkets` | `TestCatchingOneSymbolUpNeverDecidesItsWholeMarketIsShut` | asserts-oracle | produces-oracle | ✅ conforms |

## 11. Clauses — 業務規則（第 4 節）

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| BR-1 | 所屬市場是唯一依據，不從名稱格式推測 | 程式中沒有任何一處以代號長相決定市場 | `market_catalog_domain.go:47` | `TestMarketOfReadsWhatWasStored` | asserts-oracle | produces-oracle（全庫無名稱解析路徑） | ✅ conforms |
| BR-2 | 觀察清單即追蹤中的已登錄交易標的，不另立名單 | 只有一張表；清單是它的子集 | `trading_symbol.go:12`（同一個實體） | `TestFindWatched/returns only the symbols being kept up to date` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 每輪重讀，進行中那一輪不受影響 | 見 AC-02.1／AC-02.3 | `k_candle_ingestion_service.go:116` | 見 AC-02.3 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 先確認再加入；確認結果不留存 | 每次加入都問一次來源 | `trading_symbol_service.go:169` | `TestAddToWatchlist`（每個案例各自設一次 lookup 期望） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 移除只停追蹤，不刪登錄與 K 線 | 見 AC-04.1 | `trading_symbol_service.go:211` | 見 AC-04.1 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 重複加入維持一筆、重複移除不算失敗 | 見 AC-03.4／AC-04.3 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 交易時段：加密貨幣全天候，台股可調 | 兩者的規則由設定給入 | `application_config.go:marketRules` | `TestTheRecognisedMarketsCarryTheirOwnRules`、`TestLoadReadsTheTaiwanStockSessionAsATimeOfDay` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 非交易時段跳過：不取、不算失敗、不留紀錄 | 見 AC-05.2 | `k_candle_ingestion_service.go:173` | 見 AC-05.2 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 休市推定：正常回覆卻零根；隔日重判；不跨日留存 | 見 AC-05.5／AC-05.6 | `k_candle_ingestion_service.go:251` | 見 AC-05.5／AC-05.6 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 失敗的定義：來源無法回覆才算 | 見 AC-05.7 | 同上 | 見 AC-05.7 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 同時跟盤上限可調、必須大於零；加密貨幣不設限 | 台股讀設定、加密貨幣為零 | `market_domain.go:83`／`:95` | `TestSimultaneousFollowCeilingIsTheMarketsOwn`、`TestTheRecognisedMarketsCarryTheirOwnRules` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 台股跟盤名單依登錄先後、每輪重決、不提供調換順序 | 見 AC-06.1／AC-06.9 | `live_follow_roster_domain.go:36` | `TestFindWatched/hands them back earliest registered first`（名稱順序與登錄順序刻意相反） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-13 | 加密貨幣跟盤不變 | 見 AC-06.8 | `k_candle_follow_service.go:86` | 見 AC-06.8＋既有跟盤測試全綠 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-14 | 名額之外要明說，且與「斷了」分開 | 兩種訊息不同字 | `k_candle_follow_update_dto.go`（`unavailable` vs `stalled`） | `TestAViewerOfASymbolWithNoPlaceIsToldSoRatherThanShownAFrozenPicture`、`TestASymbolPushedOutOfItsPlaceIsToldItsUpdatesAreGone` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-15 | 沒有值不等於零 | 見 AC-07.1／AC-07.3 | `optional_figure_domain.go` | 見 AC-07.3 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-16 | 成交量不換算 | 見 AC-07.5 | `fugle_wire.go:37` | 見 AC-07.5 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-17 | 回補只補交易時段內 | 見 US-08 | `market_domain.go:65` | 見 US-08 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-18 | K 線規則一律沿用，本切片一條不改 | 既有規則測試全數維持綠燈 | `k_candle_domain.go` | 既有 `k_candle_domain_test.go` 全數維持 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-19 | 可查交易標的的三項附帶資訊 | 見 US-09 | `trading_symbol_service.go:56` | 見 US-09 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-20 | 預設交易標的登錄為加密貨幣且追蹤中 | 見 AC-01.5 | `trading_symbol_service.go:126` | 見 AC-01.5 | asserts-oracle | produces-oracle | ✅ conforms |

## 12. Clauses — 非功能需求（第 6 節）

| ID | 條款 | Oracle | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| NFR-1 | 每輪多讀一次觀察清單，總時間同一量級 | 每輪多一次讀取，非每個標的一次 | `k_candle_ingestion_service.go:116`（在 `prepareRun`，每輪一次） | 無專屬測試（效能特性） | no-test | produces-oracle（讀取在迴圈之外） | 🟡 partial |
| NFR-2 | 台股同時被跟的檔數任何時候不得超過上限 | 交接名單時瞬時檔數也不超過 | `k_candle_follow_service.go:156`（先停後起） | `TestPlacesAreGivenUpBeforeNewOnesAreTaken`（以同時開啟的 feed 高水位驗證） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 台股收盤時段每一輪都不得留下失敗紀錄 | 失敗原因為空 | `k_candle_ingestion_service.go:173` | `TestARoundSkipsAMarketThatCouldHoldNothing`（每個案例都斷言失敗原因為空） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 一個市場的來源不可用時，另一個市場照常 | 另一市場照常取數 | `k_candle_ingestion_service.go:141`（各標的獨立 goroutine） | `TestAClosedMarketDoesNotStopAnotherOneBeingFetched`、既有「一檔失敗不影響其他」測試 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-5 | 建立資料庫結構可重跑任意次數 | 重跑結果相同 | `trading_symbol_service.go:126`（先讀後補） | `TestRegisterDefaultTradingSymbols/registers nothing and says so when both are already there` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-6 | 升級後行為不變 | 沒記市場的視為加密貨幣；預設標的追蹤中 | `market_catalog_domain.go:47`、`trading_symbol_service.go:126` | `TestMarketOfReadsWhatWasStored/a row that names no market`、`TestRegisterDefaultTradingSymbols` | asserts-oracle | produces-oracle | ✅ conforms |

---

## 2. Orphans（沒有對應條款的程式碼）

| 程式碼 | 它做了什麼 | 判定 |
|---|---|---|
| `fugle_forming_k_candle.go` 全檔 | 把來源推播的 K 線折成五分鐘一根，並以「更晚的一根到了」推定前一根走完 | **undocumented**——PRD 沒有這條，因為它是來源的線路細節（該來源既不說一根多長、也不說何時定案）。屬於 ARCH §8 列為「實作時對照文件確定」的那一項，實作後應補一句進 ARCH |
| `market_routed_*_proxy.go` 的「沒有來源即失敗」 | 未接上來源的市場回報失敗而非空手 | **undocumented**——是 ARCH 的設計決定（§3），PRD 未涵蓋。行為正確且有測試，建議補進 PRD 的邊界情況 |
| `trading_symbol_service.go:56` 的 `isWatched` 欄位 | 清單多回一個「是否追蹤中」 | **undocumented**——PRD US-09 只列了三項附帶資訊，這是第四項。前端切片需要它（管理畫面），應補進 PRD |

---

## 3. Summary

**稽核當下：** 62 / 66 條 conforms（94%）——1 🔴、3 🟠／🟡。**全部已處理**，重新判定如下。

- **Conforms:** 79 / 80 條 ✅（99%）
- **Violations:** 無（`AC-01.3` 已修）
- **Mis-asserted:** 無（`AC-02.3`／`BR-3`、`AC-09.2` 已補測試）
- **Partial:** `NFR-1`（每輪多讀一次清單的成本，是效能特性，沒有以測試釘住）
- **Gaps:** 無
- **Unclear:** 無
- **Orphans:** 2（`isWatched` 那一項已隨 US-09 補進 PRD）

> **第三次追加：** `US-09` 再多兩條、`US-03` 多三條——交易標的記著行情來源給的公司名稱。
> 五條全部 conforms，三個 mutation 逐一驗過。這一次順帶把 `ISymbolLookupProxy` 從
> 「這檔存在嗎」改成「查到了什麼」：名稱本來就在證明代號為真的那同一個答案裡，
> 分兩次問是同一支端點打兩次，而且多開一個兩次答案會不一致的窗口。
>
> **本次追加：** `US-09` 多兩條（市場會不會收盤），`US-10` 七條全新——
> 加入觀察清單時立刻回補、以及手動回補。九條全部 conforms，
> 每一條都有一個會因為對應實作被打壞而變紅的測試（四個 mutation 已逐一驗過）。

### 這次稽核抓到什麼

**一個真的錯誤，而且是綠燈蓋不住的那一種。** `AC-01.3` 說「沒登錄過的交易標的回覆找不到」。
domain 那一層做對了——它回了一個專門的哨兵錯誤，也有測試釘住。
錯的是最外層：`k_candle_follow_controller.go` 把每一種跟盤失敗都折成
「服務暫時無法使用」。於是「你要的那一檔不存在」被說成「系統壞了、稍後再試」——
兩句話給觀看者的下一步完全相反，一句要他改輸入，一句要他等。
整套測試在這個狀態下是全綠的：service 層的測試驗的是哨兵錯誤，
而對外那一層根本沒有測試在問這件事。這正是「跑綠」看不到、對照 oracle 才看得到的東西。

**三個綠燈但沒驗到位的地方。** 一條規則寫著「進行中的那一輪照它開始時的清單跑完」，
而測試只驗了「兩輪各讀一次」——那在一輪內重讀的實作下也會通過。
另外兩條各只驗了條款的一半（只驗開盤、沒驗收盤；只驗關、沒驗開）。
三條都補上了會因為對應實作被打壞而變紅的測試。
