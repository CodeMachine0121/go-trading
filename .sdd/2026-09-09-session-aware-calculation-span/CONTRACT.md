# Contract Traceability Matrix — 會收盤的市場，一段時間裡有幾格

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/domains/`, `internal/domain/service/`, `internal/controller/`
Oracle: Acceptance Criteria (24 scenarios，含審查後回填的兩條邊界) + Core Business Rules (8) + Non-Functional (2) = 34 clauses
審查後已修：AC-17 條款改寫成程式實際說的話、AC-18/BR-5 補上測試、兩條邊界回填 PRD。

Bridging notes used throughout (UL-MAP + ARCH):
- 「要看幾格」→ `AggregationIntervalDomain.SlotCount(tradingTime)`；讀取上限 `SourceCandleLimit()` = （計算根數＋一格）× 一格幾根，所以測試由上限反推格數。
- 「這一段時間市場沒有交易」→ `ErrObservationWindowHoldsNoTrading`；HTTP 上是 `observationWindowHoldsNoTrading: true`。
- 「湊不出最少可算根數」→ `ErrIndicatorCalculationCandleCoverageTooThin`；「要太多」→ `ErrIndicatorCalculationCandleCountExceeded`。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 台股看一個完整交易日（09:00–13:30、5m） | 要看 54 格 | `aggregation_interval_domain.go:157` + `market_domain.go:94` | `indicator_calculation_domain_test.go:988`（no look-back declared） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 台股看整整二十四小時（涵蓋一個完整交易日、5m） | 要看 54 格，不是 288 格 | `indicator_calculation_domain.go:97` | `indicator_calculation_service_test.go:640`（a whole day of a market that shuts） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 觀察區間完全落在交易時段之內（11:00–12:00、5m） | 要看 12 格 | 同上 | `market_domain_test.go:414`（wholly inside a session → 1h）＋ `aggregation_interval_domain_test.go:287`（an hour at five minutes → 12） | asserts-oracle（兩段合起來釘住；整條路徑另在 1m 下以 60 格斷言） | produces-oracle | ✅ conforms |
| AC-04 | 觀察區間跨過一次收盤（13:00→隔日 10:00、5m） | 要看 18 格 | 同上 | `market_domain_test.go:414`（across one close → 1.5h）＋ `indicator_calculation_domain_test.go:916`（1m 下 90 格） | asserts-oracle（5m 的 18 由 floor 規則決定，該規則有 8 列窮舉） | produces-oracle | ✅ conforms |
| AC-05 | 觀察區間跨過週末（週五 12:00→週一 10:00、5m） | 要看 30 格 | 同上 | `market_domain_test.go:414`（across a weekend → 2.5h）＋ `indicator_calculation_domain_test.go:916`（1m 下 150 格） | asserts-oracle（同上） | produces-oracle | ✅ conforms |
| AC-06 | 觀察區間短到不滿一格 | 要看 1 格 | `aggregation_interval_domain.go:157` | `indicator_calculation_domain_test.go:916`（shorter than one slot）＋ `aggregation_interval_domain_test.go:287`（3 分鐘→1） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 加密貨幣看整整二十四小時（5m） | 要看 288 格 | `market_domain.go:96`（never-closes 分支） | `indicator_calculation_domain_test.go:969`＋`indicator_calculation_service_test.go:640`（1445 根） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 加密貨幣看深夜兩小時（5m） | 要看 24 格，且不被拒絕 | 同上 | `market_domain_test.go:488`（the middle of the night → 2h） | asserts-oracle（2h 交易時間 > 0 即不觸發拒絕；格數由 floor 規則決定） | produces-oracle | ✅ conforms |
| AC-09 | 加密貨幣看不滿一格的一段 | 要看 1 格 | `aggregation_interval_domain.go:157` | `aggregation_interval_domain_test.go:287`（3 分鐘→1） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 台股一個交易日（54 格）＋回看 20 | 計算根數 73；不足的 19 根取自前一交易日；54 個位置都有值 | `indicator_calculation_domain.go:113` | `indicator_calculation_domain_test.go:988`（第一列） | asserts-oracle（73 直接斷言）；「取自前一交易日」是既有的**按根數往回讀**所致，非本切片新增 | produces-oracle | ✅ conforms |
| AC-11 | 台股開盤後一小時（12 格）＋回看 20 | 計算根數 31 | 同上 | `indicator_calculation_domain_test.go:988`（第二列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 策略沒有宣告回看根數 | 計算根數 54，不多取一根 | 同上 | `indicator_calculation_domain_test.go:988`（第三列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 更早的行情不存在（可用 54、需要 73） | 以 54 根執行；回報需要 73、實際採用 54 | `indicator_calculation_domain.go:220`（既有 `SelectInputCandles`） | `indicator_calculation_service_test.go`（湊不滿以可用根數執行，既有案例） | asserts-oracle（回報路徑與市場無關，既有案例已釘住；73 的來源由 AC-10 釘住） | produces-oracle | ✅ conforms |
| AC-14 | 觀察區間完全落在收盤後 | 整次拒絕，說明這一段時間市場沒有交易，且呼叫端分辨得出 | `indicator_calculation_domain.go:98`、`indicator_calculation_errors.go:119`、`indicator_calculation_controller.go:70` | `indicator_calculation_domain_test.go:1033`＋`indicator_calculation_controller_test.go:367` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 觀察區間是整個週六 | 整次拒絕，說明這一段時間市場沒有交易 | 同上 | 同上（兩個測試各有這一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 市場有交易但可用根數 19 < 最少可算根數 20 | 整次拒絕，說出 19 與 20，且與「沒有交易」分辨得開 | 既有 `indicator_calculation_domain.go:238` | `indicator_calculation_service_test.go`（既有 too-thin 案例）＋`indicator_calculation_domain_test.go:1033`（`NotErrorIs` too-thin） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 計算根數超過單次上限 | 照原樣整次拒絕，說出用到幾根與上限是多少；指出要改的是觀察區間 | `indicator_calculation_errors.go:26`、`indicator_calculation_controller.go:60` | `indicator_calculation_controller_test.go:259` | asserts-oracle | produces-oracle | ✅ conforms（條款已於本次審查後改寫：原句沿用自 BRIEF 的「提示縮短觀察區間或改用粗一點的彙總刻度」，而這條路徑 PRD 自己寫明「照原樣不改」，既有訊息從來沒說過那兩條出路） |
| AC-18 | 觀察區間涵蓋三個平日、其中一天休市 | 要看 162 格（不預先扣除）；回報需要 162、實際採用 108 | `market_domain.go:94`（只看星期與時段） | `indicator_calculation_domain_test.go:1097`（`TestHolidaysAreNotDeductedFromTheSlotsAskedFor`） | asserts-oracle（162 直接斷言；「回報 162/108」的那一半走既有的落差回報路徑，由 AC-13 的既有案例釘住） | produces-oracle | ✅ conforms |
| AC-19 | 涵蓋的平日全部正常交易 | 計算根數與實際採用根數相同 | 既有 `SelectInputCandles` | `indicator_calculation_service_test.go`（既有「湊得滿」案例） | asserts-oracle（此規則與市場無關） | produces-oracle | ✅ conforms |
| AC-20 | 觀察區間只給起點 | 終點視為現在 | `observation_window_domain.go:42` | `observation_window_domain_test.go:16`（no end named） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 觀察區間終點晚於現在 | 終點視為現在，不拒絕 | 同上 | `observation_window_domain_test.go:16`（an end that has not arrived） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 起點晚於終點 | 整次拒絕，說明起點不得晚於終點 | `observation_window_domain.go:52` | `observation_window_domain_test.go:61`（it ends before it begins） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 要看幾格＝觀察區間與交易時段重疊的總長 ÷ 彙總刻度，向下取整，最少一格；無交易時段的市場整段都算 | 同上 | `market_domain.go:94`＋`aggregation_interval_domain.go:157` | `market_domain_test.go:414/488`＋`aggregation_interval_domain_test.go:287` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 重疊為零即拒絕，且呼叫端辨認得出 | 同 AC-14 | `indicator_calculation_domain.go:98` | `indicator_calculation_domain_test.go:1033` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 計算根數＝要看幾格 ＋ 最大回看根數 − 1（無宣告則等於格數） | 同 AC-10/12 | `indicator_calculation_domain.go:113` | `indicator_calculation_domain_test.go:988` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 回看取用的歷史不受觀察區間限制，可跨收盤與交易日 | 讀取不以觀察區間起點為下界 | `indicator_calculation_domain.go:185`（`SourceCandleLimit` 只由根數決定）、`indicator_calculation_service.go:86`（`FindLatestBefore` 只帶截止時間與根數） | `indicator_calculation_domain_test.go:988` | asserts-oracle（斷言的是要讀幾根；「不以起點為下界」由讀取介面的形狀保證——它根本收不到起點） | produces-oracle | ✅ conforms |
| BR-5 | 休市日不預先扣除 | 同 AC-18 | `market_domain.go:328`（`eachTradingDaySession` 不查假日名單） | `indicator_calculation_domain_test.go:1097` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 觀察區間終點即計算截止時間 | 讀取截止時間由觀察區間的終點決定 | `indicator_calculation_domain.go:145`（`endTime` 取自 window）、`:176`（`ReadCutoff`） | `indicator_calculation_service_test.go`（an end time already past is read up to） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 既有規則全部不變（湊不滿以可用根數執行、too thin 拒絕、超過上限拒絕、刻度未指定視為一分鐘、只採用走完的刻度區間） | 既有行為不變 | 未改動 | 既有整套 domain/service/controller 測試 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 推算格數走過的天數最多等於觀察區間橫跨的天數，不得逐格掃描 | 走訪次數與區間天數同階 | `market_domain.go:328`（逐「日」迴圈，非逐格） | — | no-test（複雜度以閱讀驗證，非執行） | produces-oracle | 🟡 partial |
| NFR-2 | 交易時段沿用既有設定，不新增設定項 | `cmd/server/config.go` 無新增變數 | `config.go` 未改動（`git diff` 為空） | — | no-test | produces-oracle | 🟡 partial |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `observation_window_domain.go:52` | 起點**等於**終點（長度為零的區間）也被拒絕 | **已回填**：PRD US-06「起點就是終點」與 BR「觀察區間必須有長度」 |
| `observation_window_domain.go:47` | 未指定起點即拒絕 | **已回填**：PRD US-06「沒有說要看哪一段」 |
| `indicator_calculation_controller.go:63` | 「要太多」的回應把 `field` 從 `candleCount` 改為 `startTime` | undocumented — 對外形狀改變，PRD 未描述（與 AC-17 相關） |

## Summary（審查後的最終狀態）

- Conforms: 32/34 clauses ✅ (94%)
- Violations: 無
- Mis-asserted: 無（AC-17 已由改寫條款解決——改的是規格，不是程式：那條路徑 PRD 自己寫明照原樣不改）
- Partial: NFR-1、NFR-2（結構性條款：走訪次數與「沒有新增設定項」以閱讀與 `git diff` 驗證，不寫測試）
- Gaps: 無
- Unclear: 無
- Orphans: 1（`field` 由 `candleCount` 改為 `startTime`——對外形狀的改變，屬於 ARCH 的範圍，PRD 是業務語言不描述欄位名）

新增的兩條邊界條款（起點等於終點、沒給起點）由 `observation_window_domain_test.go:61` 的
「the boundary: it begins exactly where it ends」與「no beginning named」兩列釘住，皆 ✅。

Note: static conformance audit against the Acceptance Criteria — it judges test
assertions and code paths against the spec's expected outcome, not by running the
full suite. For dynamic proof of a partial clause, drive it via `/tdd`.
