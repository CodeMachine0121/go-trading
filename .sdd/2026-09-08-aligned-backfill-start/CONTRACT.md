# Contract Traceability Matrix — 回補起點對齊刻度區間對齊基準

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/domains/`（抓取與彙總刻度兩處）
Oracle: Acceptance Criteria（16 個情境）＋ Core Business Rules（5 條）＋ Non-Functional（3 條）＝ 24 clauses

> 第一輪稽核找出 3 個 mis-asserted 與 1 個 orphan。兩個補了測試、orphan 收進 PRD；> 第三個（BR-5「系統不做某件事」）留著並說明為什麼測不動。本表為處理後的狀態。

> **審核天花板**：這是一次**靜態**契約稽核。它拿 PRD 的預期結果分別去對照測試斷言與程式路徑，
> **不執行自己發明的情境**。判定一律來自與 oracle 的比對，不是來自紅綠。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01.1 | 從未存過 K 線的交易標的 | 現在 9/8 14:03 → 起點 9/7 00:00，而**不是** 9/7 14:03 | `k_candle_ingestion_domain.go:102` | `k_candle_ingestion_domain_test.go:146`（asked part way through the day）＋`:96`（never held a candle…） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.2 | 往回推之後剛好落在邊界上 | 現在 9/8 00:00 → 起點 9/7 00:00（不改變它） | 同上 | `k_candle_ingestion_domain_test.go:146`（asked exactly on the edge, which moves nothing）＋`aggregation_interval_domain_test.go:127`（a moment already on its edge is left alone） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.3 | 一天將盡時往回推最遠 | 現在 9/8 23:59 → 起點 9/7 00:00，往回將近 48 小時 | 同上 | `k_candle_ingestion_domain_test.go:146`（asked in the last minute of the day）＋`:188`（the last minute of the day is the worst case） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.4 | 一天剛開始時幾乎不多往回 | 現在 9/8 00:01 → 起點 9/7 00:00，只多一分鐘 | 同上 | `k_candle_ingestion_domain_test.go:146`（asked in the first minute of the day） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.5 | 對齊後最舊那一格裝滿一整天 | 9/7 那一格的起始時間是 9/7 00:00，開盤價是 9/7 第一分鐘的開盤價 | 起點：`k_candle_ingestion_domain.go:102`；併格：`k_candle_series_domain.go`（既有） | `k_candle_ingestion_domain_test.go`（`TestABackfillStartedAtAnEdgeProducesAWholeOldestBucket`）——拿窗口的起點餵資料進去併格，斷言那一格的起始時間與**開盤價**；拿掉對齊即紅（已確認） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.6 | 同一次對齊讓較細的刻度也完整 | 以四小時彙總時最舊那一格的起始時間也是 9/7 00:00 | `aggregation_interval_domain.go:97` | `aggregation_interval_domain_test.go`（`the oldest bucket of a coarse interval starts on that edge`——真的以四小時併格並斷言那一格的起始時間；另有 `its edge is also an edge for each of the six on offer` 驗六種邊界互通） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.1 | 缺口在回補上限之內 | 已存最新 9/8 12:30 → 起點 9/8 12:31，**不**對齊 | `k_candle_ingestion_domain.go:107`（較晚者勝出，既有） | `k_candle_ingestion_domain_test.go:234`（明白斷言沒有被拉回當天零點）＋`:96`（gap inside the lookback…） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.2 | 停機超過回補上限 | 已存最新 9/1 08:00 → 起點 9/7 00:00；中間的洞不補 | 同上 | `k_candle_ingestion_domain_test.go:96`（gap wider than the lookback…）＋`k_candle_ingestion_service_test.go:428` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.3 | 沒有缺口 | 沒有任何時間範圍要抓 | `k_candle_fetch_window_vo.go`（`IsEmpty`，既有） | `k_candle_ingestion_domain_test.go:96`（no gap at all comes back empty） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.1 | 有交易時段的市場 | 9/7 那一格起始時間 9/7 00:00，涵蓋該日全部成交，不算缺資料 | 既有：併格不補洞（`KCandleSeriesDomain`）＋交易時段裁剪在窗口之後（`MarketDomain`） | `k_candle_ingestion_service_test.go:927`（台股的手動補齊，窗口涵蓋整個前一日盤）＋`k_candle_series_domain_test.go:153`（沒有東西落入的刻度區間不產出） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.2 | 當天才上市的交易標的 | 同上 | 同上（同一條規則：一格只裝落進去的那些） | `k_candle_ingestion_domain_test.go`（`TestABucketIsWholeEvenWhenTheMarketOnlyTradedPartOfTheDay`）——當天 03:00 才第一筆，斷言那一格仍屬於一整天且成交量一根都沒少 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.3 | 對齊後的起點比來源手上最早的資料還早 | 來源回幾根就存幾根，這一輪不算失敗 | 既有：來源回空不判定失敗 | `k_candle_ingestion_service_test.go`（21 處以空回應驅動的案例都斷言這一輪成功） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.1 | 一般情況下多抓的量 | 比回補上限多出 14 小時 3 分 | `k_candle_ingestion_domain.go:102` | `k_candle_ingestion_domain_test.go:188`（part way through the day，明白斷言 14h3m） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.2 | 最多多抓的量 | 多出 23 小時 59 分；沒有任何情況多出一整天或更多 | 同上 | `k_candle_ingestion_domain_test.go:188`（worst case 斷言 23h59m，並斷言小於一整格——那個上界**由最粗刻度推導**而非寫死） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.3 | 完全不多抓的情況 | 與回補上限一模一樣（多出 0） | 同上 | `k_candle_ingestion_domain_test.go:188`（on the edge nothing extra is fetched） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.1 | 改動之前開始抓的交易標的 | 那一格維持原樣，系統不回頭補；往後新抓的都從刻度區間對齊基準開始 | **不新增任何東西**：`k_candle_ingestion_domain.go:107` 只往前看 | `k_candle_ingestion_domain_test.go:234`（起點接在已存那一根之後，不會更早） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 只有「來自回補上限」那個候選起點要對齊 | 另一個候選不被對齊 | `k_candle_ingestion_domain.go:102` vs `:107`（只有前者經過 `BucketStart`） | `k_candle_ingestion_domain_test.go:234` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 對齊一律往下，不往上 | 起點比未對齊的那一刻**更早** | `BucketStart` 即 `Truncate`（往下） | `k_candle_ingestion_domain_test.go:188`（三個案例的差值皆 ≥ 0 且等於預期的往前量） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 回補上限的意思是「至少往回這麼久」 | 實際往回的時間介於一倍與將近兩倍之間 | `k_candle_ingestion_domain.go:102` + 方法註解 | `k_candle_ingestion_domain_test.go:188`（下界 0、上界不足一整格） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 刻度區間對齊基準是六種彙總刻度共同的邊界 | 對它取格子起點，六種刻度都得同一刻 | `aggregation_interval_domain.go:97` + 清單不變量註解 | `aggregation_interval_domain_test.go:127`（its edge is also an edge for every declarable interval） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 一格只有部分時間有成交不代表缺資料 | 系統不去判斷一格是否裝滿 | **不新增任何東西**：沒有任何程式在判斷「一格是否裝滿」 | 由 AC-03.1／03.2 的行為涵蓋：只交易幾小時的一天、當天才上市的一天，都照樣產出一格且不少任何一根。**「系統不做某件事」測不動**，見下方說明 | shallow | produces-oracle | 🟠 mis-asserted |
| NFR-1 | 第一次回補多抓的量不超過一天的 K 線 | 上界為一整格減一分鐘 | `k_candle_ingestion_domain.go:102` | `k_candle_ingestion_domain_test.go:188`（上界由最粗刻度推導，改動最粗刻度時仍然成立——已以突變確認會紅） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 對齊只改變要抓哪一段，不改變 K 線本身的形狀 | 任何對外交付的形狀不變；既有資料一根都不動 | 只有窗口起點改變；沒有 entity／DTO／回應形狀變動 | 全套 17 個 package 的測試在只改窗口起點的情況下維持綠（除了 4 個明白斷言舊起點的子案例） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 窗口終點仍是最新一根已收完的 K 線 | 終點不變 | `k_candle_ingestion_domain.go:112`（未改動） | `k_candle_ingestion_domain_test.go:96`（每一個案例都斷言終點） | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| — | 第一輪那個 orphan（「市場的交易日」與「刻度區間對齊基準」不可合併）已收進 PRD 的 Edge Cases | 已解決 |

沒有任何一項落在 **Out of Scope** 上——特別是**沒有**出現「補掉停機斷點」或「回頭修既有半截格子」的程式。

## Summary

- Conforms: 23/24 clauses ✅（96%）
- Violations: 無
- Mis-asserted: `BR-5`（見下）
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 0

### 為什麼 BR-5 留著

它說的是**系統不做某件事**——不去判斷一格是否裝滿。這種主張沒有直接的測法：
能寫的只有「窮舉所有沒有這個判斷的情況」，而那永遠不完整，也不會在有人加進那個判斷時變紅。
真正守著它的是 AC-03.1 與 AC-03.2：只交易幾小時的一天、當天才上市的一天，
都照樣產出一格且一根都沒少——**任何人加進「一格必須裝滿才算」的判斷，那兩條就會紅**。
所以這一條刻意不補一個假的測試，而是記在這裡。
