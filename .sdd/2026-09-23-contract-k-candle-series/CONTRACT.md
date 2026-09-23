# 契約追溯矩陣 — 合約 K 線的彙總序列（contract-k-candle-series）

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/domain/models/domains/k_candle_contract_series_domain.go`、`internal/domain/models/domains/k_candle_series_query_domain.go`、`internal/domain/service/k_candle_contract_service.go`、`internal/application/k_candle_contract_application.go`、`internal/controller/k_candle_contract_controller.go`、`cmd/server/dependencies.go`
Oracle: Acceptance Criteria（13 條：AC 11 條 + BR 2 條；PRD 無 Section 6 NFR）
Out of Scope（負面清單）：合約即時跟盤、合約指標計算、現貨彙總的改變

> 靜態契約符合度稽核：依 PRD 預期結果判斷測試斷言與程式路徑，不執行自創情境；僅執行各條已對應的既有測試作佐證（皆為綠燈），判定不以綠燈為據。

## Clauses

`Spec-expected` 欄為 Phase 2 僅依 PRD 文字推得的業務可觀察預期（oracle）；稽核欄是把它經 UL-MAP／ARCH 橋接到具體產物後的判定。

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 同一格的價量合併：Given 09:00 到 09:04 五根合約 K 線,開盤最早那根是 100,其中一根最高 120 / When 以五分鐘刻度查 09:00 到 09:05 的合約 K 線序列 / Then 回一根起始時間 09:00 的合約 K 線,開 100、收為 09:04 那根的收、高 120 | 只回一根、起始 09:00；開＝最早那根的開（100）；收＝09:04 那根的收；高＝五根中最高者 | k_candle_contract_series_domain.go:34-59（分格）、:68-82（開最早/收最晚/高最高） | k_candle_contract_series_domain_test.go:52 `TestKCandleContractSeriesDomainMergesABucketIntoOneCandle` | asserts-oracle（亂序輸入、斷言單根、09:00、開 100、收 104＝09:04 那根、高＝最高那根的高；fixture 的最高值是 121 而非 120，但釘住的是同一條規則） | produces-oracle | ✅ conforms |
| AC-02 | 成交筆數加總：Given 那五根成交筆數各 7 / When 以五分鐘刻度查 / Then 那一根成交筆數 35 | 合併後那根成交筆數＝35 | k_candle_contract_series_domain.go:87 | k_candle_contract_series_domain_test.go:76；service test k_candle_contract_service_test.go:225（7+8＝15）；contract_controllers_test.go:620（`tradeCount":14`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 三條價格線各自合併：Given 那五根裡標記價格最高的是 125,溢價指數最低的是 -0.0009 / When 以五分鐘刻度查 / Then 那一根標記價格最高 125、溢價指數最低 -0.0009 | 標記價格高＝125；溢價指數低＝-0.0009 | k_candle_contract_series_domain.go:88-92、:119-136 | k_candle_contract_series_domain_test.go:58-59、:78、:86 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 一格裡有舊資料,那條線沒有值：Given 那五根裡有一根沒有指數價格 / When 以五分鐘刻度查 / Then 那一根的指數價格顯示為沒有值 / And 標記價格照常合併 | 該格指數價格（開高低收）皆無值；標記價格仍有合併後的值 | k_candle_contract_series_domain.go:119-124（任一根缺即 broken）、:139-146（輸出無值）、:74-75、:88-89（標記照常） | k_candle_contract_series_domain_test.go:91 `...LeavesALineOutOfABucketThatLacksItAnywhere`（首根/中間/末根三例） | asserts-oracle（指數四值皆斷言 invalid；「標記照常合併」只斷言 MarkOpen＝100，偏弱但足以抓到標記線被一併清空） | produces-oracle | ✅ conforms |
| AC-05 | 沒有資料的一格不產出：Given 09:05 到 09:09 沒有合約 K 線 / When 以五分鐘刻度查 09:00 到 09:10 / Then 只回 09:00 那一根 | 沒有合約 K 線的格不出現在序列中（不補洞） | k_candle_contract_series_domain.go:35-52（只對有資料的格產出） | k_candle_contract_series_domain_test.go:129 `...ProducesNothingForAnEmptyBucket`（09:05 格空，只回 09:00 與 09:10） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 只讀合約的：Given 同一時間現貨也有 K 線 / When 查合約 K 線序列 / Then 回來的每一根都是合約 K 線,帶著標記價格 | 即使同時間現貨有 K 線，序列只含合約 K 線，每根帶標記價格；現貨那根不被讀入 | k_candle_contract_service.go:59-60（只讀合約 repository）；k_candle_contract_repository.go:178-197（查合約表）；dependencies.go:180、:245 | contract_controllers_test.go:609（mock 合約 repository，斷言 `markClose`） | **shallow**：沒有任何測試建立「同時間現貨也有 K 線」的 Given。controller／service 測試用 mock repository，若 `FindInRange` 誤讀現貨表仍會通過；既有 `TestKCandleContractRepositoryKeepsContractAndSpotCandlesApart`（repository_test.go:55）只驗 `FindOne`，未驗序列所走的 `FindInRange` | produces-oracle | 🟠 mis-asserted |
| AC-07 | 由可顯示根數挑刻度：Given 查一整天,可顯示 100 根 / When 查合約 K 線序列 / Then 回十五分鐘刻度的序列,並說出用的是十五分鐘 | 選中十五分鐘刻度，回應明說用的是十五分鐘 | k_candle_series_query_domain.go:111-121；k_candle_contract_series_domain.go:56 | k_candle_contract_series_domain_test.go:149；contract_controllers_test.go:624（`"interval":"15m"`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 兩種說法同時給：Given 同時給了刻度與可顯示根數 / When 查合約 K 線序列 / Then 查詢被拒絕,並說明「彙總刻度與可顯示根數只能挑一種說法」 | 拒絕，且說明「彙總刻度與可顯示根數只能挑一種說法」 | k_candle_series_query_domain.go:96-99；k_candle_contract_service.go:53-57；k_candle_contract_controller.go:238-241（400） | k_candle_contract_service_test.go:250（子案「兩種說法同時給」）；contract_controllers_test.go:643、:653-654 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 認不得的刻度：Given 刻度是 2m / When 查合約 K 線序列 / Then 查詢被拒絕 | 拒絕 | k_candle_series_query_domain.go:101-106；aggregation_interval_domain.go:83 | k_candle_contract_service_test.go:250（子案「認不得的刻度」，斷言合約驗證哨兵＋訊息） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 區間過大：Given 以一分鐘刻度查一個月 / When 查合約 K 線序列 / Then 查詢被拒絕,並說明時間區間過大 | 拒絕，且說明時間區間過大 | k_candle_series_query_domain.go:59-65 | k_candle_contract_service_test.go:250（子案「區間過大」，30 天 × 1m） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 沒指定合約標的：Given 沒指定合約標的 / When 查合約 K 線序列 / Then 查詢被拒絕,並說明「必須指定交易標的」 | 拒絕，且說明「必須指定交易標的」 | k_candle_series_query_domain.go:48-51（沿用區間查詢驗證）；k_candle_contract_service.go:53-57 | k_candle_contract_service_test.go:250（子案「沒指定合約標的」）；contract_controllers_test.go:645、:655-656 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-01 | 刻度挑選、刻度區間切分、單次上限與現貨共用同一套規則；永續合約全天候，每一格都算交易時段。 | 合約序列的刻度挑選／切分／上限與現貨一致；計格時一天每一格都算（不套任何休市時段） | k_candle_contract_service.go:39（取 crypto 市場）、:51-52（重用 `NewKCandleSeriesQueryDomain`）；application_config.go:545（crypto 規則為零值＝永不休市） | k_candle_contract_series_domain_test.go:149；contract_controllers_test.go:624；k_candle_contract_service_test.go:225、:250 | **shallow**（僅「全天候」半句）：測試 fixture 的 `MarketCatalogDomain` 只登錄 crypto（service_test.go:79、controller_test.go:86），而 `MarketOf` 對未登錄名稱 fallback 到 crypto——service 若誤取台股市場，所有測試仍會得到全天候結果而通過；沒有測試釘住「service 取的是全天候市場」。共用規則本身（兩種說法、上限、刻度拒絕）有被斷言 | produces-oracle | 🟠 mis-asserted |
| BR-02 | 價格線合併：開取最早、收取最晚、高取最高、低取最低；一格裡任何一根缺那條線，那一格那條線就沒有值。 | 每條價格線：開＝最早、收＝最晚、高＝最高、低＝最低；格內任一根缺該線 → 該格該線無值 | k_candle_contract_series_domain.go:62-100、:119-146 | k_candle_contract_series_domain_test.go:52（價量/標記/指數/溢價四組開高低收全斷言）、:91（缺線三例） | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans（無對應條款的程式行為）

| Code | Description | Verdict |
|------|-------------|---------|
| k_candle_contract_controller.go:100-111 | `displayableCandleCount` 非整數 → 400「displayableCandleCount 必須是整數」 | undocumented（PRD 未提；與現貨端點一致的輸入解析） |
| k_candle_contract_controller.go:88-98 | `startTime`／`endTime` 無法解析 → 400 | undocumented（沿用既有合約查詢行為） |
| k_candle_contract_service.go:61-63；k_candle_contract_controller.go:249 | 儲存讀取失敗原樣上拋 → 502，且不被標成驗證錯誤 | undocumented（基礎設施錯誤路徑，有測試：service_test.go:281、controller_test.go:661） |

非行為備註（不計入 orphan，僅供對齊）：
- k_candle_series_query_domain.go:151-170：`ContractSeriesOf` 被插在 `SourceCandleLimit` 的 doc comment 與其函式之間，導致 `SourceCandleLimit` 失去說明、`ContractSeriesOf` 的 doc 前面黏著一段不相干的敘述。
- k_candle_contract_service.go:17-19：struct 註解仍寫「It knows nothing about a market catalogue」，與本切片注入 `MarketCatalogDomain`（:23-26、:33、:39）矛盾，屬過時註解。
- Out of Scope 檢查：本切片 commit（d218800）未改動現貨彙總（`SeriesOf`／`KCandleSeriesDomain` 未動），未觸及合約即時跟盤或指標計算——無越界。

## Summary

- Conforms: 11/13 clauses ✅（84.6%）
- Violations: 無
- Mis-asserted: AC-06、BR-01（綠燈測試未釘住 oracle）
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 3

---

## 第二輪（修正後）

| ID | 第一輪 | 第二輪 | 怎麼解決的 |
|----|--------|--------|------------|
| AC-06 | 🟠 | ✅ | `TestKCandleContractRepositoryReadsARangeOfContractCandlesOnly`：同一時間已有現貨 K 線時，讀一段只拿回合約的那一根（收盤 101，不是現貨的 100） |
| BR-01 | 🟠 | ✅ | 測試用的市場目錄裡放了一個會休市的市場；`TestKCandleContractServiceMergesAWeekendStretchBecauseContractsNeverClose` 讀星期日的三個小時，每一分鐘都讀得到，兩根都併進序列。把服務改成去查會休市的市場後，這支測試就會失敗（已驗證）。 |

其他：`SourceCandleLimit` 的說明註解移回它自己身上；服務的結構說明改成說它從市場目錄拿了什麼、為什麼拿。

孤兒（非數字的可顯示根數、讀不懂的起訖時間 → 請求有誤；儲存失敗 → 上游失敗）都是這個 API 既有的邊界回應，跟現貨序列一樣，不列為違規。

**第二輪結果：13 條條款全部 conforms。**
