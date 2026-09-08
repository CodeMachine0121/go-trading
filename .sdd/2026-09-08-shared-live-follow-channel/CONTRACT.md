# Contract Traceability Matrix — 一條即時通道跟多檔

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/`, `internal/domain/service/`, `internal/infrastructure/marketdata/`, `internal/config/`
Oracle: Acceptance Criteria（20 條）＋ Core Business Rules（12 條）＋ Non-Functional（4 條）＝ 36 clauses

> **Ceiling.** 這是**靜態**符合性稽核：逐條把「規格推導出的預期結果」分別對照測試斷言與 production 程式碼路徑，**不以整套測試綠燈作為判準**，也不自行撰寫或執行新的探測案例。

## Clauses

### US-01 — 同時跟盤上限由方案的兩個數字推導

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 台股的上限是一條通道乘上每條五檔 | 上限為五檔 | `market_domain.go:88` | `market_domain_test.go:TestSimultaneousFollowCeilingIsTheTwoPlanNumbersMultiplied/一條通道乘上每條五檔是五檔` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 兩條通道各跟三檔的上限是六檔 | 上限為六檔 | `market_domain.go:88` | `.../兩條通道各跟三檔是六檔` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 一條通道只跟一檔的上限是一檔 | 上限為一檔 | `market_domain.go:88` | `.../一條通道只跟一檔是一檔` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 不限通道數的市場不設上限 | 不設上限，任何交易標的都跟得動 | `market_domain.go:89`（通道數為零即回零）＋ `HasFollowCeiling` | `.../不限通道數就不設上限`；`market_domain_test.go:TestSimultaneousFollowCeilingIsTheMarketsOwn` | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 — 一個市場的跟盤名單共用一條通道

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-05 | 四檔名單只開一條通道 | 只開一條通道，四檔全部掛在那一條上 | `live_follow_roster_domain.go:118 Channels()` ＋ `k_candle_follow_service.go:startMissingChannels` | `live_follow_channel_test.go:TestALimitedMarketPutsItsWholeRosterOnOneChannel`（切通道）；`k_candle_follow_service_test.go:1391 TestALimitedMarketsRosterTravelsDownOneChannel`（真的只開一條） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 只有一檔時走的是同一條路 | 開一條通道，上面掛著那一檔 | `live_follow_roster_domain.go:118` | `live_follow_channel_test.go:TestASingleHolderStillTravelsOnAChannel` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 名單為空時不開通道 | 不開任何通道 | `live_follow_roster_domain.go:118`（回空）＋ `startMissingChannels` | `live_follow_channel_test.go:TestAnEmptyRosterAsksForNoChannels` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 不限量的市場維持一檔一條 | 為每一檔各開一條即時通道 | `k_candle_follow_service.go:WatchKCandles`（以單檔建立通道）＋ `market_domain.go:100`（零視為一） | `k_candle_follow_service_test.go:261 TestChangingSymbolLeavesTheOldMarketBehind`、`:185 TestOneFollowPerSymbolNoMatterHowManyAreWatching`（各自一份跟盤、彼此的更新不互串）；`live_follow_channel_test.go:TestAMarketWithNoCeilingIsNotRosteredIntoChannels`（不由名單發動） | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 — 同一條通道上的每一檔各自收到自己的資料

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-09 | 收到自己那一檔的進行中 K 線 | 看甲的人收到甲的那一根 | `k_candle_follow_service.go:483`（依代號分流）；`fugle_live_market_data_proxy.go:161`（每檔一個折疊器） | `k_candle_follow_service_test.go:1403 TestACandleReachesOnlyTheViewersOfItsOwnSymbol`；`fugle_live_market_data_proxy_test.go:TestEachSymbolOnTheLineIsFoldedOnItsOwn` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 收不到別人那一檔的資料 | 看甲的人收不到任何更新 | `k_candle_follow_service.go:483` | 同上（該測試先送乙、再送甲，斷言收到的是甲） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 走完的那一根只算在自己頭上 | 甲那一根被存入；乙沒有任何 K 線被存入 | `k_candle_follow_service.go:483` → `report`／`store`（以該根自己的代號建立） | `k_candle_follow_service_test.go:TestOnlyTheSymbolACandleNamesIsStored`（在共用通道上送出後掛那一檔走完的 K 線，斷言存入的是它、且只有一筆） | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 — 通道中斷時掛在上面的每一檔一起處理

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-12 | 一條通道斷掉，上面每一檔的觀看者都被告知 | 甲、乙、丙的觀看者都被告知即時更新已停止 | `k_candle_follow_service.go:436` → `follow_channel.go:publishStalled` | `k_candle_follow_service_test.go:1424 TestAChannelEndingTellsEverySymbolOnIt` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 重新跟上時整份名單一起回來 | 開一條新通道，三檔全部掛在上面 | `k_candle_follow_service.go:run`（以同一個 `LiveFollowChannelVo` 重開） | `k_candle_follow_service_test.go:1444 TestAChannelThatComesBackCarriesTheWholeRosterAgain` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 連得上但不送資料，重試間隔照樣拉長 | 間隔逐次拉長到三十秒，不停在最短 | `live_channel_health_domain.go:MarkConnected`（不重設間隔） | `live_channel_health_domain_test.go:TestConnectingWithoutDeliveringNeverShortensTheRetryGap` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 真的收到資料才算恢復 | 下一次的重試間隔回到最短 | `live_channel_health_domain.go:MarkReceived`；`k_candle_follow_service.go:479` | `live_channel_health_domain_test.go:TestReceivingSomethingAgainPutsTheRetryGapBackToItsShortest` | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 — 名單改變時通道重新建立

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-16 | 加入一檔會重建通道 | 舊通道先結束，新通道掛著加進來之後的名單 | `live_follow_channel_vo.go:32`（鍵＝集合）＋ `k_candle_follow_service.go:takeDepartedChannels`／`startMissingChannels`（先退後起） | `k_candle_follow_service_test.go:1458 TestARosterThatGainsASymbolRebuildsTheChannel`；`:1056 TestChannelsAreGivenUpBeforeNewOnesAreTaken`（換名單的瞬間也只有一條） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 移除一檔會重建通道 | 舊通道先結束，新通道只掛著剩下的 | 同上 | `k_candle_follow_service_test.go:1487 TestARosterThatLosesASymbolRebuildsTheChannel` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 名單沒變就不動通道 | 通道維持原樣，沒有任何一檔被中斷 | `live_follow_channel_vo.go:32`（鍵不變即比對得上） | `k_candle_follow_service_test.go:1502 TestARosterThatDidNotChangeLeavesTheChannelAlone`；`:1474 TestASymbolThatWinsNoPlaceLeavesTheChannelAlone`（名額已滿、名單其實沒變）；`live_follow_channel_test.go:TestAChannelIsTheSameChannelWhateverOrderItsSymbolsArriveIn` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 名單變成空的就不再開通道 | 舊通道結束，不開新的 | `live_follow_roster_domain.go:118`（回空）＋ `takeDepartedChannels` | `k_candle_follow_service_test.go:TestARosterThatEmptiesEndsTheOpenChannel`（觀看者被告知、更新真的收掉、之後沒有新線）；`live_follow_channel_test.go:TestAnEmptyRosterAsksForNoChannels` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 重建期間走完的那一根不會永久少掉 | 下一輪自動抓取把它取回並存入 | `k_candle_ingestion_job.go`／`k_candle_ingestion_service.go`（既有，每分鐘一輪） | `k_candle_ingestion_service_test.go:TestScheduledRoundStoresTheNewestClosedCandles` | asserts-oracle（抓取行為）；「重建期間」這個前提不由測試建立 | produces-oracle | ✅ conforms |

### Core Business Rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-01 | 即時通道是一條連線，可同時跟多檔 | 一條連線承載多檔 | `live_follow_channel_vo.go`；`fugle_live_market_data_proxy.go:80`（一次認證、逐檔訂閱） | `fugle_live_market_data_proxy_test.go:TestOneLineCarriesEverySymbolOfTheChannel` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 兩個上限皆由方案決定、可調整、必須大於零 | 兩個設定值可調整；非正值回到預設 | `application_config.go:loadTaiwanStockConfig`（`positiveIntWithDefault`） | `ingestion_config_test.go:TestLoadAppliesTaiwanStockDefaultsWhenNothingIsSet` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 同時跟盤上限是推導值，不可直接設定 | 沒有任何可直接設定上限的途徑 | `market_rules_vo.go`（無上限欄位）；`market_domain.go:88` | 全 codebase 已無「上限」欄位或環境變數（見 Orphans） | no-test（「無法設定某件事」無從以行為測試斷言） | produces-oracle | 🟡 partial |
| BR-04 | 不限通道數的市場不設上限，且一條通道只跟一檔 | 加密貨幣維持現況 | `market_domain.go:89,100` | `market_domain_test.go:TestSymbolsPerLiveChannelIsNeverFewerThanOne` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 名單挑選規則完全不變 | 登錄最早的那幾檔，至多上限，非交易時段為空 | `live_follow_roster_domain.go:NewLiveFollowRosterDomain`（未改動） | `k_candle_follow_service_test.go:TestALimitedMarketFollowsItsEarliestRegisteredSymbolsAndNoMore` 等既有四支 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 名單整份掛上同一條通道，有幾檔跟幾檔 | 同 AC-05／AC-06 | `live_follow_roster_domain.go:118` | 同 AC-05／AC-06 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 同一條通道上每一檔各自收到自己的資料 | 同 AC-09～AC-11 | `k_candle_follow_service.go:483`；`fugle_live_market_data_proxy.go:161` | 同 AC-09～AC-11 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 通道斷掉時每一檔都被告知，並重新跟上整條 | 同 AC-12／AC-13 | `follow_channel.go:publishStalled`；`k_candle_follow_service.go:run` | 同 AC-12／AC-13 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-09 | 重試間隔以通道為單位；只有收到資料才回到最短 | 同 AC-14／AC-15 | `live_channel_health_domain.go` | `live_channel_health_domain_test.go` 全套 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 名單改變時先結束舊的再開新的；沒變則不動 | 同 AC-16～AC-18 | `k_candle_follow_service.go:RefreshFixedFollows`（退場在前、啟動在後） | `k_candle_follow_service_test.go:1056 TestChannelsAreGivenUpBeforeNewOnesAreTaken`（高水位為一） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 重建期間的空窗由每分鐘一輪的自動抓取補上 | 同 AC-20 | `k_candle_ingestion_job.go`（既有） | 同 AC-20 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 觀看者看到的內容不變 | 進行中／走完／已停止的形狀與時機不變；名額外的一樣被告知沒有即時更新 | `dto.KCandleFollowUpdateDto`（未改動）；`symbol_follow.go` 的 publish 家族（未改動） | `k_candle_follow_service_test.go` 既有的觀看者相關測試全數未改斷言而通過 | asserts-oracle | produces-oracle | ✅ conforms |

### Non-Functional

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-01 | 台股任一時刻同時開啟的通道數不得超過方案允許的條數 | 換名單的瞬間也不得超過一條 | `k_candle_follow_service.go:RefreshFixedFollows`（先退後起）＋ `Channels()`（依上限切段） | `k_candle_follow_service_test.go:1056 TestChannelsAreGivenUpBeforeNewOnesAreTaken`（高水位斷言為一） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-02 | 連得上卻不送資料的通道，重試頻率最終降到每三十秒一次 | 間隔數列到達三十秒並停在那裡 | `live_channel_health_domain.go:NextRetryDelay` | `live_channel_health_domain_test.go:TestConnectingWithoutDeliveringNeverShortensTheRetryGap`、`TestNextRetryDelayGrowsUpToTheCeilingAndNeverGivesUp` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-03 | 不涉及任何權限或身分規則的變更 | 認證授權相關程式碼未改動 | — | — | no-test | produces-oracle（本切片 diff 未觸及 `security/`、`session`、`user`） | 🟡 partial（以 diff 範圍佐證） |
| NFR-04 | 行情來源本來就支援一條通道訂閱多檔，取得方式相同 | 一次認證、逐檔訂閱，位址不變 | `fugle_live_market_data_proxy.go:80` | `fugle_live_market_data_proxy_test.go:TestOneLineCarriesEverySymbolOfTheChannel` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `binance_live_market_data_proxy.go:FollowKCandles`（拿到非一檔即回絕） | 契約說「一組」，這個來源只做得到一檔，超過就明確拒絕 | undocumented — `ARCH.md` §8 已記為刻意的取捨（安靜地只跟第一檔，會讓其餘看起來被跟著卻永遠不動）。**不是 out-of-scope 違規**：它守的正是 out-of-scope 那條「加密貨幣不改為共用通道」 |
| `TAIWAN_STOCK_SIMULTANEOUS_FOLLOW_CEILING` 設定已淘汰 | 舊設定名稱在程式與文件中皆已不存在 | 已於實作時一併移除，README 與 `.env.example` 同步換成兩個新設定。**無殘留** |

## Summary

- Conforms: 34/36 clauses ✅（94.4%，另兩條為 🟡）
- Violations: 無 🔴
- Mis-asserted: 無 🟠（原 `AC-11`、`AC-19` 缺的兩支已補上並各自以變異驗證會紅）
- Partial: `BR-03`、`NFR-03` 🟡（兩者都在描述「系統刻意不做／不提供某件事」，無從以行為測試斷言）
- Gaps: 無 ❌
- Unclear: 無 ❔
- Orphans: 2，皆已在設計文件中交代，無 out-of-scope 違規
