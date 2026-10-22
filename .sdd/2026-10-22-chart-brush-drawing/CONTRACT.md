# Contract Traceability Matrix — 2026-10-22-chart-brush-drawing

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/domain/models/domains/`, `internal/application/assistantqueries/`, `internal/config/`
Oracle: Acceptance Criteria (21 AC + 2 BR + 1 NFR = 24 clauses)

## Clauses

`Test audit` / `Code audit` are Claude's shadow verdict (recorded before Jev ran). `Jev test` / `Jev code` are Jev's pick with confidence.
`Status` and `Route` come from `jev_audit.py` (final run per clause). Paths are abbreviated: `BT` = `internal/domain/models/domains/tests/chart_drawing_board_domain_test.go`,
`BAT` = `…/domains/tests/assistant_chart_drawing_batch_domain_test.go`, `AQT` = `internal/application/assistantqueries/tests/chart_drawing_assistant_queries_test.go`,
`AT` = `internal/application/tests/chart_drawing_application_test.go`, `CT` = `internal/controller/tests/chart_drawing_controller_test.go`.

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Jev test | Jev code | Status | Route |
|----|--------|------------------------|------|------|------------|------------|----------|----------|--------|-------|
| AC-1 | US-01 一般長度的奇異筆（50 個錨點） | 存回成功，讀出照原順序 50 個錨點 | chart_drawing_kind_domain.go:32,60 | BT:439（an ordinary brush）、AT:248 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.57 | ✅ conforms | escalate (low confidence) |
| AC-2 | US-01 最短的奇異筆（2 個錨點） | 存回成功 | chart_drawing_kind_domain.go:60 | BT:439（the shortest brush） | asserts-oracle | produces-oracle | asserts-oracle 0.45 | produces-oracle 0.84 | ✅ conforms | escalate (low confidence) |
| AC-3 | US-01 最長的奇異筆（200 個錨點） | 存回成功 | chart_drawing_kind_domain.go:60 | BT:439（the longest brush） | asserts-oracle | produces-oracle | asserts-oracle 0.64 | produces-oracle 0.79 | ✅ conforms | recheck |
| AC-4 | US-01 只有一個錨點的奇異筆 | 整份拒絕「第 1 個繪圖：奇異筆要 2 到 200 個錨點，收到 1 個」，繪圖板不動 | chart_drawing_kind_domain.go:70；chart_drawing_board_domain.go:75 | BT:412、CT:208（400） | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.95 | ✅ conforms | accept |
| AC-5 | US-01 超過 200 個錨點的奇異筆 | 整份拒絕「…收到 201 個」 | chart_drawing_kind_domain.go:70 | BT:412 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.87 | ✅ conforms | accept |
| AC-6 | US-01 其他種類的錨點數不變 | 1 個錨點的水平線存回成功 | chart_drawing_kind_domain.go:24 | BT:134 | asserts-oracle | produces-oracle | asserts-oracle 0.55 | produces-oracle 0.92 | ✅ conforms | escalate (low confidence) |
| AC-7 | US-02 錨點不在任何 K 線的起始時間 | 存回成功，讀出時間仍是 10:17:42.350 | chart_drawing_domain.go:33-42 | BT:471、AT:248；儲存往返 persistence tests（brush stroke keeps sub-second moments） | asserts-oracle | produces-oracle | asserts-oracle 0.91 | produces-oracle 0.52 | ✅ conforms | escalate (low confidence) |
| AC-8 | US-02 錨點沒有時間 | 整份拒絕，說每個錨點都要有起始時間 | chart_drawing_domain.go:34 | BT:412 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.95 | ✅ conforms | accept |
| AC-9 | US-02 錨點價格為零 | 整份拒絕，說錨點價格必須大於零 | chart_drawing_domain.go:37 | BT:412 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.99 | ✅ conforms | accept |
| AC-10 | US-03 沒給樣式用預設 | 橙色、線寬 2、實線、不鎖定、顯示 | chart_drawing_kind_domain.go:32；chart_drawing_style_domain.go:39 | BT:482、AT:248 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.97 | ✅ conforms | accept |
| AC-11 | US-03 改過的樣式照存 | 紅色、線寬 4、虛線 | chart_drawing_style_domain.go:47-72 | BT:482 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.75 | ✅ conforms | recheck |
| AC-12 | US-03 線寬超出範圍 | 整份拒絕，說線寬只能是 1 到 4 | chart_drawing_style_domain.go:57 | BT:412 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.40 | ✅ conforms | escalate (low confidence) |
| AC-13 | US-04 剛好到上限 | 20 筆各 200 錨點存回成功（共 4000） | chart_drawing_board_domain.go:89 | BT:528 | asserts-oracle | produces-oracle | asserts-oracle 0.77 | produces-oracle 0.75 | ✅ conforms | recheck |
| AC-14 | US-04 超過上限 | 整份拒絕「一份繪圖板的繪圖最多共 4000 個錨點，收到 4001 個」，繪圖板不動 | chart_drawing_board_domain.go:89-92 | BT:535、AT:266 | asserts-oracle | produces-oracle | asserts-oracle 0.87 | produces-oracle 0.95 | ✅ conforms | accept |
| AC-15 | US-04 助手新增剛好填滿 | 3998 + 2 → 成功，剛好 4000 | chart_drawing_board_domain.go:109-118 | BT:582 | asserts-oracle | produces-oracle | asserts-oracle 0.95 | produces-oracle 0.85 | ✅ conforms | accept |
| AC-16 | US-04 助手新增會超過上限 | 整批拒絕「…這份只剩 1 個錨點的空間，這一批有 2 個」，繪圖板不動 | chart_drawing_board_domain.go:109-118 | BT:591 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.92 | ✅ conforms | accept |
| AC-17 | US-04 原本的繪圖數上限照舊 | 201 個繪圖整份拒絕，說最多 200 個繪圖 | chart_drawing_board_domain.go:50 | BT:549 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.87 | ✅ conforms | accept |
| AC-18 | US-05 助手送了奇異筆 | 整批拒絕「第 2 個繪圖：奇異筆只能由使用者手畫，助手不能畫」，一個都沒畫 | assistant_chart_drawing_batch_domain.go:54；chart_drawing_kind_domain.go:74 | BAT:86、AQT:138（無任何讀寫被呼叫） | asserts-oracle | produces-oracle | asserts-oracle 0.51 | produces-oracle 0.95 | ✅ conforms | escalate (low confidence) |
| AC-19 | US-05 助手讀到奇異筆 | 讀到種類奇異筆、30 個錨點、來源手畫 | chart_drawing_list_assistant_query.go:39-59 → chart_drawing_service.go:39 → chart_drawing_board_domain.go:185 | AQT:185 | asserts-oracle | produces-oracle | asserts-oracle 0.68 | unclear 0.68 | ❔ unclear | escalate (disagree) |
| AC-20 | US-05 助手移除不了奇異筆 | 整次拒絕，說手畫的助手不能移除 | chart_drawing_board_domain.go:150 | BT:461 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.99 | ✅ conforms | accept |
| AC-21 | US-05 助手畫其他種類照舊 | 新增趨勢線成功 | chart_drawing_kind_domain.go:22；chart_drawing_board_domain.go:99 | BT:582 | asserts-oracle | produces-oracle | asserts-oracle 0.84 | produces-oracle 0.87 | ✅ conforms | recheck |
| BR-1 | 繪圖種類九選一：原有八種點數不變；奇異筆要 2 到 200 個錨點 | 九種；八種固定數；奇異筆 2–200 | chart_drawing_kind_domain.go:22-33 | BT:117（不認得的種類列出九種）、BT:412-460 | asserts-oracle | produces-oracle | asserts-oracle 0.88 | produces-oracle 0.97 | ✅ conforms | accept |
| BR-2 | 助手新增：奇異筆一律拒絕；先看能不能畫，讀過 K 線後再看放不放得下 | 在讀行情前拒絕；助手能力不提供奇異筆 | assistant_chart_drawing_batch_domain.go:54；chart_drawing_create_assistant_query.go:34,44 | AQT:138-168 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.80 | ✅ conforms | recheck |
| NFR-1 | 4000 錨點的繪圖板整份存回遠低於單次送出大小上限 | 不需調整上限 | config 既有 `REQUEST_BODY_LIMIT_KILOBYTES` 1024 | — | no-test | produces-oracle | not run | not run | 🟡 partial | — |

### Recheck / escalation notes

- **AC-19**: Jev could not see the read path through `loadBoard`. Re-read: `ChartDrawingService.loadBoard` (chart_drawing_service.go:172-182) wraps the stored row unchanged in `ChartDrawingBoardDomain`, and `ToDto` clones the drawings — nothing filters by kind. The mapped test was run on its own and passed. The status stays as the script set it.
- **Low-confidence escalations (AC-1, 2, 6, 7, 12, 18)**: Claude and Jev agree on both audits. They were escalated only for confidence. Each one is also pinned by a mutation that was killed during implement (anchor range ±1, hand-only flag, default colour/width, the bounds checks).
- **NFR-1**: worked out by hand rather than tested. A 20 × 200-anchor board with millisecond times and 8-decimal prices serializes to about 279 KB, against the 1024 KB request ceiling.
- **AC-13 / AC-15** were `shallow` in the first run: they asserted success without the 4000 total. Commit `test(chart-drawing): pin that a board at the anchor ceiling holds exactly 4000 anchors` fixed them, and the re-run is recorded above.

## Orphans (code with no clause)

| Code | Description | Verdict |
| ---- | ----------- | ------- |
| chart_drawing_create_assistant_query.go:30 | The capability description now states the board anchor ceiling (`錨點加總最多 %d 個`) | undocumented in PRD (supporting text for US-04; harmless) |
| chart_drawing_list_assistant_query.go:30 | The list description explains that brush anchors are a stroke whose times are not candle starts | undocumented in PRD (supporting text for US-05 讀取; harmless) |

## Summary

- Conforms: 22/24 clauses ✅ (92%)
- Violations: none
- Mis-asserted: none (AC-13, AC-15 fixed during this run)
- Partial: NFR-1
- Gaps: none
- Unclear: AC-19 (Jev disagree; mapped test run separately and passed)
- Orphans: 2 (descriptive text only)
- Jev: ran on 23 clauses (NFR-1 is not a code clause)
- Needs James: AC-19 (disagree); AC-1, AC-2, AC-6, AC-7, AC-12, AC-18 (low confidence, both audits agree)

> Static conformance audit against the Acceptance Criteria: it judges test assertions and code paths against the spec's expected outcomes. It does not run the full suite or execute invented scenarios.
