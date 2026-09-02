# Contract Traceability Matrix — 彙總 K 線序列

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/{vo,domains,dto}`、`internal/domain/service/k_candle_service.go`、`internal/controller/k_candle_controller.go`、`cmd/server/dependencies.go`
Oracle: Acceptance Criteria（19 個情境 + 6 條業務規則 + 4 條非功能需求 = 29 clauses）

## Clauses

`Spec-expected` 欄是只讀規格文字得出的業務可觀察結果；`Impl` / `Test` 欄是把它橋接到程式碼之後查到的位置。

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 同一個刻度區間內的 K 線合併成一根 | 一根：起始 10:00、開盤 100、最高 140、最低 90、收盤 110、成交量 10 | `k_candle_bucket_domain.go:30` | `k_candle_bucket_domain_test.go:33` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 刻度區間內只有一根時，價量原樣呈現 | 一根：起始 10:00，開高低收 100/110/90/105 | `k_candle_bucket_domain.go:30` | `k_candle_bucket_domain_test.go:68` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 跨刻度區間的 K 線分屬不同的彙總 K 線 | 兩根：起始 10:00 與 11:00 | `k_candle_series_domain.go:33` | `k_candle_series_domain_test.go:41`（第一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 五分鐘刻度等同不合併 | 三根，起始時間與價量逐根與原本相同 | `aggregation_interval_domain.go:87` + `k_candle_bucket_domain.go:30` | `k_candle_series_domain_test.go:41`（第二列）、`k_candle_service_test.go:248` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 序列依起始時間由早到晚 | 依序為起始 10:00、11:00、12:00 | `k_candle_series_domain.go:46` | `k_candle_series_domain_test.go:90` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 查詢區間的起訖不必對齊刻度區間 | 一根，起始 10:00，只採計 10:35 與 10:40 | `aggregation_interval_domain.go:88` | `aggregation_interval_domain_test.go:70`（10:35→10:00 一列）、`k_candle_series_domain_test.go:106` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 落在不同刻度區間的資料不會被併在一起（查 09:50–10:30） | 兩根：起始 09:00 只含 09:55、起始 10:00 只含 10:00 | `k_candle_series_domain.go:38` | `k_candle_series_domain_test.go:41`（跨小時邊界一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7b | 查詢區間切掉的那幾根不算（查 09:58–10:30） | 只回一根：起始 10:00，只含 10:00 那根 | `k_candle_repository.go:91`（`open_time >= startTime`）+ `k_candle_series_domain.go:33` | `k_candle_series_storage_test.go`（走真實資料庫：兩根存進去，一次查 09:50 起回兩根、一次查 09:58 起只回一根） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 一天的刻度自世界標準時間的零點切分 | 兩根，起始時間為那兩日的零點 | `aggregation_interval_domain.go:88` | `aggregation_interval_domain_test.go:70`（1d 兩列）、`k_candle_series_domain_test.go:41`（跨午夜一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 中間整段沒有資料時不產出那一根 | 兩根：起始 10:00 與 12:00，沒有 11:00 那根 | `k_candle_series_domain.go:33`（只走讀到的 K 線） | `k_candle_series_domain_test.go:41`（空桶一列）、`k_candle_service_test.go:216` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 整段區間都沒有資料時回覆空的序列 | 空序列，且不是錯誤 | `k_candle_series_domain.go:33` | `k_candle_series_domain_test.go:119`、`k_candle_service_test.go:232`、`k_candle_controller_test.go:193` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 恰好等於上限的區間正常回覆 | 正常回覆序列（1000 個刻度區間） | `k_candle_series_query_domain.go:42` | `k_candle_series_query_domain_test.go:14`（第一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 超過上限的區間整次被拒絕 | 拒絕，訊息含「區間過大」與兩條出路，且不回任何 K 線 | `k_candle_series_query_domain.go:42` | `k_candle_series_query_domain_test.go:57`、`k_candle_service_test.go:268`、`k_candle_controller_test.go:231` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 同一段區間改用更長的刻度就落回上限之內 | 正常回覆序列 | `aggregation_interval_domain.go:93` | `k_candle_service_test.go:282`、`k_candle_series_query_domain_test.go:14`（1d 一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 未指定彙總刻度時視為五分鐘 | 依五分鐘回覆，回覆自報刻度為五分鐘 | `aggregation_interval_domain.go:54` | `aggregation_interval_domain_test.go:22`（空字串一列）、`k_candle_series_domain_test.go:127`、`k_candle_controller_test.go:207` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 五種以外的彙總刻度一律拒絕 | 拒絕，訊息點名那五種 | `aggregation_interval_domain.go:52` | `aggregation_interval_domain_test.go:50`、`k_candle_series_query_domain_test.go:104`、`k_candle_controller_test.go:220` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 未指定交易標的 | 拒絕，訊息「必須指定交易標的」 | `k_candle_series_query_domain.go:28`（委由 `k_candle_query_domain.go:20`） | `k_candle_series_query_domain_test.go:69`、`k_candle_service_test.go:310`、`k_candle_controller_test.go:242` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 結束時間早於開始時間 | 拒絕，訊息「結束時間不得早於開始時間」 | `k_candle_series_query_domain.go:28`（委由 `k_candle_query_domain.go:26`） | `k_candle_series_query_domain_test.go:69`（第二列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 起訖同一個時點是合法的 | 一根，起始 10:00 | `aggregation_interval_domain.go:93`（同桶 → 1） | `k_candle_series_query_domain_test.go:14`（同起訖一列）、`aggregation_interval_domain_test.go:123`（第一列） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 合併規則：開盤取最早、收盤取最晚、最高取最高、最低取最低、四項成交數字各自加總 | 依序等於 100 / 110 / 140 / 90，四項成交數字為兩根之和 | `k_candle_bucket_domain.go:38-58` | `k_candle_bucket_domain_test.go:33`、`:54` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 對齊規則：刻度區間邊界自世界標準時間當日零點依刻度長度切分 | 5m/15m/1h/4h/1d 各自落在整齊的格子上，4h 的 02:03 落在 00:00 | `aggregation_interval_domain.go:88` | `aggregation_interval_domain_test.go:70`（八列涵蓋五種刻度） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 不補洞：沒有 K 線的刻度區間不產出 | 序列中沒有那一根，也沒有補值的一根 | `k_candle_series_domain.go:33` | `k_candle_series_domain_test.go:41`（空桶一列） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 上限規則：刻度區間數超過上限即整次拒絕，訊息給出兩條出路 | 拒絕，訊息同時提到縮小區間與更長的彙總刻度 | `k_candle_series_query_domain.go:42` | `k_candle_series_query_domain_test.go:57` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 預設刻度：未指明時視為五分鐘 | 回覆自報刻度為五分鐘 | `aggregation_interval_domain.go:54` | `aggregation_interval_domain_test.go:22`、`k_candle_series_domain_test.go:127` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 排序：序列一律依起始時間由早到晚 | 起始時間遞增 | `k_candle_series_domain.go:46` | `k_candle_series_domain_test.go:90` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 一次彙總查詢是即時讀取，與既有區間查詢同一量級 | 一次查詢只讀一次資料，讀取根數上界由刻度與桶數決定 | `k_candle_service.go:79`（單次 `FindInRange`，limit = `SourceCandleLimit()`） | `k_candle_service_test.go:192`（斷言讀取上限為 3×12）、`k_candle_application_test.go:130`（2×12） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 不留存：彙總結果不寫入、不快取 | 沒有任何寫入路徑被觸發 | `k_candle_service.go:79`（只呼叫 `FindInRange`） | `k_candle_service_test.go:192`（gomock 嚴格模式：任何未預期的 `Save`／`Update` 呼叫即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 精確度：合併過程不得失真 | 加總與比大小以精確數字進行 | `k_candle_bucket_domain.go:38-55`（全程 `decimal.Decimal`，無浮點數） | `k_candle_bucket_domain_test.go:33`（以 `decimal.Equal` 斷言） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 時間基準：一律以世界標準時間表示與切分 | 桶起點為世界標準時間 | `aggregation_interval_domain.go:88`（先 `.UTC()` 再切） | `aggregation_interval_domain_test.go:70` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `k_candle_controller.go:166` `readTime` | 把五個 handler 重複的 RFC3339 解析收成一處；行為與先前逐處解析完全相同 | 內部重構，非新行為——不對應任何 clause 也不違反 Out of Scope |
| `dto/k_candle_series_query_dto.go:18` `ToQueryDto` | 彙總查詢輸入交出底下那組一般區間查詢輸入 | 形狀轉換，非新行為 |

未發現實作到 Out of Scope 項目的程式碼：無彙總結果的寫入或快取路徑、無補洞邏輯、
既有的查詢／新增／修改／刪除與指標計算取數路徑一行未動（`git diff` 僅觸及本切片列出的檔案）。

## Summary

- Conforms: 29/29 clauses ✅（100%）
- Violations: 無
- Mis-asserted: 無
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 2（皆為重構產物，非未記載的行為）

### Code review 之後的修正

初版的 AC-7 寫的是「查 09:58 到 10:30 會回兩根」，但那與 PRD 第 4 節自己的 Edge Case
（「只採計落在查詢區間內的 K 線」）互相矛盾——09:55 落在查詢區間之外，本來就不該被採計。
更嚴重的是本表當時引用的證據是 `k_candle_series_domain_test.go` 的一列，
它直接把兩根 K 線餵進 `KCandleSeriesDomain`，**完全繞過範圍過濾**，
結構上不可能分辨規格與實際行為的差異，卻被評為 `asserts-oracle`。

已把情境改成與 Edge Case 一致的兩條（AC-7 換成真的涵蓋兩根的區間，AC-7b 明寫被切掉的不算），
並改引用會走過範圍過濾的 service 層測試。

> 本表是**靜態一致性稽核**：它把測試斷言與程式碼路徑分別對照規格推導出的預期結果，
> 而不是以「跑起來是綠的」作為判準。作為佐證，本切片新增／變更的每一個函式
> 在 `go test ./... -coverpkg=./internal/...` 下均為 100% 覆蓋。
