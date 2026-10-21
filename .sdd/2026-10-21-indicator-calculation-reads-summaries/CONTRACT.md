# Contract Traceability Matrix — 指標計算改取留存彙總

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/domain/models/domains/{indicator_source_reach_domain,indicator_calculation_domain,k_candle_summary_plan_domain,contract_k_candle_alignment_domain}.go`、`internal/domain/service/{indicator_calculation_service,contract_indicator_calculation_service,k_candle_summarized_series_reader,k_candle_contract_summarized_series_reader}.go`、`internal/infrastructure/persistence/contract_position_statistic_repository.go`
Oracle: Acceptance Criteria（17 clauses）+ NFR（1）

縮寫：P＝`persistence/tests/indicator_calculation_reads_test.go`（真資料庫上新舊讀法逐值相等）、SS／CS＝`service/tests/(contract_)indicator_calculation_service_test.go`、AD＝`domains/tests/contract_k_candle_alignment_domain_test.go`、RD＝`domains/tests/indicator_source_reach_domain_test.go`。
AC-2、AC-4、AC-7、AC-8、AC-14 於補強測試／更正引用範圍後重新交 Jev 稽核，下表為重新稽核結果；首次稽核的分歧見 Notes。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Jev test | Jev code | Status | Route |
|----|--------|------------------------|------|------|------------|------------|----------|----------|--------|-------|
| AC-1 | 走完的日子取留存彙總，算出的值與逐分鐘合併相同 | 九十五個值、起始時間、兩種根數與逐分鐘相同；一分鐘 K 線最多今天份 | k_candle_contract_summarized_series_reader.go:117 | P:279、CS:911 | asserts-oracle | produces-oracle | asserts-oracle 0.89 | produces-oracle 0.65 | ✅ conforms | recheck |
| AC-2 | 還沒彙總好的日子一天一天補好後照用 | 缺的三天被補成留存彙總，值與逐分鐘相同 | k_candle_summarized_series_reader.go:60 | SS:981 | asserts-oracle | produces-oracle | asserts-oracle 0.47 | produces-oracle 0.43 | ✅ conforms | escalate (low confidence) |
| AC-3 | 截止時間落在格子中間，那一格不用 | 最後一格是九點，十點那格不採用 | indicator_calculation_domain.go:121、indicator_source_reach_domain.go:45 | SS:537、CS:556 | asserts-oracle | produces-oracle | asserts-oracle 0.60 | produces-oracle 0.85 | ✅ conforms | recheck |
| AC-4 | 截止時間剛好落在格子邊界，邊界之前那一格採用 | 最後一格是昨天 | k_candle_summary_plan_domain.go:35 | CS:911、P:294 | asserts-oracle | produces-oracle | asserts-oracle 0.79 | produces-oracle 0.71 | ✅ conforms | recheck |
| AC-5 | 細刻度維持逐分鐘合併 | 十五分鐘指標值不變，且不取留存彙總 | indicator_source_reach_domain.go:24、k_candle_summarized_series_reader.go:123 | P:303、RD:31 | asserts-oracle | produces-oracle | asserts-oracle 0.69 | produces-oracle 0.97 | ✅ conforms | recheck |
| AC-6 | 中間停牌十天，跳過空檔繼續往前湊滿 | 採用最近有行情的日子，與逐分鐘相同 | indicator_source_reach_domain.go:102 | P:296、CS:890、SS:881 | asserts-oracle | produces-oracle | asserts-oracle 0.64 | produces-oracle 0.55 | ✅ conforms | escalate (low confidence) |
| AC-7 | 台股只算有行情的交易日 | 最近有行情的交易日，與逐分鐘相同，週末不成格 | k_candle_summarized_series_reader.go:120 | P:384 | asserts-oracle | produces-oracle | asserts-oracle 0.96 | produces-oracle 0.41 | ✅ conforms | escalate (low confidence) |
| AC-8 | 行情歷史不夠時以可用根數計算 | 回報計算根數為要的格數、實際採用根數為有的格數 | k_candle_summarized_series_reader.go:146 | SS:917 | asserts-oracle | produces-oracle | asserts-oracle 0.91 | produces-oracle 0.28 | ✅ conforms | escalate (low confidence) |
| AC-9 | 可用根數低於最少可算根數照舊拒絕 | 拒絕，說出可用與至少要的根數 | indicator_calculation_domain.go（usableBucketCount） | CS:576、P:297 | asserts-oracle | produces-oracle | asserts-oracle 0.29 | produces-oracle 0.31 | ✅ conforms | escalate (low confidence) |
| AC-10 | 行情歷史最早那一天不完整照舊採用 | 最早那格只含下午行情並採用 | k_candle_summarized_series_reader.go:120 | P:295、P:410 | asserts-oracle | produces-oracle | asserts-oracle 0.52 | produces-oracle 0.39 | ✅ conforms | escalate (low confidence) |
| AC-11 | 每五分鐘都有統計，每一格只取收盤前最後一筆 | 每格採收盤前最後一筆，且只取那一筆 | contract_k_candle_alignment_domain.go:152、contract_indicator_calculation_service.go:98 | CS:522、AD:11 | asserts-oracle | produces-oracle | asserts-oracle 0.67 | produces-oracle 0.43 | ✅ conforms | escalate (low confidence) |
| AC-12 | 統計中途斷了，採用格子內最後一筆 | 採用格內最後一筆 | contract_k_candle_alignment_domain.go:152 | AD:11、P:301 | asserts-oracle | produces-oracle | asserts-oracle 0.73 | produces-oracle 0.91 | ✅ conforms | recheck |
| AC-13 | 整格沒有統計，持倉數字是零 | 持倉數字全部為零 | contract_k_candle_alignment_domain.go:152 | AD:11、CS:455 | asserts-oracle | produces-oracle | asserts-oracle 0.79 | produces-oracle 0.81 | ✅ conforms | recheck |
| AC-14 | 一分鐘刻度可採用前四分鐘內的那一筆 | 一分鐘格採用收盤前五分鐘內那一筆 | contract_k_candle_alignment_domain.go:185 | AD:33、CS:443 | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.44 | ✅ conforms | escalate (low confidence) |
| AC-15 | 剛好落在收盤那一刻的統計屬於下一格 | 這格不採用，下一格才可能採用 | contract_k_candle_alignment_domain.go:166 | AD:21、CS:461 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.55 | ✅ conforms | escalate (low confidence) |
| AC-16 | 同時取得，結果與依序取得相同 | 每格資金費率、是否結算、持倉數字與依序相同 | contract_indicator_calculation_service.go:76-117 | P:279 | asserts-oracle | produces-oracle | asserts-oracle 0.81 | produces-oracle 0.61 | ✅ conforms | recheck |
| AC-17 | 任何一種取不到，整次計算失敗 | 整次失敗、不回傳任何指標值 | contract_indicator_calculation_service.go:111 | CS:662-760 | asserts-oracle | produces-oracle | asserts-oracle 0.85 | produces-oracle 0.73 | ✅ conforms | recheck |
| NFR-1 | 讀取耗時至少降到改前的五分之一；記錄改前改後筆數與耗時 | 有量測紀錄，改後 ≤ 改前 1/5 | contract_indicator_calculation_service.go:52 | P:446（Benchmark） | shallow | produces-oracle | shallow 0.77 | not-implemented 0.64 | ❌ gap | escalate (disagree) |

## Notes

- **首次稽核分歧（已處理）**：AC-2 當時只靠「補了再取」的相等比對、沒驗「被補成留存彙總」→ 新增 `TestCalculateIndicatorSummarizesTheMissingDaysItReads`；AC-4、AC-8、AC-14 首次給的引用範圍沒含真正斷言那一段，更正後 Jev 判 asserts-oracle；AC-7 補上「週末不成格」的斷言。
- **低信心的 code 判定**：Jev 只看得到引用的片段，看不到 `SelectInputCandles`／`usableBucketCount` 等既有規則，故 code 信心偏低。行為等價另有兩道保證：真資料庫上 19 個情境新舊讀法逐值相等（兩輪：邊補邊取、取已補好的），以及 25 個突變全部被測試抓到。
- **NFR-1**：效能以 benchmark 量測並記錄於 ARCH.md（約 1,250 ms → 17 ms、164,004 → 26,049 列），不做成會因機器快慢而時好時壞的自動斷言；這是刻意的取捨，非實作缺漏。

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| k_candle_summary_plan_domain.go:112（DaysFromFirstMinute） | 看圖與指標共用：缺彙總的日子早於該段第一根一分鐘 K 線者不合併（那些天本來就合不出東西） | undocumented（效能；結果不變，看圖亦受益） |
| k_candle_summary_plan_domain.go:35、83 | 區間終點落在格子最後一刻時那格也取留存彙總；尾端空段不查詢 | undocumented（ARCH 已載；看圖結果不變） |

無任何落在 Out of Scope（回測讀取）的變更。

## Summary

- Conforms: 17/18 clauses ✅（94%）
- Violations: —
- Mis-asserted: —
- Partial: —
- Gaps: NFR-1（量測以 benchmark 記錄、刻意不設自動門檻）
- Unclear: —
- Orphans: 2（效能性、結果不變）
- Jev: ran on 18 clauses（5 條重新稽核）
- Needs James: AC-2、AC-6、AC-7、AC-8、AC-9、AC-10、AC-11、AC-14、AC-15（low confidence，理由見 Notes）、NFR-1（disagree）
