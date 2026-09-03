# 策略與執行參數分離 — Contract Verification

**Contract source:** `.sdd/2026-09-03-strategy-execution-parameters/PRD.md`（Section 3 驗收條件）
**Design map:** `ARCH.md`（同資料夾）
**Verified:** 2026-09-03
**Ceiling:** 靜態一致性稽核——逐條把**測試的斷言**與**程式碼路徑**各自對照 PRD 導出的預期結果，
不靠整套測試的綠燈下判斷，也不執行自行發明的情境。

---

## 1. Clauses

**Oracle** 一律先只讀 PRD 導出，再去對程式碼；`code` 欄記最具體的實作位置。

### US-01 — 策略只記住一套算法

| ID | Clause | Oracle（由 PRD 導出） | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01.1 | 新建立的策略不再帶著取數計畫 | 建立成功；讀回只有名稱、算式、種類與兩個時間，沒有彙總刻度與計算根數 | `entities/strategy.go:20`、`domains/strategy_domain.go:30` | `entities/tests/strategy_test.go:TestStrategyToDto`、`controller/tests:answers created and hands back the strategy` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.2 | 既有策略的算法原封不動 | 名稱、算式、種類、建立時間與先前完全相同；回覆不出現那兩樣 | `persistence/schema_migrator.go:74` | `persistence/tests:TestSchemaMigratorLeavesTheAlgorithmAloneWhileDroppingThePlan` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.3 | 附上取數計畫也不會被記住 | 建立成功；讀回不出現那兩樣 | `controller/models/strategy_request.go:14` | `controller/tests:a plan for feeding the algorithm is not part of a strategy` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.4 | 修改策略時沒有取數計畫可以一起改 | 修改成功、最後修改時間更新、回覆不出現那兩樣 | `dto/strategy_write_dto.go` | `application/tests:rewrites the strategy the write names` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.5 | 名稱為空白仍然被拒絕 | 拒絕，說明必須給策略取一個名稱 | `domains/strategy_domain.go:46` | `domains/tests:TestNewStrategyDomainRefusesContentThatBreaksARule/no name at all` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.6 | 名稱重複仍然被拒絕 | 拒絕、說明名稱已被使用；既有那一支不受影響 | `persistence/strategy_repository.go` 唯一索引 | `application/tests:reports a name another strategy already holds` | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 — 一次計算自己說要吃什麼

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-02.1 | 依指定的彙總刻度與根數取數 | 算式收到 24 根一小時的彙總 K 線，由早到晚 | `domains/indicator_calculation_domain.go:153` | `application/tests:reads at the coarseness asked for, up to the stretch that has finished` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.2 | 較細的刻度看見更多細節 | 288 根五分鐘 K 線；根數與刻度無關 | 同上 | `domains/tests:TestSourceCandleLimitCoversTheBucketsAskedForPlusOneSpare` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.3 | 未指定彙總刻度時視為五分鐘 | 收到 20 根五分鐘 K 線 | `domains/aggregation_interval_domain.go:NewAggregationIntervalDomain` | `domains/tests:TestNewIndicatorCalculationDomainReadsTheDeclaredInterval` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.4 | 根數的上限數的是彙總後的根數 | 一天刻度 × 1000 根被接受 | `domains/indicator_calculation_domain.go:60` | `domains/tests:TestNewIndicatorCalculationDomainCountsAggregatedCandlesNotStoredOnes` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.5 | 超過單次可用的最大根數即拒絕 | 拒絕，說明超過單次可用的最大根數 | 同上 | `domains/tests:TestNewIndicatorCalculationDomainRejectsBrokenRequests` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.6 | 計算根數必須大於零 | 拒絕，說明計算根數必須大於零 | `domains/indicator_calculation_domain.go:53` | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.7 | 彙總刻度只認得那五種 | 拒絕，列出五種可選刻度 | `domains/indicator_calculation_domain.go:68` | 同上 + `controller/tests:reports an interval nobody offers as a bad request` | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 — 只採用走完的刻度區間

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-03.1 | 還在走的那一格不採用 | 現在 08:37、一小時：採用 05/06/07:00，不含 08:00 | `domains/indicator_calculation_domain.go:132` | `domains/tests:TestReadCutoffStopsBeforeTheBucketStillRunning` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.2 | 截止時間落在邊界上時前一格算走完 | 截止 08:00：採用 05/06/07:00 | 同上 + `persistence/k_candle_repository.go:163`（嚴格小於） | 同上 + `persistence/tests:a candle opening exactly at the cut-off is left out` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.3 | 五分鐘刻度下等同排除最新一根 | 採用 08:20/25/30，不含 08:35 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.4 | 過去的半成品仍然是半成品 | 截止 2025-03-01 14:30：採用 12/13:00，不含 14:00 | 同上 | 同上 + `controller/tests:reads at the coarseness and up to the moment the body named` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.5 | 未指定截止時間時視為現在 | 行為同 AC-03.1 | `domains/indicator_calculation_domain.go:95` | 同上（`endTime` 零值案例） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.6 | 截止時間指向未來時視同現在，不拒絕 | 行為同 AC-03.1，且不被拒絕 | 同上 | `domains/tests:TestReadCutoffTreatsAnEndTimeThatHasNotArrivedAsNow`、`service/tests:an end time that has not arrived is read up to now` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.7 | 一天刻度下只採用已經過完的那一天 | 採用 09-02 零點那一根，不含 09-03 | `domains/aggregation_interval_domain.go:BucketStart` | `domains/tests:TestReadCutoffStopsBeforeTheBucketStillRunning/a day still being lived through` | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 — 湊不滿就說湊不滿

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-04.1 | 走完的格子湊得滿就算得成 | 算式收到 3 根 | `domains/indicator_calculation_domain.go:153` | `domains/tests:TestSelectInputCandlesKeepsEveryBucketAReadThatCameUpShortFound` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.2 | 湊不滿即整次拒絕，不回覆任何部分結果 | 拒絕，說明湊得出 2 根、要求 3 根；無指標值 | `domains/indicator_calculation_domain.go:166` | `domains/tests:TestSelectInputCandlesRefusesWhenTooFewBucketsAreThere` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.3 | 中間沒有資料的那一格不補洞，但不妨礙湊滿 | 採用 05:00 與 07:00；沒有任何一根是 06:00 | `domains/k_candle_series_domain.go:40` | `domains/tests:TestSelectInputCandlesSkipsTheStretchesWithNoMarketInThem`（斷言完整清單，等同排除 06:00） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.4 | 一根 K 線都沒有的交易標的 | 拒絕，說明湊不出 3 根 | 同 AC-04.2 | `domains/tests:.../no candles at all` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.5 | 刻度區間內只有一根也算數 | 收到 1 根，起始時間 07:00 | `domains/k_candle_bucket_domain.go:39` | `domains/tests:TestSelectInputCandlesCountsABucketHoldingOneCandle` | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 — 結果說得出第幾個值對應哪一根

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-05.1 | 回覆每一根起始時間，由早到晚，第 n 個值對應第 n 根 | 帶著 05/06/07:00 三個起始時間 | `service/indicator_calculation_service.go` 的 `openTimes` | `service/tests:names where each candle the script saw begins, earliest first` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.2 | 指標值只有一個時照樣回覆起始時間 | 仍帶著三個起始時間 | 同上 | `service/tests:names them even when the kind is a single number` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.3 | 回覆這次實際採用的彙總刻度與根數 | 刻度一小時、根數 3 | `dto/indicator_calculation_result_dto.go` | `service/tests:names the coarseness actually used, declared or not` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.4 | 一個指標名稱都沒放進結果仍是一次成功的計算 | 成功、空的一組、起始時間照樣回覆 | 同上 | `service/tests:names them even when the script produced nothing at all` | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 — 至少需要幾根由算式自己把關

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-06.1 | 系統不猜算法的最低需求 | 只要求 10 根就照常交出 10 根 | **刻意的空白**：`IndicatorCalculationDomain` 不含任何最低根數規則、也拿不到算式 | `domains/tests:TestSelectInputCandlesHandsOverExactlyWhatWasAskedForAndNeverGuessesAMinimum` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.2 | 算式自己檢查不足時整次計算失敗 | 拒絕整次計算，說明算式執行失敗的原因 | `script/yaegi_indicator_script_proxy.go`（未改動） | `service/tests:reports a script failure without any partial result` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.3 | 算式沒有自己檢查時照常回覆 | 成功並回覆算出來的值 | 同上 | `service/tests:reports the indicator values and how many candles were used` | asserts-oracle | produces-oracle | ✅ conforms |

---

## 2. Orphans

| Behavior | Site | 判定 |
| :--- | :--- | :--- |
| `AggregationIntervalDomain.SourceCandleCount` 的既有用途（彙總查詢的讀取上限） | `domains/k_candle_series_query_domain.go` | 非孤兒——屬於既有切片，本切片只是第二個使用者 |
| `KCandleSeriesDomain.Buckets()` 成為公開方法 | `domains/k_candle_series_domain.go:40` | ARCH §4 明列的改動；由 AC-04.3 與 AC-04.5 覆蓋 |
| `spareBucketCount` 多讀一格 | `domains/indicator_calculation_domain.go` | 非孤兒——它是 AC-04.1 與 AC-04.2 之所以能同時成立的機制；由 `TestSelectInputCandlesNeverHandsOverABucketTheReadCutInHalf` 釘住 |

**無違反 Out of Scope 的實作。** 特別確認：未新增「最低需求根數」欄位（PRD Out of Scope），
未在圖表上畫任何東西，未把交易標的記在策略身上，未遷移既有策略的那兩個欄位。

---

## 3. Summary

```
✅ 29 conforms · 🔴 0 violations · 🟠 0 mis-asserted · 🟡 0 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 0 orphans
Conformance: 100%
```

**本次稽核修掉的兩處**（皆為測試不足，程式碼本來就正確）：

- **AC-01.2**：原本只有「欄位被刪掉」的測試，沒有任何測試釘住「其餘內容原封不動」。
  補上 `TestSchemaMigratorLeavesTheAlgorithmAloneWhileDroppingThePlan`——
  刪欄位的遷移一旦寫錯範圍，就是在無聲地毀掉它本該保留的東西。
- **AC-06.1**：這一條的內容是**一個刻意的空白**，而空白最容易在日後被「好心」補上一條猜測。
  補上 `TestSelectInputCandlesHandsOverExactlyWhatWasAskedForAndNeverGuessesAMinimum` 把它釘住。

**測試覆蓋率**（`go test -coverpkg=./internal/...`）：本切片新增／改動的每一個函式皆 100%，
唯一例外是 `SchemaMigrator.dropRetiredColumns` 的 85.7%——未覆蓋的是「資料庫拒絕刪欄位」
那條錯誤分支，需要一個會失敗的資料庫才跑得到（既有的 `Migrate` 為相同形狀，76.9%）。
錯誤不吞、分支保留。
