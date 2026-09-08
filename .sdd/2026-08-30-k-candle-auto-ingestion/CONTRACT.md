# Contract Traceability Matrix — K 線自動抓取

Contract: `.sdd/2026-08-30-k-candle-auto-ingestion/PRD.md`
Design map: `.sdd/2026-08-30-k-candle-auto-ingestion/ARCH.md`
Implementation: `internal/domain/models/domains/`, `internal/domain/service/`, `internal/application/`, `internal/infrastructure/marketdata/`, `internal/job/`, `internal/config/`, `cmd/server/`
Oracle: Acceptance Criteria (40 clauses — 25 scenarios, 12 business rules, 3 non-functional; Compatibility is `N/A` and skipped)

**Ceiling:** static conformance audit. It reads the test code and the production code
against the oracle derived from the spec alone. It does not write new probes and does
not execute scenarios it invents. Where a single mapped test was run for corroboration
it is noted; no verdict rests on suite pass/fail.

## Clauses

The `Spec-expected` column holds the business-observable oracle derived in Phase 2,
before any code was opened.

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 一輪抓取替清單上每個交易標的取回最新已收完的 K 線 | 清單兩個標的時，兩者起始時間 09:00 的 K 線都已存入 | `k_candle_ingestion_service.go:48`、`k_candle_ingestion_domain.go:60` | `k_candle_ingestion_service_test.go:198` + `:135` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 涵蓋的五分鐘尚未走完的那一根不存入 | 09:09 時已存入的最晚一根為 09:00；09:05 不存在於系統中 | `k_candle_ingestion_domain.go:51`、`:90` | `k_candle_ingestion_service_test.go:141`、`k_candle_ingestion_domain_test.go:144` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 五分鐘走完之後該根才被存入 | 09:11 時 09:05 的 K 線已存入 | `k_candle_ingestion_domain.go:51` | `k_candle_ingestion_service_test.go:150`、`k_candle_ingestion_domain_test.go:47` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 觀察清單為空時一輪抓取不做任何事 | 沒有任何 K 線被存入，且該輪不視為錯誤 | `k_candle_ingestion_service.go:102` | `k_candle_ingestion_service_test.go:214` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 交易標的尚無任何資料時存入最近數根 | 根數為 5 時，系統內有 5 根該標的的 K 線 | `k_candle_ingestion_domain.go:60`、`k_candle_ingestion_service.go:148` | `k_candle_ingestion_service_test.go:156` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 行情來源修正過的數字覆蓋既有的那一根 | 09:00 的收盤價變為 120，且該標的仍為 5 根、未變成 10 根 | `k_candle_repository.go:32`（`OnConflict` 更新價量欄位）、`k_candle_ingestion_service.go:159` | `k_candle_repository_test.go:64` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 取回的資料部分已有、部分尚無 | 3 根覆蓋、2 根新增、沒有任何起始時間出現兩根 | `k_candle_ingestion_service.go:148`（逐根 `Save`）、`k_candle_repository.go:32` | `k_candle_repository_test.go:327` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 補齊停機期間的缺口 | 起始時間 07:05 到 09:00 之間每個五分鐘刻度上的 K 線都已存入 | `k_candle_ingestion_service.go:65`、`k_candle_ingestion_domain.go:72` | `k_candle_ingestion_service_test.go:361`、`k_candle_ingestion_application_test.go:85` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 缺口超出回補上限時只補上限之內的 | 只有落在昨日 09:07 之後的 K 線被補上；更早的缺口維持空白 | `k_candle_ingestion_domain.go:72` | `k_candle_ingestion_service_test.go:366`、`k_candle_ingestion_domain_test.go:109` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 沒有缺口時不做任何回補 | 沒有任何 K 線被新增或覆蓋；系統直接進入定時抓取 | `k_candle_fetch_window_vo.go:27`、`k_candle_ingestion_service.go:135` | `k_candle_ingestion_service_test.go:392`、`k_candle_ingestion_domain_test.go:124` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 從未有過任何資料的交易標的補滿回補上限 | 最近 24 小時內每個五分鐘刻度上的 K 線都已存入 | `k_candle_ingestion_service.go:81`、`k_candle_ingestion_domain.go:72` | `k_candle_ingestion_service_test.go:371`、`k_candle_ingestion_domain_test.go:114` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 回補完成之後才開始定時抓取 | 回補全部完成前不跑任何一輪；完成後才按五分鐘間隔運行 | `k_candle_ingestion_job.go:62` | `k_candle_ingestion_job_test.go:89`、`:98` | asserts-oracle | produces-oracle | ✅ conforms（間隔為「五分鐘」這一半見 BR-02） |
| AC-13 | 一個交易標的取不到資料，其他照常存入 | 正常的標的已存入；失敗的標的零存入；留下一筆指出該標的的失敗紀錄 | `k_candle_ingestion_service.go:139`、`k_candle_ingestion_job.go:97` | `k_candle_ingestion_service_test.go:223`、`k_candle_ingestion_job_records_test.go:103` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 全部交易標的都取不到資料時系統仍繼續運行 | 每個標的各留下一筆失敗紀錄；系統不停止，五分鐘後仍會跑下一輪 | `k_candle_ingestion_service.go:102`、`k_candle_ingestion_job.go:70` | `k_candle_ingestion_service_test.go:243`、`k_candle_ingestion_job_records_test.go:153` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 上一輪失敗漏掉的 K 線在下一輪自動補回 | 下一輪之後，上一輪漏掉的 09:00 K 線已存入，且無人工介入 | `k_candle_ingestion_domain.go:60`（窗口寬度＝單輪取回根數） | `k_candle_ingestion_service_test.go:485` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 同一批全部合規時全部存入 | 該批 K 線全部存入系統 | `k_candle_ingestion_service.go:148` | `k_candle_ingestion_service_test.go:299` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 最高價低於最低價的那一根被跳過 | 其餘存入；08:55 那根不存在；留下紀錄指出該標的、該根與「最高價不得低於最低價」 | `k_candle_ingestion_service.go:149`、`k_candle_ingestion_job.go:102` | `k_candle_ingestion_service_test.go:263`、`k_candle_ingestion_job_records_test.go:116` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 起始時間不在五分鐘刻度上的那一根被跳過 | 其餘存入；08:53 那根不存在；留下紀錄指出該根違反「起始時間必須落在五分鐘刻度上」 | `k_candle_ingestion_service.go:149`、`k_candle_domain.go:44` | `k_candle_ingestion_service_test.go:275` | asserts-oracle | produces-oracle | ✅ conforms（措辭差異見備註 N-1） |
| AC-19 | 同一批全部違規時一根都不存，但不算整輪失敗 | 零存入；每根各留一筆指出是哪根、哪條規則的紀錄；不視為取不到資料的失敗 | `k_candle_ingestion_service.go:148`、`:130` | `k_candle_ingestion_service_test.go:287` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 每一輪固定只抓啟動時定下的交易標的 | 該輪只針對清單上的那些標的取回 K 線 | `k_candle_ingestion_job.go:38`、`k_candle_ingestion_service.go:102` | `k_candle_ingestion_service_test.go:198`（`Times(2)`）、`k_candle_ingestion_job_test.go:108` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 執行期間變更觀察清單不生效 | 該輪仍只針對原本的標的；沒有新標的的 K 線被存入 | `k_candle_ingestion_job.go:36`（建構時複製）、`application_config.go:65`（啟動讀一次） | `k_candle_ingestion_job_test.go:108` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 觀察清單為空等同關閉自動抓取 | 不會有任何 K 線被自動存入；K 線的查詢/新增/修改/刪除與指標計算全部照常可用 | `k_candle_ingestion_service.go:102`；既有路由未被更動 | `k_candle_ingestion_service_test.go:214`；既有 controller 與 service 測試 | asserts-oracle | produces-oracle | ✅ conforms（後半見備註 N-2） |
| AC-23 | 自動抓取被整個停用 | 不執行啟動回補、不跑任何一輪；其他 K 線功能全部照常可用 | `dependencies.go:64` | `dependencies_test.go:10` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 來源給出所要求時間範圍以外的 K 線時不予採用 | 沒有任何 K 線被存入；系統不因這個回答繼續向來源追問 | `binance_market_data_proxy.go:111`、`:52` | `binance_market_data_proxy_test.go:252` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 單輪取回根數被設成不合理的值 | 該次執行整個不進行；不取資料、不存入；留下說明根數必須大於零的紀錄 | `k_candle_ingestion_service.go:48`、`:65`、`k_candle_ingestion_domain.go:31` | `k_candle_ingestion_service_test.go:448`、`k_candle_ingestion_job_records_test.go:130` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-01 | 觀察清單規則：啟動時讀取並定案，執行期間不變；為空時形同關閉；數量不設上限 | 清單來自啟動時的設定、之後不變；空清單不抓；沒有數量檢查 | `application_config.go:65`、`k_candle_ingestion_job.go:38` | `ingestion_config_test.go:22`、`k_candle_ingestion_job_test.go:108`、`k_candle_ingestion_service_test.go:214` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 抓取間隔規則：固定五分鐘一輪，不可調整；停用走總開關或空清單 | 間隔恆為五分鐘，且沒有任何設定能改它 | `k_candle_ingestion_job.go:16`（常數）、`dependencies.go:82`；`application_config.go` 無對應設定 | `k_candle_ingestion_job_test.go:142` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 收完才存規則：只存入五分鐘已完整走完的 K 線 | 進行中那一根一律不存入 | `k_candle_ingestion_domain.go:51`、`:90` | `k_candle_ingestion_domain_test.go:30`、`:144` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-04 | 單輪取回根數規則：每輪取回最近五根（可調整） | 預設窗口涵蓋五根；設定可改 | `k_candle_ingestion_domain.go:60`、`application_config.go:66` | `k_candle_ingestion_domain_test.go:70`、`ingestion_config_test.go:83` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 覆蓋規則：同起始時間以新數字整根覆蓋，不產生第二根 | 覆蓋而非新增，總根數不變 | `k_candle_repository.go:32` | `k_candle_repository_test.go:64` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 回補規則：啟動時補齊缺口，最多往回 24 小時（可調整）；回補完成後定時抓取才開始 | 窗口起點取「最後一根的下一根」與「當下減回補上限」之中較晚者；回補先於定時 | `k_candle_ingestion_domain.go:72`、`k_candle_ingestion_job.go:62`、`application_config.go:67` | `k_candle_ingestion_domain_test.go:96`、`k_candle_ingestion_job_test.go:89`、`ingestion_config_test.go:83` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 並行規則：同一輪內所有交易標的同時進行，彼此獨立 | 多個標的同時開始，一個的結果不影響另一個 | `k_candle_ingestion_service.go:109`（`sync.WaitGroup`，每標的一個 goroutine） | `k_candle_ingestion_service_test.go:534`（來源扣住答案直到兩個標的都到齊）、`:223` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 失敗獨立規則：該輪不存入並留下失敗紀錄；不重試、不中止，交由下一輪 | 失敗的標的零存入且有紀錄；程式中沒有重試；系統不中止 | `k_candle_ingestion_service.go:139`、`k_candle_ingestion_job.go:70` | `k_candle_ingestion_service_test.go:223`、`:243` | asserts-oracle | produces-oracle | ✅ conforms（「不中止」的斷言缺口同 AC-14） |
| BR-09 | 逐根跳過規則：未通過既有 K 線規則者跳過該根並留下紀錄，同批其餘照常存入 | 逐根判斷；違規者跳過，其餘存入；同批全違規不算整輪失敗 | `k_candle_ingestion_service.go:148`-`:166` | `k_candle_ingestion_service_test.go:255` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 紀錄規則：紀錄必須足以辨識哪個交易標的、哪一根 K 線、違反哪一條規則或取不到資料 | 紀錄同時帶出標的、起始時間與原因 | `k_candle_ingestion_job.go:85`-`:106` | `k_candle_ingestion_job_records_test.go:103`、`:116`、`:130` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 窗口信任規則：只採用落在所要求時間範圍之內的 K 線；範圍以外的不予採用，且不因此繼續追問 | 範圍外的 K 線被丟掉，且索取就此停止 | `binance_market_data_proxy.go:111`、`:52` | `binance_market_data_proxy_test.go:252` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 設定合理性規則：單輪取回根數必須大於零；不合理時該次執行整個不進行並留下紀錄，不替呼叫端猜值 | 兩個用例都拒絕執行並回報原因 | `k_candle_ingestion_domain.go:31` | `k_candle_ingestion_service_test.go:448` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-01 | Performance：一輪抓取必須在五分鐘之內完成；一輪耗時取決於最慢的標的而非全部相加 | 一輪的牆鐘時間 < 五分鐘 | `k_candle_ingestion_service.go:102`（併發）、`binance_market_data_proxy.go:38`（單次請求逾時） | — | no-test | unclear | ❔ unclear |
| NFR-02 | Security：沒有身分驗證與權限控管；不對外開放任何操作進入點，觀察清單無法從外部改變 | 本功能沒有新增任何路由；清單只能由啟動設定決定 | `dependencies.go`（`registerRoutes` 未更動，本次僅新增 `backgroundJobsFor`）、`k_candle_ingestion_job.go:38` | `k_candle_ingestion_job_test.go:108`、`routes_test.go:15` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-04 | Analytics：不埋任何使用行為追蹤；僅在失敗與跳過時保留紀錄 | 正常輪次不留任何紀錄；只有失敗與跳過會寫 | `k_candle_ingestion_job.go:85`-`:106` | `k_candle_ingestion_job_records_test.go:103`、`:116`、`:130`、`:176`（含「無事則不寫」） | asserts-oracle | produces-oracle | ✅ conforms |

### 備註（第二輪：補測試之後）

第一輪稽核判為 🟠／🟡 的六條已補上測試，每一條都經過反向驗證（把行為弄壞、確認該測試轉紅、還原）。
唯一仍未關閉的是 `BR-07`（同時性）與 `NFR-01`（效能），理由見下方摘要。

### 備註

- **N-1（AC-18）** PRD 引用的規則文字是「起始時間必須落在**五**分鐘刻度上」，實際訊息由
  `k_candle_domain.go:44` 以數字產生，輸出為「起始時間必須落在**5**分鐘刻度上」。
  語意相同、辨識得出是哪一條規則，因此判為 conforms；但字面與 PRD 不一致，
  且這段訊息屬於既有的 K 線管理切片，**本次未更動**。要不要統一是文件層面的取捨。
- **N-2（AC-22）** 「其他功能照常可用」屬於不干擾主張。判定依據是變更範圍——
  `internal/controller/` 零變更、`registerRoutes` 零變更（本次對
  `cmd/server/dependencies.go` 只有新增 34 行、無刪除），加上既有測試套件仍全綠。
  沒有任何一個測試「在空清單情境下」重跑既有功能。
- **N-3（AC-01）** 存入時「這根 K 線屬於哪個交易標的」在服務層從未被斷言——
  測試的替身只記錄起始時間。該環節改由 `market_k_candle_vo` 與 proxy 的測試分段守住
  （`k_candle_ingestion_domain_test.go:204`、`binance_market_data_proxy_test.go:44`）。

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `internal/job/k_candle_ingestion_job.go:55` | `Stop()`：關閉工作、結束定時迴圈。**正式程式碼沒有任何地方呼叫它**——行程目前就是直接結束，沒有關閉流程；實際使用它的只有測試（讓背景 goroutine 在測試替身收攤前停下） | deferred — 刻意不立條文。為一個不會發生的行為寫驗收條件會讓契約說謊；要讓它成為可依賴的行為，應另開處理整個服務關閉流程的切片（訊號攔截、HTTP 伺服器收尾、背景工作停止三者一起） |

> 第一輪稽核列出的另外兩個 orphan——「丟棄窗口外的 K 線」與「根數不合法時拒絕執行」——
> 已補進 PRD，成為 `AC-24`／`BR-11` 與 `AC-25`／`BR-12`，不再是 orphan。

**Out of Scope 檢查：無違規。** PRD 列出的九項界外事項——執行期間動態增減觀察清單、
觀察清單管理功能、對外查看抓取狀況、失敗主動通知、補救回補上限以前的歷史缺口、
批次匯入、交易標的名稱格式驗證、觀察清單數量上限、衍生分析——
在程式碼中**都找不到對應實作**。沒有 scope creep。

## Summary

- Conforms: 39/40 clauses ✅ (98%)
- Violations: 無 🔴 —— 沒有任何一條的程式碼產出與 spec 相反
- Mis-asserted: 無 🟠
- Partial: 無 🟡
- Gaps: 無 ❌
- Unclear: `NFR-01` ❔
- Orphans: 1 ⚠️（`Stop()`，刻意延後立約，非界外違規）

### 仍未關閉的一條

- **`NFR-01` 效能 ❔** —— 「一輪必須在五分鐘內完成」取決於觀察清單長度與來源的實際回應時間，
  靜態判不了，也不該用單元測試假裝。建議實際跑起來之後量。

### 第三輪：orphan 收斂

- `BR-07` 並行的**同時性**已補上測試：行情來源扣住答案直到兩個交易標的都抵達，
  逐一執行的實作永遠等不到第二個，會在測試自己的期限上失敗而非拖垮整包。
  反向驗證過（把併發改成逐一 → 轉紅）。
- 「丟棄窗口外的 K 線」與「根數不合法時拒絕執行」兩個 orphan 已補進 PRD 成為條文。
- `Stop()` **刻意留在 orphan**，理由見上表。

### 執行環境備註

`internal/infrastructure/persistence/tests/` 在沒有 `TEST_POSTGRES_DSN` 時會整包跳過。
本次稽核與補測試期間，以一個丟棄式 PostgreSQL 容器實際跑過該套件（含新增的 `AC-07` 測試），
全數通過，容器已移除。**日常 `make test` 若未設該變數，`AC-06`／`BR-05`／`AC-07`
這三條的測試不會執行。**
