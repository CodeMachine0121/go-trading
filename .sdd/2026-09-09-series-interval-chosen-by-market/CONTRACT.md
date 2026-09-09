# Contract Traceability Matrix — 每根多長由市場的作息決定

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/domains/`, `internal/domain/service/`, `internal/controller/`
Oracle: Acceptance Criteria (19 scenarios) + Core Business Rules (7) + Non-Functional (2) = 28 clauses

Bridging notes（UL-MAP + ARCH）：
- 「可顯示根數」→ `KCandleSeriesQueryDto.DisplayableCandleCount`（指標：`nil` 是沒說）；HTTP 上是 `displayableCandleCount`。
- 「由系統挑的彙總刻度」→ `NewFittingAggregationIntervalDomain(tradingTime, capacity)`；回應的 `interval` 欄位說出挑到哪一種。
- 「一段裡有幾根」→ `MarketDomain.TradingTimeBetween` ＋ `AggregationIntervalDomain.SlotCount`（與指標計算同一對）。
- 讀取上限 `SourceCandleLimit()` =（根數＋一格）× 一格幾根，所以測試由上限反推根數。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 台股看一整天,挑到一分鐘 | 一分鐘 | `market_domain.go:100` + `aggregation_interval_domain.go:124` | `k_candle_series_query_domain_test.go`（台股看一整天）＋`k_candle_service_test.go`（會收盤的市場…挑得到一分鐘，並反推 271） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 加密貨幣看同樣的一整天,挑到五分鐘 | 五分鐘 | 同上（never-closes 分支） | 同上（加密貨幣看同樣的一整天／全天候市場…退到五分鐘） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 剛好擺得下（400 分鐘交易、400 格） | 一分鐘 | `aggregation_interval_domain.go:124` | `aggregation_interval_domain_test.go`（邊界：剛好 400 分鐘、剛好 400 格） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 多一根就擺不下（401 分鐘） | 五分鐘 | 同上 | `aggregation_interval_domain_test.go`（邊界：多一分鐘就擺不下） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 台股看五個交易日（1350 分鐘） | 五分鐘 | 同上 | `aggregation_interval_domain_test.go`（五個台股交易日）＋`k_candle_series_query_domain_test.go`（台股看五個交易日） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 連最粗的都擺不下 | 一天 | 同上（取最粗的退路） | `aggregation_interval_domain_test.go`（長到連一天一根都擺不下） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 只擺得下一根 | 一天 | 同上 | `aggregation_interval_domain_test.go`（邊界：只擺得下一根）＋`k_candle_series_query_domain_test.go`（同名，整天的全天候行情） | asserts-oracle（**這一條的期待值一開始寫錯**：以四個半小時的一段去問「只擺得下一根」，正確答案是四小時而不是一天。測試抓到了，改成一整天的一段才是條款講的情形） | produces-oracle | ✅ conforms |
| AC-08 | 指定了刻度就照指定的算 | 五分鐘 | `k_candle_series_query_domain.go:81`（宣告路徑，未改） | 既有 `k_candle_series_query_domain_test.go`（接受落在上限內的區間，三列各指定刻度） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 兩種都不說時視為一分鐘 | 一分鐘 | 同上 | 既有 `TestNewKCandleSeriesQueryDomainDeclaringNoIntervalMeansOneMinute`＋`k_candle_controller_test.go`（什麼都不說仍然是一分鐘） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 兩種都給就拒絕 | 整次拒絕，說明只能挑一種 | `k_candle_series_query_domain.go:92` | `k_candle_series_query_domain_test.go`（兩種問法只能挑一種）＋`k_candle_controller_test.go`（兩種問法都給就回 400） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 指定一種不存在的刻度仍照原樣拒絕 | 整次拒絕，並列出六種 | 既有 `NewAggregationIntervalDomain` | 既有 `TestNewKCandleSeriesQueryDomainRefusesAnIntervalNobodyOffers` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 說擺得下的根數超過系統一次願意答的 | 以單次上限為準挑，且挑出來的不被「一次要太多」拒絕 | `k_candle_series_query_domain.go:107`（`min`） | `k_candle_series_query_domain_test.go`（`TestAChosenIntervalIsNeverRefusedByTheSystemsOwnCeiling`，說 5000 而上限 1000 → 十五分鐘且未被拒） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 自己指定的刻度切太多根,仍照原樣拒絕 | 整次拒絕，提示縮小區間或改用更長的刻度 | `k_candle_series_query_domain.go:62` | 既有 `TestNewKCandleSeriesQueryDomainRefusesARangeCutIntoTooManyBuckets`＋`TestTheCeilingCountsTradingTimeToo`（加密貨幣一分鐘看一整天仍然要太多） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 擺得下的根數不大於零 | 整次拒絕，說明必須大於零 | `k_candle_series_query_domain.go:102` | `k_candle_series_query_domain_test.go`（零／負數兩列）＋`k_candle_controller_test.go`（擺得下零根就回 400） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 台股看整個週六 | 回空序列，不是拒絕 | `k_candle_series_query_domain.go:44`（沒有任何拒絕分支被觸發） | `k_candle_series_query_domain_test.go`（`TestAStretchWithNoTradingIsNotRefused`／整個週六） | asserts-oracle（釘住「沒有被拒絕」與「序列是空的」；真正讀不到 K 線是既有讀取路徑，本切片未改） | produces-oracle | ✅ conforms |
| AC-16 | 台股看整段收盤後的時間 | 回空序列，不是拒絕 | 同上 | 同上（整段落在收盤之後） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 加密貨幣看同一個週六 | 照常回那一天的 K 線 | `market_domain.go:100`（never-closes 分支） | `market_domain_test.go`（永不收盤的市場：a whole Saturday → 24h） | asserts-oracle（全天候市場沒有星期的概念，週六與任何一天走同一條路） | produces-oracle | ✅ conforms |
| AC-18 | 與指標計算問到同一個數字（一個交易日、五分鐘 → 54 根） | 54 根 | 兩條路共用 `TradingTimeBetween`＋`SlotCount` | `aggregation_interval_domain_test.go`（a Taiwan session at five minutes → 54）＋指標那一側的 `TestLookBackStillReachesBackPastTheClose`（同樣 54） | asserts-oracle（同一個函式、同一個數字，兩邊各自釘住） | produces-oracle | ✅ conforms |
| AC-19 | 涵蓋國定假日時不預先扣除 | 照交易日推算，不查假日名單 | `market_domain.go:336`（`eachTradingDaySession` 不查名單） | `k_candle_series_query_domain_test.go`（`TestSeriesQueryDoesNotDeductHolidaysWhenPickingTheInterval`，三個平日 → 162 格 → 五分鐘）＋指標那一側的同名案例 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 兩種問法互斥；都不給視為一分鐘 | 同 AC-09/10 | `k_candle_series_query_domain.go:81` | 同 AC-09/10 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 挑法：最細且擺得下；六種都超過取最粗 | 同 AC-03～07 | `aggregation_interval_domain.go:124` | `aggregation_interval_domain_test.go`（九列窮舉） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 一段裡有幾根照交易時段數；全天候整段都算 | 同 AC-01/02 | `market_domain.go:100` | `market_domain_test.go`（兩張表，含邊界與零） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 可顯示根數與單次上限取較嚴的那一個 | 同 AC-12 | `k_candle_series_query_domain.go:107` | 同 AC-12 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 可顯示根數必須大於零 | 同 AC-14 | `k_candle_series_query_domain.go:102` | 同 AC-14 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 那一段裡完全沒有交易不是錯誤 | 同 AC-15/16 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 其餘不變（沒有 K 線的刻度區間不產出、六種刻度、彙總算法、回應說出實際刻度） | 既有行為不變 | 未改動 | 既有整套（domain／service／controller／persistence 全綠） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 挑刻度最多走六種、成本與天數同階、**不讀任何 K 線** | 走訪次數與天數同階；挑刻度期間不觸及儲存 | `aggregation_interval_domain.go:124`（六次迴圈）、`market_domain.go:336`（逐「日」）、挑刻度全在建構子內 | 「不讀 K 線」由拒絕案例間接釘住：`k_candle_controller_test.go` 的三個 400 案例都沒有為讀取設任何期待，一旦讀了就會失敗 | asserts-oracle（就「不讀 K 線」而言）；複雜度以閱讀驗證 | produces-oracle | ✅ conforms |
| NFR-2 | 交易時段沿用既有設定，不新增設定項；既有呼叫端不需改動 | `cmd/server/config.go` 無新增變數；指定刻度的呼叫端不受影響 | `config.go` 未改動 | 既有呼叫端的整套測試（助手查詢、指標計算、回測）全綠且未改一行請求 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `k_candle_controller.go:207` | 可顯示根數不是整數時，控制器自己回 400 並指名欄位 | undocumented（傳輸層的讀取失敗，與其他參數同一種處理）——PRD 只談「不大於零」。建議視為既有慣例的一部分，不另立條款 |
| `aggregation_interval_domain.go:145` | 「找最粗的那一種」抽成共用的私有函式，兩個建構子共用 | 重構，非行為——原本 `NewCoarsestAggregationIntervalDomain` 自己有一份 |

## Summary

- Conforms: 28/28 clauses ✅ (100%)
- Violations: 無
- Mis-asserted: 無
- Partial: 無（AC-19 於審查後補上序列這條路自己的案例）
- Gaps: 無
- Unclear: 無
- Orphans: 2（皆為傳輸層慣例與重構，非越界實作）

### 審查另外記下的一件事

**規格裡的一個數字原本是錯的。** BRIEF 與 PRD 一開始寫「台股看一天的二十四小時裡有約 390 分鐘在交易時段內」。
實際上一段二十四小時的視窗**恆為 270 分鐘**（剛好一個交易時段的長度），因為它總是把一個交易日切成兩半、
兩半加起來還是一整段。測試在 Phase 4 抓到這個矛盾（期待 391 根、實得 271 根），文件與測試都已更正為 270。
結論不變：270 ≤ 400，台股看一天照樣挑到一分鐘。

Note: static conformance audit against the Acceptance Criteria — it judges test
assertions and code paths against the spec's expected outcome, not by running the
full suite. For dynamic proof of a partial clause, drive it via `/tdd`.
