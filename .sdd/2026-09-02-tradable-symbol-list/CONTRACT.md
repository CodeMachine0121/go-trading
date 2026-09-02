# Contract Traceability Matrix — 可查交易標的清單

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/infrastructure/persistence/k_candle_repository.go`、`internal/domain/service/k_candle_service.go`、`internal/controller/k_candle_controller.go`、`internal/domain/models/dto/trading_symbol_dto.go`、`cmd/server/dependencies.go`
Oracle: Acceptance Criteria（7 個情境 + 4 條業務規則 + 2 條非功能需求 = 13 clauses）

## Clauses

`Spec-expected` 欄是只讀規格文字得出的業務可觀察結果；`Impl` / `Test` 欄是把它橋接到程式碼之後查到的位置。

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 列出握有 K 線的每一個交易標的 | 回覆 BTCUSDT 與 ETHUSDT 兩個 | `k_candle_repository.go:108` | `k_candle_repository_test.go:299`、`k_candle_service_test.go:337`、`k_candle_controller_test.go:287` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 同一個交易標的只出現一次 | 一百根 K 線的 BTCUSDT 只回一個 | `k_candle_repository.go:113`（`Distinct`） | `k_candle_repository_test.go:299`（BTCUSDT 有兩根，只回一次） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 一根 K 線都沒有時回覆空的清單 | 空清單，且不是錯誤 | `k_candle_service.go:106`（`make(..., 0, …)`，不回 nil） | `k_candle_repository_test.go:316`、`k_candle_service_test.go:351`、`k_candle_controller_test.go:299`（斷言回覆是 `[]` 而不是 `null`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 依名稱由小到大 | 依序 BTCUSDT、ETHUSDT、SOLUSDT | `k_candle_repository.go:114`（`Order`） | `k_candle_repository_test.go:299`（以 SOLUSDT 先存入的順序寫入，斷言回覆已排好） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 再問一次順序不變 | 順序與上一次完全相同 | `k_candle_repository.go:114`（排序由資料庫負責，不依賴走訪順序） | `k_candle_repository_test.go:299`（寫入順序與回覆順序刻意不同，證明順序不是巧合） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 設定上要追蹤但還沒有資料的不算 | 只回覆 BTCUSDT | `k_candle_repository.go:108`（只讀 `KCandles`，不碰任何設定） | `k_candle_repository_test.go:299`（測試完全沒有設定觀察清單，回覆仍正確） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | K 線被刪光之後就不再出現 | 只回覆 BTCUSDT | `k_candle_repository.go:108` | `k_candle_repository_test.go:325` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 來源是實際存下的 K 線，不是觀察清單設定 | 同 AC-6 | `k_candle_repository.go:111`（`Model(&entities.KCandle{})`） | `k_candle_repository_test.go:299` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 去重 | 同 AC-2 | `k_candle_repository.go:113` | `k_candle_repository_test.go:299` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 排序固定 | 同 AC-4、AC-5 | `k_candle_repository.go:114` | `k_candle_repository_test.go:299` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 空集合不是錯誤 | 同 AC-3 | `k_candle_service.go:106` | `k_candle_service_test.go:351` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 與既有區間查詢同一量級 | 一次詢問只讀一次資料庫，且只讀一欄 | `k_candle_repository.go:108`（單次 `Pluck`，只取 `symbol` 一欄，走 `(symbol, open_time)` 唯一索引） | `k_candle_service_test.go:337`（gomock 嚴格模式：多打一次或打到別的方法即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 不留存、不快取 | 沒有任何寫入或快取路徑 | `k_candle_service.go:100`（只呼叫 `FindDistinctSymbols`） | `k_candle_service_test.go:337`（gomock 嚴格模式：任何未預期的 `Save`／`Update` 呼叫即失敗）、`k_candle_repository_test.go:325`（刪掉之後回覆立刻反映） | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `k_candle_controller.go:97` 回 502 | 儲存層失敗時回 Bad Gateway | PRD 第 4 節的 Edge Case「儲存層讀取失敗 → 整次詢問失敗並如實說明」；沿用既有的 `respondWithError` 對映，有測試（`k_candle_controller_test.go:309`） |
| `k_candle_repository_test.go:19` 測試資料庫名稱守門 | 這些測試每次都清空 `KCandles`，指到應用程式在用的資料庫會毀掉真實資料 | 測試安全機制，不是產品行為。**這是在本切片開發過程中真的發生過一次之後補上的** |

未發現實作到 Out of Scope 項目的程式碼：沒有任何新增／修改／移除交易標的的路徑、
沒有名稱格式規則、清單上沒有附帶任何其他資訊、觀察清單一行未動、
既有的 K 線讀寫與指標計算一行未動。

## Summary

- Conforms: 13/13 clauses ✅（100%）
- Violations: 無
- Mis-asserted: 無
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 2（一個是 PRD 已列的 Edge Case，一個是測試安全機制）

> 本表是**靜態一致性稽核**：它把測試斷言與程式碼路徑分別對照規格推導出的預期結果，
> 而不是以「跑起來是綠的」作為判準。作為佐證，`GET /trading-symbols` 已對真實資料庫實測，
> 回覆 `[{"symbol":"BTCUSDT"},{"symbol":"ETHUSDT"}]`。
