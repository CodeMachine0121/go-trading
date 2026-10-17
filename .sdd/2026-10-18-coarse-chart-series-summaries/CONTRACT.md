# Contract Traceability Matrix — coarse-chart-series-summaries

Contract: PRD.md
Design map: ARCH.md
Implementation: go-trading `feature/coarse-chart-series-summaries`
Oracle: Acceptance Criteria (24) + Core Business Rules (8) + Non-Functional (2) = 34 clauses

> Static conformance audit: test assertions and code paths are judged against each clause's spec-derived oracle, not by running the suite.
> The first Jev pass on AC-1, 8, 9, 10, 12, 13, 15, 19, 20, 21, 22 and NFR-1 was given incomplete code references (repository methods without the invalidation component, readers without the plan); those clauses were re-audited with the full path and the second verdict is recorded (marked "recheck with full path").

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Jev test | Jev code | Status | Route |
|----|--------|------------------------|------|------|------------|------------|----------|----------|--------|-------|
| AC-1 | 九百天都已彙總好，日線很快回來：回來九百根日線，每一根每一項都與逐分鐘當場合併相同，且兩秒內回來 | 日線取日留存彙總轉成一分鐘形狀，經原合併規則合出 | `internal/domain/service/k_candle_contract_summarized_series_reader.go:28-100` | `internal/domain/service/tests/k_candle_summarized_series_test.go:113-134` | shallow | produces-oracle | shallow 0.48 | produces-oracle 0.19 | 🟠 mis-asserted | escalate (low confidence) (recheck with full path) |
| AC-2 | 四小時由小時彙總合併而成：六根四小時彙總K線，每根與逐分鐘合併相同 | SummaryInterval 4h→1h；四份小時留存彙總經原合併規則合併 | `internal/domain/models/domains/aggregation_interval_domain.go:123-142` | `internal/domain/service/tests/k_candle_summarized_series_test.go:135-158` | asserts-oracle | produces-oracle | asserts-oracle 0.75 | produces-oracle 0.65 | ✅ conforms | recheck (mid confidence) |
| AC-3 | 今天還在走的那一根當場合併：今天那根由今天零點起的一分鐘K線合併，昨天以前取留存彙總 | 尾段分鐘 [todayStart, end] 由 FindInRange 讀 | `internal/domain/models/domains/k_candle_summary_plan_domain.go:22-87` | `internal/domain/service/tests/k_candle_summarized_series_test.go:159-181` | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.56 | ✅ conforms | escalate (low confidence) |
| AC-4 | 今天的每一小時當場合併：昨天的小時取留存彙總，今天零點到十點的小時由今天的一分鐘K線合併 | summarizedEnd=todayStart；尾段分鐘合出今天的小時 | `internal/domain/models/domains/k_candle_summary_plan_domain.go:22-87` | `internal/domain/service/tests/k_candle_summarized_series_test.go:182-202` | asserts-oracle | produces-oracle | asserts-oracle 0.79 | produces-oracle 0.67 | ✅ conforms | recheck (mid confidence) |
| AC-5 | 細刻度照舊當場合併：十五分鐘五天每根由一分鐘K線合併，與現在相同 | UsesSummaries 為假時走原 FindInRange(SourceCandleLimit) | `internal/domain/service/k_candle_contract_summarized_series_reader.go:28-36` | `internal/domain/service/tests/k_candle_summarized_series_test.go:203-216` | asserts-oracle | produces-oracle | asserts-oracle 0.75 | produces-oracle 0.84 | ✅ conforms | recheck (mid confidence) |
| AC-6 | 一天裡有一分鐘缺指數價格，日線的指數價格就是缺 | 合約日留存彙總 IndexOpen..Close 為 null | `internal/domain/models/domains/k_candle_contract_summary_day_domain.go:23-63` | `internal/domain/models/domains/tests/k_candle_summary_day_domain_test.go:130-167` | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.43 | ✅ conforms | escalate (low confidence) |
| AC-7 | 四小時格子裡有一小時缺溢價指數，那一格溢價指數為缺，標記價格與成交數字照常合併 | 四份小時留存彙總其一 premium 為 null → 4h 那格 premium null，mark/volume 合併 | `internal/domain/models/domains/k_candle_contract_series_domain.go:53-135` | `internal/domain/service/tests/k_candle_summarized_series_test.go:135-158` | no-test | produces-oracle | no-test 0.76 | produces-oracle 0.88 | 🟡 partial | recheck (mid confidence) |
| AC-8 | 市場不報的成交數字維持不報：台股一天不報成交金額，日線成交金額為不報而非零 | QuoteVolume NullDecimal 無效 | `internal/domain/models/domains/k_candle_summary_day_domain.go:24-52` | `internal/domain/models/domains/tests/k_candle_summary_day_domain_test.go:77-129` | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.95 | ✅ conforms | accept (recheck with full path) |
| AC-9 | 只有部分分鐘報了的成交數字取有報的加總 | 小時留存彙總 QuoteVolume=有報的加總 | `internal/domain/models/domains/k_candle_summary_day_domain.go:24-52` | `internal/domain/models/domains/tests/k_candle_summary_day_domain_test.go:77-129` | asserts-oracle | produces-oracle | asserts-oracle 0.80 | produces-oracle 0.97 | ✅ conforms | recheck (mid confidence) (recheck with full path) |
| AC-10 | 沒有任何一分鐘K線的那一天不會出現：週末兩天沒有日線，也沒有留存彙總 | 無分鐘的天標記已彙總但無留存彙總，序列無該日 | `internal/domain/models/domains/k_candle_summary_day_domain.go:24-52` | `internal/domain/service/tests/k_candle_summarized_series_test.go:265-283` | asserts-oracle | produces-oracle | mis-asserted 0.19 | diverges 0.35 | 🔴 violation | escalate (violation) (recheck with full path) |
| AC-11 | 歷史同步補進一根，所屬天與小時作廢；之後日線成交量包含那一根 | SaveAllIfAbsent 同交易內 revision+1、summarized=false、刪該日留存彙總 | `internal/infrastructure/persistence/k_candle_repository.go:61-122` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:102-146` | asserts-oracle | produces-oracle | asserts-oracle 0.72 | produces-oracle 0.92 | ✅ conforms | recheck (mid confidence) |
| AC-12 | 合約晚補指數價格，所屬天與小時作廢；之後日線指數價格不再是缺 | 合約 SaveAllIfAbsent 晚補計入 RowsAffected → 作廢 | `internal/infrastructure/persistence/k_candle_contract_repository.go:69-138` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:203-226` | shallow | produces-oracle | asserts-oracle 0.79 | produces-oracle 0.41 | ✅ conforms | escalate (disagree) (recheck with full path) |
| AC-13 | 手動改寫一根，所屬天與小時作廢 | Update 同交易內作廢 | `internal/infrastructure/persistence/k_candle_repository.go:124-150` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:102-146` | asserts-oracle | produces-oracle | asserts-oracle 0.65 | produces-oracle 0.48 | ✅ conforms | escalate (low confidence) (recheck with full path) |
| AC-14 | 手動刪除一根，所屬天與小時作廢 | Delete 同交易內作廢 | `internal/infrastructure/persistence/k_candle_repository.go:246-266` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:102-146` | asserts-oracle | produces-oracle | asserts-oracle 0.76 | produces-oracle 0.45 | ✅ conforms | escalate (low confidence) |
| AC-15 | 例行抓取修正剛走完那一小時裡的一根：09:00那一小時與今天的留存彙總作廢 | Save 同交易內作廢該日（含該小時） | `internal/infrastructure/persistence/k_candle_repository.go:35-60` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:102-146` | asserts-oracle | produces-oracle | asserts-oracle 0.81 | produces-oracle 0.44 | ✅ conforms | escalate (low confidence) (recheck with full path) |
| AC-16 | 一天最後一分鐘變了只影響那一天：三月一日作廢，三月二日不受影響 | 作廢範圍 [day, day+24h) | `internal/infrastructure/persistence/k_candle_summary_invalidation.go:15-52` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:164-180` | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.97 | ✅ conforms | accept |
| AC-17 | 合併當中那一天被改動，這次結果不留下；下次重新合併含那次改寫 | SaveSummarized 以 revision 比對，不符回 false 不寫 | `internal/infrastructure/persistence/k_candle_summary_day_repository.go:80-127` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:181-202` | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.76 | ✅ conforms | recheck (mid confidence) |
| AC-18 | 只補缺的那幾天：三十天一天一天合併並留存，九百根日線與逐分鐘合併相同 | DaysToSummarize 只列未彙總的天 | `internal/domain/service/k_candle_contract_summarized_series_reader.go:58-100` | `internal/domain/service/tests/k_candle_summarized_series_test.go:217-264` | asserts-oracle | produces-oracle | asserts-oracle 0.85 | produces-oracle 0.51 | ✅ conforms | escalate (low confidence) |
| AC-19 | 一天都還沒彙總好，也只一天一天處理：同時只處理一天份，結果相同，之後再要每根取自留存彙總 | 逐天讀 ≤1440 根並 SaveSummarized | `internal/domain/service/k_candle_contract_summarized_series_reader.go:58-100` | `internal/domain/service/tests/k_candle_summarized_series_test.go:284-329` | asserts-oracle | produces-oracle | asserts-oracle 0.65 | diverges 0.36 | 🔴 violation | escalate (violation) (recheck with full path) |
| AC-20 | 補到一半失敗，已補好的不必重做：這次告知暫時無法取得，之前的天已有留存彙總 | 讀某天失敗即回傳錯誤；之前各天已各自 SaveSummarized | `internal/domain/service/k_candle_contract_summarized_series_reader.go:58-100` | `internal/domain/service/tests/k_candle_summarized_series_test.go:284-329` | asserts-oracle | produces-oracle | asserts-oracle 0.57 | produces-oracle 0.33 | ✅ conforms | escalate (low confidence) (recheck with full path) |
| AC-21 | 剛走完的一天連同它的每一小時被補好（觀察清單現貨） | 背景輪對昨天補日與各小時留存彙總 | `internal/domain/service/k_candle_summary_service.go:51-98` | `internal/domain/service/tests/k_candle_summary_service_test.go:59-78` | asserts-oracle | produces-oracle | mis-asserted 0.37 | produces-oracle 0.63 | 🟠 mis-asserted | escalate (disagree) (recheck with full path) |
| AC-22 | 剛走完的一天被補好（合約追蹤名單） | 背景輪對合約昨天補留存彙總，已彙總的天跳過 | `internal/domain/service/k_candle_summary_service.go:51-98` | `internal/domain/service/tests/k_candle_summary_service_test.go:79-96` | asserts-oracle | produces-oracle | asserts-oracle 0.81 | produces-oracle 0.83 | ✅ conforms | recheck (mid confidence) (recheck with full path) |
| AC-23 | 背景那一輪停用時看圖結果不變：缺的天由現場一天一天補，日線與啟用時相同 | KCANDLE_SUMMARY_INTERVAL_MINUTES=0 不排 job；序列讀取自行補 | `cmd/server/dependencies.go:1031-1034` | `internal/domain/service/tests/k_candle_summarized_series_test.go:217-264` | asserts-oracle | produces-oracle | asserts-oracle 0.85 | produces-oracle 0.76 | ✅ conforms | recheck (mid confidence) |
| AC-24 | 不在任何名單上的標的不在背後補：以太幣沒有新增任何留存彙總 | 背景輪只走 FindWatched 兩份名單 | `internal/domain/service/k_candle_summary_service.go:51-98` | `internal/domain/service/tests/k_candle_summary_service_test.go:59-78` | asserts-oracle | produces-oracle | no-test 0.51 | produces-oracle 0.95 | 🟡 partial | escalate (disagree) |
| BR-1 | 留存彙總：走完的每一小時與每一天的彙總K線，現貨與合約各一份 | 一天合出 1h 與 1d 留存彙總 | `internal/domain/models/domains/k_candle_summary_day_domain.go:24-52` | `internal/domain/models/domains/tests/k_candle_summary_day_domain_test.go:44-69` | asserts-oracle | produces-oracle | asserts-oracle 0.81 | produces-oracle 0.77 | ✅ conforms | recheck (mid confidence) |
| BR-2 | 走完：只替今天零點以前的日子留存；今天一律由一分鐘K線當場合併 | summarizedEnd 不晚於 todayStart | `internal/domain/models/domains/k_candle_summary_plan_domain.go:22-48` | `internal/domain/models/domains/tests/k_candle_summary_plan_domain_test.go:39-70` | asserts-oracle | produces-oracle | asserts-oracle 0.88 | produces-oracle 0.93 | ✅ conforms | accept |
| BR-3 | 一字不差：留存彙總以現行合併規則合成，再合成四小時亦同 | 留存彙總經 KCandleSeriesDomain/KCandleContractSeriesDomain 合成，每欄原樣 | `internal/domain/models/domains/k_candle_summary_day_domain.go:24-52` | `internal/domain/models/domains/tests/k_candle_summary_day_domain_test.go:168-199` | asserts-oracle | produces-oracle | asserts-oracle 0.51 | produces-oracle 0.63 | ✅ conforms | escalate (low confidence) |
| BR-4 | 作廢：任何一分鐘變動作廢所屬小時與天，與那次變動同時成立 | 寫入與作廢同一交易；寫入失敗則作廢也不成立 | `internal/infrastructure/persistence/k_candle_repository.go:35-150` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:102-146` | asserts-oracle | produces-oracle | asserts-oracle 0.95 | produces-oracle 0.72 | ✅ conforms | recheck (mid confidence) |
| BR-5 | 合併中被改動不留存 | revision 條件式 claim | `internal/infrastructure/persistence/k_candle_summary_day_repository.go:80-127` | `internal/infrastructure/persistence/tests/k_candle_summary_day_repository_test.go:181-202` | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.63 | ✅ conforms | recheck (mid confidence) |
| BR-6 | 一天一天補：同一時間只處理一天份的一分鐘K線 | 每次讀取上限 1440 | `internal/domain/service/k_candle_contract_summarized_series_reader.go:58-100` | `internal/domain/service/tests/k_candle_summarized_series_test.go:284-329` | asserts-oracle | produces-oracle | asserts-oracle 0.81 | produces-oracle 0.51 | ✅ conforms | escalate (low confidence) |
| BR-7 | 沒有分鐘就沒有格子：無分鐘的小時或天不產生留存彙總，也不出現在序列 | 空的一天 Summaries 為空 | `internal/domain/models/domains/k_candle_summary_day_domain.go:24-52` | `internal/domain/models/domains/tests/k_candle_summary_day_domain_test.go:70-76` | asserts-oracle | produces-oracle | asserts-oracle 0.47 | produces-oracle 0.23 | ✅ conforms | escalate (low confidence) |
| BR-8 | 背景那一輪：值班那一處、每五分鐘、零或負值停用、補最近四十八小時走完的日子 | IsLeader 守門；預設 5 分；0 停用；窗口 48h | `internal/job/k_candle_summary_job.go:16-31` | `internal/job/tests/k_candle_summary_job_test.go:53-75` | asserts-oracle | produces-oracle | asserts-oracle 0.69 | produces-oracle 0.48 | ✅ conforms | escalate (low confidence) |
| NFR-1 | 已有留存彙總時，以一天為刻度要九百天在兩秒內回來 | 讀九百筆日留存彙總，無逐分鐘讀取 | `internal/domain/service/k_candle_contract_summarized_series_reader.go:28-100` | `—` | no-test | unclear | no-test 1.00 | produces-oracle 0.45 | 🟡 partial | escalate (disagree) (recheck with full path) |
| NFR-2 | 任何一次要求同一時間處理的一分鐘K線不超過一天份（1440根） | 頭尾段與每個缺日各自讀取上限 ≤1440 | `internal/domain/models/domains/k_candle_summary_plan_domain.go:22-113` | `internal/domain/service/tests/k_candle_summarized_series_test.go:284-329` | asserts-oracle | produces-oracle | shallow 0.33 | produces-oracle 0.48 | 🟠 mis-asserted | escalate (disagree) |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `internal/infrastructure/persistence/k_candle_summary_day_repository.go:25-78` | 讀留存彙總用唯讀 Repeatable Read 快照 | undocumented（ARCH 已載明，PRD 只要求「不會讀到變了卻還沒作廢」；屬實作手段，非新行為） |
| `internal/domain/models/domains/k_candle_summary_day_domain.go:24-52` | 沒有分鐘的一天仍記一筆「已彙總（空）」狀態 | undocumented（避免休市日每次重算；不產生任何彙總 K 線，與 AC-10 的「沒有留存彙總」以彙總 K 線解讀一致） |

Out-of-scope check: 指標計算、回測、策略機器人、交易日誌的讀法未被改動 — no violation.

## Analysis of non-conforming clauses

- **AC-19 / NFR-2 — real spec defect.** A request holds the partial first bucket's minutes and today's minutes at once — up to about two days (≈2 880 minutes) — while missing days are filled strictly one at a time. Memory stays flat and bounded (the user's actual requirement: limit the per-request cost), but the PRD's "at most one day (1 440) at the same time" is stricter than the design ever was. Resolution: correct the PRD's number to the bound the design guarantees, and pin each stretch's capacity in a test.
- **AC-10 — interpretation.** Jev read "也沒有留存彙總" as "no stored record"; the code stores a "summarized, empty" day state but no summary candle. Test strengthened to the week scenario.
- **AC-1 — shallow test.** Only three days; strengthened to the nine-hundred-day scenario comparing every figure. The two-second bound is a production measure (NFR-1), not unit-testable.
- **AC-7 — no test** for a four-hour bucket with one hour missing the premium index. Add it.
- **AC-21 — shallow**: the hourly summaries of the filled day were not asserted. Strengthen.
- **AC-24 — implicit only**: add an explicit "symbol on no list is never summarized" assertion.
- **NFR-1 — partial**: two seconds is a production latency target; verified after deployment, not in unit tests.

## Summary

- Conforms: 26/34 ✅ (76%)
- Violations: AC-10 (interpretation), AC-19 (spec defect, with NFR-2)
- Mis-asserted: AC-1, AC-21, NFR-2
- Partial: AC-7, AC-24, NFR-1
- Gaps: none
- Unclear: none
- Orphans: 2 (undocumented, not out-of-scope)
- Jev: ran on 34 clauses (12 re-audited with complete code paths)
- Needs James: AC-1 (low confidence), AC-3 (low confidence), AC-6 (low confidence), AC-10 (violation), AC-12 (disagree), AC-13 (low confidence), AC-14 (low confidence), AC-15 (low confidence), AC-18 (low confidence), AC-19 (violation), AC-20 (low confidence), AC-21 (disagree), AC-24 (disagree), BR-3 (low confidence), BR-6 (low confidence), BR-7 (low confidence), BR-8 (low confidence), NFR-1 (disagree), NFR-2 (disagree) — mostly Jev's low confidence over multi-file paths; the two violations are analysed above
