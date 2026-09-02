# Contract Traceability Matrix — 交易標的登錄

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/entities/trading_symbol.go`、`internal/infrastructure/persistence/trading_symbol_repository.go`、`internal/domain/service/trading_symbol_service.go`、`internal/controller/trading_symbol_controller.go`、`cmd/migrate/main.go`
Oracle: Acceptance Criteria（9 個情境 + 5 條業務規則 + 2 條非功能需求 = 16 clauses）

> **本切片推翻了 `.sdd/2026-09-02-tradable-symbol-list/` 的 US-03**（「設定上要追蹤但還沒有資料的不算」）。
> 那一份 `CONTRACT.md` 仍記載舊規則，作為當時的紀錄保留；**以本表為準**。

## Clauses

`Spec-expected` 欄是只讀規格文字得出的業務可觀察結果；`Impl` / `Test` 欄是把它橋接到程式碼之後查到的位置。

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 全新的資料庫 | BTCUSDT 與 ETHUSDT 都被登錄，並說明這次新登錄了這兩個 | `trading_symbol_service.go:74`、`cmd/migrate/main.go:63` | `trading_symbol_service_test.go:125`、`trading_symbol_application_test.go`（回報的名單）、`trading_symbol_repository_test.go:27`（真的寫進資料庫） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 重跑一次不會重複登錄 | 兩個都不重複登錄，並說明這次沒有新登錄任何一個 | `trading_symbol_service.go:85`（先讀後比）、`trading_symbol_repository.go:48`（併發保險）、`cmd/migrate/main.go:59` | `trading_symbol_service_test.go:137`、`trading_symbol_repository_test.go:37` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 只補上缺的那一個 | 只登錄 ETHUSDT，並說明新登錄了它一個 | `trading_symbol_service.go:85` | `trading_symbol_service_test.go:149` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 已登錄但還沒有任何資料的也要出現 | 回覆 BTCUSDT 與 ETHUSDT | `trading_symbol_service.go:52` | `trading_symbol_service_test.go:47`（第一列）、`trading_symbol_controller_test.go:54` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 有資料但沒登錄過的也要出現 | 回覆 BTCUSDT 與 XRPUSDT | `trading_symbol_service.go:53-55` | `trading_symbol_service_test.go:47`（第二列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 兩邊都有的只出現一次 | 只回覆 BTCUSDT 一個 | `trading_symbol_service.go:52`（以名稱為鍵的集合） | `trading_symbol_service_test.go:47`（第三列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 依名稱由小到大 | 依序 BTCUSDT、SOLUSDT | `trading_symbol_service.go:61`（合併後排序一次） | `trading_symbol_service_test.go:47`（第四列，兩個來源刻意各給一個且順序相反） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 兩邊都空時回覆空的清單 | 空清單，且不是錯誤 | `trading_symbol_service.go:64`（`make(..., 0, …)`，不回 nil） | `trading_symbol_service_test.go:47`（第六列，斷言 `NotNil`）、`trading_symbol_controller_test.go:68`（斷言回覆是 `[]` 而不是 `null`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 已登錄的市場即使資料被刪光也還在 | 仍然回覆 ETHUSDT | `trading_symbol_service.go:52` | `trading_symbol_service_test.go:47`（第五列） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 預設交易標的是 BTCUSDT 與 ETHUSDT，寫在程式碼裡 | 登錄的就是這兩個 | `trading_symbol_service.go:16` | `trading_symbol_service_test.go:125`（逐名斷言，改動清單即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 登錄的時機是建立資料庫結構時 | 建完資料表之後登錄 | `cmd/migrate/main.go:53`（在 `Migrate()` 之後） | 實測：`make migrate` 第一次印 `registered 2 new (BTCUSDT, ETHUSDT)` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 冪等：先確認在不在，重跑結果都一樣 | 第二次跑不會多出重複的登錄 | `trading_symbol_service.go:80-89` + `trading_symbol_repository.go:48` | `trading_symbol_service_test.go:137`、`:160`、`trading_symbol_repository_test.go:37`；實測第二次印 `already registered, nothing to add` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 回報這次新登錄了哪幾個（可能是零個） | 回傳新登錄的名單 | `trading_symbol_service.go:95` | `trading_symbol_service_test.go:125`、`:137`、`:149` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 清單來源＝已登錄 ∪ 有 K 線，去重、由小到大 | 同 AC-4…AC-9 | `trading_symbol_service.go:41` | `trading_symbol_service_test.go:47`（六列） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 建立資料庫結構可以重跑任意次數 | 同 BR-3；且不因搶著登錄同一個而失敗 | `trading_symbol_repository.go:48`（`OnConflict DoNothing`） | `trading_symbol_repository_test.go:37`（同一個標的登錄兩次不報錯） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 清單查詢多讀一張表，量級不變 | 一次查詢兩次讀取，各只讀一欄／一張小表 | `trading_symbol_service.go:42`、`:47`（各一次） | `trading_symbol_service_test.go:47`（gomock 嚴格模式：多打一次即失敗） | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `trading_symbol_repository.go:43`（登錄空清單直接返回） | 沒有要登錄的東西時不打資料庫 | PRD 第 4 節 Flow 的第 3 步隱含（「把還沒登錄的挑出來，登錄它們」——一個都沒有時沒有東西可登錄）；有測試 |
| `trading_symbol_controller.go:22` 回 502、`trading_symbol_service.go:44/49/78/92` 的失敗回報 | 儲存層失敗時整次失敗並如實說明 | PRD 第 4 節的 Edge Case；有測試 |
| `k_candle_repository_test.go` 的 `closedDatabase` | 讓儲存層依指令失敗的測試用具 | 測試用具，非產品行為 |

未發現實作到 Out of Scope 項目的程式碼：沒有任何新增／改名／移除已登錄標的的對外路徑、
預設清單不可設定、觀察清單一行未動、名稱格式沒有任何規則、
既有的 K 線讀寫與指標計算一行未動。

## Summary

- Conforms: 16/16 clauses ✅（100%）
- Violations: 無
- Mis-asserted: 無
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 3（兩個是 PRD 已列的 Flow／Edge Case，一個是測試用具）

### 這次稽核連帶補上的兩件事

1. `entities.TradingSymbol.ToDto()` 一開始就寫了，但**沒有任何人呼叫**——
   清單是以名稱合併兩個來源產生的，不會把已登錄的 entity 轉成 DTO。已刪除。
2. `TradingSymbolRepository` 與 `KCandleRepository.FindDistinctSymbols` 的
   「儲存層失敗」那條分支先前沒有測到（後者是上一個切片留下的缺口，當時沒有量到 repository 這一層）。
   已補上以「連線已關閉」驅動的測試，這一段現在全數覆蓋。

> 本表是**靜態一致性稽核**：它把測試斷言與程式碼路徑分別對照規格推導出的預期結果，
> 而不是以「跑起來是綠的」作為判準。作為佐證：本切片新增／變更的每一個函式覆蓋率均為 100%，
> `make migrate` 連跑兩次的輸出已實測，`GET /trading-symbols` 在剛登錄完、一根 K 線都還沒抓的
> 狀態下回覆 `[{"symbol":"BTCUSDT"},{"symbol":"ETHUSDT"}]`。
