# Contract Traceability Matrix — 一分鐘 K 線

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/`, `internal/infrastructure/`, `internal/job/`
Oracle: Acceptance Criteria（26 條）＋ Core Business Rules（11 條）＋ Non-Functional（4 條）＝ 41 clauses

> **Ceiling.** 這是**靜態**符合性稽核：逐條把「規格推導出的預期結果」分別對照測試斷言與production 程式碼路徑，**不以整套測試綠燈作為判準**，也不自行撰寫或執行新的探測案例。

## Clauses

`Spec-expected` 欄是 Phase 2 由規格單獨推導出的業務可觀察結果。

### US-01 — K 線本身涵蓋一分鐘

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 起始時間落在整分鐘上的 K 線被接受（10:07:00） | 接受，之後查得到起始時間 10:07 的那根 | `k_candle_domain.go:40` | `k_candle_domain_test.go:TestNewKCandleDomainAcceptsAnyWholeMinute/a whole minute` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 起始時間帶有秒數的 K 線被拒絕（10:07:59） | 拒絕，說明起始時間必須落在一分鐘刻度上，且不留存 | `k_candle_domain.go:44` | `k_candle_domain_test.go:TestNewKCandleDomainRejectsBrokenRules/open time carrying seconds` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 起始時間落在整點上的 K 線被接受（10:00:00） | 接受 | `k_candle_domain.go:44` | `k_candle_domain_test.go:.../an exact hour is also a whole minute` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 帶有比秒更細成分的 K 線被拒絕 | 拒絕，說明必須落在一分鐘刻度上 | `k_candle_domain.go:44` | `k_candle_domain_test.go:.../open time carrying less than a second` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 原本合法的五分鐘刻度時間仍然合法（10:05:00） | 接受 | `k_candle_domain.go:44` | `k_candle_domain_test.go:.../a time on the old five minute mark` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 其餘 K 線規則不因顆粒度改變而放寬（最高價低於最低價） | 拒絕，說明最高價不得低於最低價 | `k_candle_domain.go:56` | `k_candle_domain_test.go:.../highest price below the lowest price` | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 — 一分鐘成為最細的彙總刻度與預設值

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-07 | 未指定彙總刻度時視為一分鐘 | 序列採一分鐘刻度，每根與原本那根一模一樣 | `aggregation_interval_domain.go:58` + `:28` | `k_candle_series_query_domain_test.go:TestNewKCandleSeriesQueryDomainDeclaringNoIntervalMeansOneMinute`；`k_candle_series_domain_test.go:TestKCandleSeriesDomainNamesTheIntervalItWasCutAt` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 指定五分鐘時真正把五根併成一根（開 100／收 108／高 110／低 99／量為總和） | 產出一根 10:00，開高低收與成交量依合併規則 | `k_candle_bucket_domain.go:35` | `k_candle_bucket_domain_test.go:33 TestKCandleBucketDomainMergesTheCandlesItHolds` | asserts-oracle | produces-oracle | ✅ conforms（合併規則不依刻度分支；測試以 1h 格驗證同一段程式） |
| AC-09 | 一分鐘是六種可選刻度之一 | 接受並以一分鐘刻度回覆 | `aggregation_interval_vo.go:13` + `aggregation_interval_domain.go:29` | `aggregation_interval_domain_test.go:.../one minute` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 不在六種之內的彙總刻度被拒絕（3m） | 拒絕整次查詢，並列出六種可選刻度 | `aggregation_interval_domain.go:69` | `aggregation_interval_domain_test.go:TestNewAggregationIntervalDomainRefusesAnythingElse` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 刻度區間內只有部分分鐘有 K 線時仍然彙總得出來 | 產出一根 10:00，開高低收與成交量全部來自那一根 | `k_candle_bucket_domain.go:35` | `k_candle_bucket_domain_test.go:97 ...HoldingOneCandleKeepsItsFiguresAsTheyAre` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 刻度區間內完全沒有 K 線時不產出那一根 | 序列裡沒有 10:00 那根，不補洞、不沿用 | `k_candle_series_domain.go` | `k_candle_series_domain_test.go:153 ...BucketsLeaveOutTheStretchesNothingFellInto`；`k_candle_service_test.go:"leaves out a bucket nothing fell into"` | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 — 自動抓取每分鐘一輪

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-13 | 一輪抓取只取到最新一根已收完的為止（現在 10:07:20） | 存入到 10:06 為止；10:07 那根不存 | `k_candle_ingestion_domain.go:49` | `k_candle_ingestion_service_test.go:TestScheduledRoundStoresTheNewestClosedCandles/the newest closed candle is stored`、`/the candle still running is left out` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 整分鐘那一刻，最新一根已收完的是前一分鐘那根 | 09:08:00 時最新已收完為 09:07 | `k_candle_ingestion_domain.go:49` | `k_candle_ingestion_domain_test.go:.../exactly on the mark, so the interval just finished` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 行情來源夾帶進行中的那根時逐根排除 | 已收完的存入，進行中的不存 | `k_candle_ingestion_domain.go:96 SelectClosed` | `k_candle_ingestion_domain_test.go:TestSelectClosedDropsTheCandleStillRunning`；`k_candle_ingestion_service_test.go:.../the candle still running is left out` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 抓取間隔與 K 線長度一致，且不提供任何調整方式 | 每輪相隔一分鐘；沒有設定可改 | `k_candle_ingestion_job.go:19` | `k_candle_ingestion_job_test.go:157 TestTheIntervalBetweenRoundsIsTheLengthOneKCandleCovers` | asserts-oracle（間隔值）；「不可調整」為結構性事實，由「常數且未接設定」保證，測試無從斷言 | produces-oracle | ✅ conforms |

### US-04 — 台股交易時段內取得完整到收盤前一分鐘

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-17 | 台股盤中一輪抓取落在交易時段之內 | 取回的起始時間全部落在當日 09:00–13:29 | `market_domain.go:65 ClampToTradingSession` | `market_domain_test.go:"a round in the middle of the session"` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 收盤剛過的那一輪仍取回當日最後一根（13:31 時） | 仍取回當日 13:29 那根 | `market_domain.go:227` | `market_domain_test.go:"the round just after the close still reaches the day's last candle"`；`k_candle_ingestion_service_test.go:TestCatchingOneSymbolUpAsksForItsOwnGapAfterTheCloseHasPassed` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 收盤那一刻不構成一根 K 線（來源提供 13:30 的成交） | 13:30 那根不被存入；當日最新一根是 13:29 | `market_domain.go:227`（視窗上界＝收盤−一根）＋ `fugle_market_data_proxy.go:79`（丟掉視窗外的） | `fugle_market_data_proxy_test.go:TestFugleLeavesOutTheClosingAuctionThatEndsATaiwanSession`（來源同時給出 13:28／13:29／13:30，斷言只回前兩根）；`market_domain_test.go:"a window reaching past the close stops at the last candle"` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 非交易日整個跳過 | 該標的整個跳過，不算失敗、不留失敗紀錄 | `market_domain.go:74`（空視窗）＋ `k_candle_ingestion_service.go:291` | `market_domain_test.go:"a sunday round covers nothing"`；`k_candle_ingestion_service_test.go:TestBackfillAsksForNothingWhenNothingInReachCouldTrade` | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 — 即時跟盤送出一分鐘的進行中 K 線

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-21 | 送出目前這一分鐘進行中的那一根 | 送出起始時間 10:07 的進行中 K 線 | `fugle_forming_k_candle.go:53`（以 `KCandleInterval` 截斷） | `fugle_live_market_data_proxy_test.go:TestAPushedCandleIsReportedInTheShapeTheDomainKnows` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 進入下一分鐘時先送出前一根的最終樣子並將它存入 | 先送出前一根（已完成），再送出新的進行中那根；前一根被存入 | `fugle_forming_k_candle.go:63`；`k_candle_follow_service.go:444` | `fugle_live_market_data_proxy_test.go:TestACandleIsReportedFinishedOnlyOnceALaterOneArrives`（送出順序＋Closed）；`k_candle_follow_service_test.go`（走完的那根落地） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 同一分鐘被重複推送時取代而非累加 | 成交量為 800，不會出現第二根 10:07 | `fugle_forming_k_candle.go:71` | `fugle_live_market_data_proxy_test.go:TestARepeatOfTheSamePushDoesNotCountTwice` | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 — 切換時由人清除既有的五分鐘 K 線

> 這條 user story 描述的是**部署時的人工步驟**，不是系統行為。以下三條的「Impl」欄因此指向說明文件而非程式碼，並註明為何不由程式承擔。

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-24 | 清乾淨之後啟動，留存的只有一分鐘 K 線 | 啟動回補依既有上限取回一分鐘 K 線；留存的全部涵蓋一分鐘 | 既有啟動回補（`k_candle_ingestion_service.go:RunBackfill`）＋ `README.md`「切換 K 線長度時要先清掉舊資料」 | `k_candle_ingestion_service_test.go:TestBackfillAsksOnlyForTheGap`（回補視窗以一分鐘為步進） | asserts-oracle（可自動驗的那一半：回補行為）；清除本身是人工步驟，無從以測試斷言 | produces-oracle | ✅ conforms |
| AC-25 | 清除只針對 K 線，其他留存資料不受影響 | 交易標的、策略、使用者完全不受影響 | `README.md` 部署步驟明確只針對 `KCandles` 一張表 | — | no-test（人工步驟） | produces-oracle（其他資料存於不同資料表，指令未觸及） | 🟡 partial |
| AC-26 | 沒有清乾淨就啟動，兩種長度會混在一起而且分不出來 | 兩種長度並存且系統無從分辨；指標與回測會當成同一種東西計算 | `entities/k_candle.go`（沒有記錄長度的欄位——這正是「分不出來」的成因）；`PRD.md` §1 Out of Scope 明列不做「兩種長度並存」 | — | no-test | produces-oracle（此為**刻意接受的後果**，不是待修的缺陷；PRD §7 已列為風險） | 🟡 partial |

### Core Business Rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-01 | 一根 K 線固定涵蓋一分鐘 | 系統唯一的 K 線長度為一分鐘 | `k_candle_domain.go:20 KCandleInterval` | 由 AC-01～AC-05、AC-13～AC-16 連帶覆蓋 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 起始時間必須落在一分鐘刻度上；秒與更細一律為零 | 不在刻度上即拒絕並說明原因 | `k_candle_domain.go:44` | `k_candle_domain_test.go` 兩個拒絕案例 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 其餘 K 線規則完全不變 | 高低、負值、未來、唯一鍵、覆蓋語意皆不變 | `k_candle_domain.go:52-`；`k_candle_repository.go:41` | `k_candle_domain_test.go`；`k_candle_repository_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-04 | 可選彙總刻度為六種，未指定視為一分鐘，不在六種內即拒絕並列出 | 六種清單與預設值 | `aggregation_interval_domain.go:28,58` | `aggregation_interval_domain_test.go` 三支 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 「彙總後每根等於原本那根」屬於最短刻度（現為一分鐘） | 一分鐘下彙總為 identity；五分鐘為真正彙總 | `aggregation_interval_domain.go:29`；`k_candle_bucket_domain.go` | `k_candle_service_test.go:"aggregating at one minute leaves every candle as it was"` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 刻度區間對齊基準不變（UTC 當日零點）；六種都整除一天 | 由零點依刻度長度切分 | `aggregation_interval_domain.go:95 BucketStart` | `aggregation_interval_domain_test.go:TestAggregationIntervalDomainBucketStartCutsFromMidnight` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 自動抓取每一分鐘一輪，固定不可調整 | 同 AC-16 | `k_candle_ingestion_job.go:19` | `k_candle_ingestion_job_test.go:157` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 每輪取回根數與回補上限沿用現值 | 預設仍為 5 根／24 小時 | `application_config.go:194-196` | `internal/config/tests/ingestion_config_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-09 | 交易時段最新一根＝收盤時間減一根長度（台股 13:29） | 台股當日最後一根為 13:29 | `market_domain.go:227` | `market_domain_test.go` 三個案例 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 即時跟盤送出一分鐘的進行中 K 線；折疊規則保留 | 折疊仍成立，一分鐘下自然一對一 | `fugle_forming_k_candle.go:50` | `fugle_live_market_data_proxy_test.go:TestPushesInsideOneSlotAreFoldedIntoOneCandle`（以同一分鐘內不同秒數的兩次推送驗折疊仍成立） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 既有 K 線由人手動清除一次，且須在啟動新版之前完成；系統不自動清除 | 系統不含任何自動清除既有 K 線的路徑；清除步驟與順序寫在部署說明 | 全 codebase 無清除既有 K 線的路徑（`IKCandleRepository` 只有指名單根的 `Delete`）；`README.md` 部署說明 | — | no-test（「系統不做某件事」無從以行為測試斷言） | produces-oracle | 🟡 partial |

### Non-Functional

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-01 | 資料量變為原本五倍，以目前規模無疑慮 | 無行為要求 | — | — | no-test | 不適用（容量陳述） | ❔ unclear（非行為條款，無從以程式驗證） |
| NFR-02 | 每分鐘對每個交易標的提問一次；台股來源有每分鐘提問上限 | 每輪每標的一次抓取呼叫 | `k_candle_ingestion_service.go:240`（每標的一次 `FetchKCandles`） | `k_candle_ingestion_service_test.go:TestScheduledRoundCoversEveryWatchedSymbol`（`.Times(2)`，兩檔各一次） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-03 | 不涉及任何權限或身分規則的變更 | 認證授權相關程式碼未改動 | — | — | no-test | produces-oracle（本分支 diff 未觸及 `security/`、`session`、`user`） | 🟡 partial（以 diff 範圍佐證，無專屬測試） |
| NFR-04 | 兩個行情來源都提供一分鐘資料，取得方式相同 | Binance 要 `1m`、Fugle 要 `timeframe=1`，路徑與現況相同 | `binance_market_data_proxy.go:23`；`fugle_market_data_proxy.go:18` | `binance_market_data_proxy_test.go:87`；`fugle_market_data_proxy_test.go:183` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| — | 本次無 code orphan：所有新增／修改的公開行為都對應到條款 | — |
| ~~15 處註解／提示詞仍寫「五分鐘」~~ | 註解與 AI 提示詞的用字落後於行為 | **已修正**（commit `bdf03b1`）。`claude_assistant_proxy.go` 那一行會直接影響助手的用詞，是其中最要緊的一處 |
| `postman` 的「策略預設 `aggregationInterval` 為 `5m`」斷言 | 策略早已不帶彙總刻度（欄位已退場） | **既有漂移，與本切片無關**——本次未動它，留給後續處理 |

## Summary

- Conforms: 37/41 clauses ✅（90.2%）
- Violations: 無 🔴
- Mis-asserted: 無 🟠（原 `AC-19` 缺的那一支已補上：`TestFugleLeavesOutTheClosingAuctionThatEndsATaiwanSession`）
- Partial: `AC-25`、`AC-26`、`BR-11`、`NFR-03` 🟡（四條都在描述「人工步驟」或「系統刻意不做某事」，本質上無從以自動化測試斷言，非缺測試）
- Gaps: 無 ❌
- Unclear: `NFR-01` ❔（容量陳述，非行為條款）
- Orphans: 2 類，皆為文件而非程式（「註解仍寫五分鐘」已於 `bdf03b1` 修正；postman 的策略欄位斷言為既有漂移，與本切片無關）
