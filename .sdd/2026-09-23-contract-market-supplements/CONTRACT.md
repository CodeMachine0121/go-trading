# Contract Traceability Matrix — 合約行情補充資料（contract-market-supplements）

Contract: PRD.md（v1.0 Draft）
Design map: ARCH.md（Confirmed）
Implementation: branch `feat/contract-market-context`（`git diff main...HEAD`）
Oracle: Acceptance Criteria（75 clauses：AC 65 · BR 7 · NFR 3）

> 靜態符合度稽核：以 PRD 的驗收條件為 oracle，分別判斷「測試是否斷言 oracle」與「正式程式碼是否產生 oracle」，
> 不以整套測試綠燈為依據、也不執行自創情境。只對個別已對應的測試做過佐證執行（domain service 與 PostgreSQL repository 數支，皆綠）。

路徑縮寫（皆在 `internal/` 之下）：
`D/`＝`domain/models/domains/`、`S/`＝`domain/service/`、`A/`＝`application/`、`C/`＝`controller/`、`MD/`＝`infrastructure/marketdata/`、`P/`＝`infrastructure/persistence/`、`J/`＝`job/`、`E/`＝`domain/models/entities/`；
測試：`Dt/`＝`D/tests/`、`St/`＝`S/tests/`、`At/`＝`A/tests/`、`Ct/`＝`C/tests/`、`MDt/`＝`MD/tests/`、`Pt/`＝`P/tests/`、`Jt/`＝`J/tests/`。

### Out of Scope（負面檢核清單）

| # | 項目 | 檢核結果 |
|---|------|----------|
| OOS-1 | 合約重演、策略、機器人使用這些資料 | 未發現（重演／策略／機器人未引用新 repository） |
| OOS-2 | 維持保證金完整分級；只記最小一級 | 只讀 `maintMarginPercent` 一個值（`MD/binance_contract_symbol_lookup_proxy.go:325`），未做分級 |
| OOS-3 | 交易規格變更歷史 | 未發現（整份覆寫，`P/contract_trading_symbol_repository.go:121`） |
| OOS-4 | 資金費率結算／持倉統計的手動新增、修改、刪除、指名一段歷史同步 | 未發現（兩者只有 GET 路由，`cmd/server/dependencies.go:263-268`） |
| OOS-5 | 主動買賣量比、大戶多空人數比、預估下一次資金費率、訂單簿、強制平倉單 | 未發現 |
| OOS-6 | 現貨的任何東西 | 未發現現貨路徑變動 |

## Clauses

`Spec-expected` 欄為 Phase 2 僅依規格寫下的業務可觀察 oracle；稽核欄則是透過 UL-MAP / ARCH 把它橋接成具體產物後的判斷。

### US-01 — 合約 K 線多帶指數價格與溢價指數

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 四份資料都齊,存成一根完整的合約 K 線 | 存入一根合約 K 線，且帶著指數價格、溢價指數各自的開高低收 | `MD/binance_contract_market_data_proxy.go:104-145`；`D/k_candle_contract_domain.go:111-131,165-172` | `MDt/binance_contract_market_data_proxy_test.go:402`；`Dt/k_candle_contract_domain_test.go:223`；`St/contract_k_candle_ingestion_service_test.go:120` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 溢價指數獨缺那一根,那一根不存 | 那一分鐘不存、留下指明缺溢價指數的紀錄；同批其他齊全的照存 | `MD/binance_contract_market_data_proxy.go:140`；`D/contract_price_line_domain.go:30-35`；`S/contract_k_candle_ingestion_service.go:475-488` | `MDt/…market_data_proxy_test.go:430`；`St/contract_k_candle_ingestion_service_test.go:165` | asserts-oracle（紀錄文字為「溢價指數必填」，業務上已點名缺溢價指數） | produces-oracle | ✅ conforms |
| AC-03 | 指數價格整份問不到,該標的這一輪一根都不存 | 該標的本輪一根都不存、留下失敗紀錄；下一輪照常再抓 | `MD/…market_data_proxy.go:110-114`；`S/contract_k_candle_ingestion_service.go:415-421` | `MDt/…market_data_proxy_test.go:451`；`St/…ingestion_service_test.go:188` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 指數價格那一份附帶的零量一律丟棄 | 存下那一根的成交量是價量那份的數字，不是指數那份的零 | `MD/binance_contract_wire.go:24-38,75-93` | `MDt/…market_data_proxy_test.go:402`（成交量）、`:153`（同一轉換的其餘量） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 還沒走完的那一分鐘四份都不存 | 最新未走完那一分鐘沒有任何合約 K 線被存 | `S/contract_k_candle_ingestion_service.go:475` | `St/…ingestion_service_test.go:202` | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 — 指數價格與溢價指數的合法性

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-06 | 負的溢價指數合法 | 被存入，溢價指數四值 -0.0005/-0.0003/-0.0007/-0.0005 原樣保存 | `D/k_candle_contract_domain.go:125-130` | `Dt/k_candle_contract_domain_test.go:223`（合約比現貨便宜） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 溢價指數全是零合法 | 被存入 | 同上 | `Dt/k_candle_contract_domain_test.go:223`（四個都是零） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 溢價指數高低於低 | 新增被拒，說明「溢價指數最高不得低於最低」 | `D/contract_price_line_domain.go:37-40` | `Dt/k_candle_contract_domain_test.go:259` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 指數價格為負 | 新增被拒，說明「指數價格不得為負」 | `D/k_candle_contract_domain.go:117-120` | `Dt/k_candle_contract_domain_test.go:259` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 指數價格高低於低 | 新增被拒，說明「指數價格最高不得低於最低」 | `D/contract_price_line_domain.go:37-40` | `Dt/k_candle_contract_domain_test.go:259` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 指數價格與最新價差很遠照收 | 被存入（不與最新價比較） | `D/k_candle_contract_domain.go:111-120` | `Dt/k_candle_contract_domain_test.go:324` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 新增時沒給指數價格 | 新增被拒，說明「指數價格必填」 | `D/contract_price_line_domain.go:30-35`；`C/models/k_candle_contract_request.go:35-38` | `Ct/contract_controllers_test.go:143`（index price left out）；`Dt/…domain_test.go:259` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 修改時沒給溢價指數 | 修改被拒，說明「溢價指數必填」；那一根維持原數字（不寫儲存） | `S/k_candle_contract_service.go:124-128` | `Ct/contract_controllers_test.go:259`（premium index left out, gomock 嚴格：未期待 Update） | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 — 這一刀之前存下的合約 K 線保留，由歷史同步補齊

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-14 | 舊的合約 K 線照查得到,兩組新價格顯示沒有值 | 舊的那根照舊回來、價量與標記價格不變；兩組新價格顯示「沒有值」而非零 | `E/k_candle_contract.go:51-59,86-93`；`dto/k_candle_contract_dto.go:29-36` | `Pt/k_candle_contract_repository_test.go`（HandsAnOldCandleOutWithoutTheLaterLines）；`Ct/contract_controllers_test.go:205` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 歷史同步把舊的那一天補齊 | 那一天被重問；每根補上兩組；原有價量、標記價格、成交筆數一個不改 | `P/k_candle_contract_repository.go:115-133`（只數四份齊全）、`:83-107`（衝突只寫兩組）；`S/contract_k_candle_ingestion_service.go:241-249` | `Pt/k_candle_contract_repository_test.go`（CountsOnlyCandlesCarryingBothLaterLines、SaveAllIfAbsentFillsInOnlyTheLinesAnOldCandleLacks）；`St/…ingestion_service_test.go:416` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 已經四份齊全的那一天連問都不問 | 那一天沒有向來源問任何東西 | `S/contract_k_candle_ingestion_service.go:247-250` | `St/…ingestion_service_test.go:395`；`Pt/…`（CountsOnly…） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 來源已經問不到那一天的指數價格 | 那一天每一根維持原狀、照舊保留；留下「補不上」的紀錄 | `S/contract_k_candle_ingestion_service.go:253-273`（缺指數 → domain 判不合格 → 不進批次、計入跳過／或整份失敗 → FetchFailureReason） | —（只有缺**標記價格**的同形測試 `St/…:802`；沒有任何測試以缺指數價格、且持有舊列的情境斷言舊列不動） | no-test | produces-oracle | 🟡 partial |
| AC-18 | 每分鐘那一輪不回頭處理舊的那幾根 | 只存下最近一根舊 K 線**之後**的新合約 K 線；那一根舊的依然沒有指數價格與溢價指數 | `S/contract_k_candle_ingestion_service.go:82-83`（ScheduledWindow＝最近 25 根已收盤分鐘，與最近一根存到哪無關）、`:430`（逐根 `Save`）；`P/k_candle_contract_repository.go:51-58`（`Save` 衝突時以 `contractFigureColumns` 覆寫，**含八個新欄**） | — | no-test | diverges：部署後重啟若在 25 分鐘內，最近那幾根舊 K 線落在每分鐘視窗內，會被整根覆寫並補上兩組新價格（也會重寫原有價量）；每分鐘那一輪也不是「只存它之後」 | 🔴 violation |

### US-04 — 資金費率結算照來源記下

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-19 | 一次一般的結算 | 存入 BTCUSDT 08:00 一筆，費率 +0.0001、標記價格 87000 | `D/contract_funding_rate_settlement_domain.go:27-54`；`S/contract_funding_rate_service.go:174-192` | `Dt/contract_funding_rate_settlement_domain_test.go:25`（一般的結算）；`St/contract_funding_rate_service_test.go:79` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 負的費率 | 存入，費率 -0.00003 | 同上（費率無規則） | `Dt/…settlement_domain_test.go:25`（負的費率）；`MDt/binance_contract_funding_rate_proxy_test.go:68` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 零費率 | 存入，費率 0 | 同上 | `Dt/…settlement_domain_test.go:25`（零費率） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 早年沒給標記價格的結算 | 存入，標記價格顯示「沒有值」而非零 | `MD/binance_contract_funding_rate_proxy.go:152-160`（空字串 → 無值）；`E/contract_funding_rate_settlement.go:30`（可為空）；`D/…settlement_domain.go:43` | `MDt/…funding_rate_proxy_test.go:68`；`Dt/…domain_test.go:25`（早年沒給）；`Ct/contract_funding_rate_settlement_controller_test.go:46`（`"markPrice":null`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 標記價格為零或負的那一筆跳過 | 那一筆不存並留紀錄；同批其他照存 | `D/…settlement_domain.go:43-46`；`S/contract_funding_rate_service.go:175-182` | `St/contract_funding_rate_service_test.go:118`；`Dt/…domain_test.go:62` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 結算時間不取整 | 存下的時間是 08:00:00.001 | `MD/…funding_rate_proxy.go:164`；`E/contract_funding_rate_settlement.go:25` | `MDt/…proxy_test.go:68`；`Dt/…domain_test.go:97`；`Pt/contract_funding_rate_settlement_repository_test.go:86` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 已存過的同一筆原封不動 | 08:00 仍只有一筆、費率仍 +0.0001 | `P/contract_funding_rate_settlement_repository.go:39-44`（DoNothing） | `Pt/…settlement_repository_test.go:25` | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 — 資金費率結算自己跟上

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-26 | 每小時只取上一次之後的新結算 | 只存 08:00 之後的結算 | `S/contract_funding_rate_service.go:151-167`；`MD/…funding_rate_proxy.go:60-65` | `St/…service_test.go:79`；`MDt/…proxy_test.go:88` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 這一小時沒有新結算 | 什麼都沒存、沒有失敗紀錄 | `P/…settlement_repository.go:35-37`；`S/…service.go:185-194` | `St/…service_test.go:102` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 加入名單時補齊完整歷史 | 從上市第一天起每一次結算都被存下 | `A/contract_trading_symbol_application.go:73-75`；`MD/…funding_rate_proxy.go:23,60-95`（2019-01-01 起、翻頁） | `At/contract_k_candle_application_test.go:166`（加入時以零時點問）；`MDt/…proxy_test.go:98,108` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 啟動時補上停機期間 | 名單上每一檔補上停機兩天；補完才開始每小時一輪 | `J/repeating_round.go:52-56`；`J/contract_funding_rate_ingestion_job.go:27-30` | `Jt/contract_series_jobs_test.go:161,273`；`St/…service_test.go:79` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 某一檔來源不答話 | BTC 本輪不存並留失敗紀錄；ETH 照存；下一輪 BTC 從上次存到處接著補 | `S/contract_funding_rate_service.go:69-76,166-172` | `St/…service_test.go:140`（隔離）、`:79`（接著上一次） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 名單是空的 | 沒向來源問任何東西 | `S/…service.go:58-76` | `St/…service_test.go:195`（gomock 嚴格） | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 — 持倉統計由三份資料合成

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-32 | 三份都齊 | 存入一筆 09:05 的持倉統計，三份數字都在 | `MD/binance_contract_position_statistic_proxy.go:96-150,156-186`；`D/contract_position_statistic_domain.go:82-93` | `MDt/binance_contract_position_statistic_proxy_test.go:81`；`Dt/contract_position_statistic_domain_test.go:31` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | 大戶多空持倉比獨缺 | 09:05 不存，留下「缺大戶多空持倉比」；其他三份齊全的時間點照存 | `D/…statistic_domain.go:61-65`；`S/contract_position_statistic_service.go:167-176` | `St/contract_position_statistic_service_test.go:130` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | 下一輪自然補上 | 上一輪因缺一份沒存的 09:05，這一次三份到齊時 09:05 被存入 | `D/contract_position_statistic_window_domain.go:35-41`（起點＝**最後存到那一筆**＋5 分鐘）；`S/…statistic_service.go:145-160` | —（沒有任何測試模擬「上一輪跳過 09:05」的下一輪） | no-test | diverges：AC-33 同一輪裡只要 09:05 之後有任何一筆被存（例如 09:10），下一輪就從 09:15 起問，09:05 永遠不會再被問到、永遠不會存入。只有被跳過的恰好是最新那一筆時才會補上。`S/…statistic_service.go:134-137` 的註解宣稱「gap left that way is asked about again」與實際不符 | 🔴 violation |
| AC-35 | 持倉量那一份整份問不到 | 該標的本輪一筆都不存、留失敗紀錄 | `MD/…statistic_proxy.go:103-107`；`S/…statistic_service.go:159-165` | `MDt/…statistic_proxy_test.go:176`；`St/…statistic_service_test.go:150` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 已存過的同一筆原封不動 | 09:05 仍只有一筆、數字不變 | `P/contract_position_statistic_repository.go:40-45`（DoNothing） | `Pt/contract_position_statistic_repository_test.go:31` | asserts-oracle | produces-oracle | ✅ conforms |

### US-07 — 持倉統計的合法性

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-37 | 一般的一筆 | 多 0.47／空 0.53／比 0.89 被存入 | `D/contract_position_statistic_domain.go:25-94` | `Dt/contract_position_statistic_domain_test.go:31`（一般的一筆） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 一面倒 | 多 1／空 0 被存入 | `D/…statistic_domain.go:69-74` | `Dt/…:31`（一面倒） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | 佔比超過一 | 不存，留「佔比必須介於零與一之間」 | `D/…statistic_domain.go:69-74`；`S/…statistic_service.go:171-174` | `Dt/…:69`（佔比超過一）；`St/…:130`（跳過紀錄路徑） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-40 | 持倉量為負 | 不存，留「持倉量不得為負」 | `D/…statistic_domain.go:44-47` | `Dt/…:69`（持倉量為負） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-41 | 統計時間不在五分鐘刻度 | 不存，留「統計時間必須落在五分鐘刻度」 | `D/…statistic_domain.go:34-38` | `Dt/…:69`（不在五分鐘刻度） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-42 | 統計時間指向未來 | 不存，留「統計時間不得指向未來」 | `D/…statistic_domain.go:39-42` | `Dt/…:69`（指向未來） | asserts-oracle | produces-oracle | ✅ conforms |

### US-08 — 持倉統計要持續錄

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-43 | 加入名單時補最近三十天 | 最近三十天的持倉統計被存下 | `A/contract_trading_symbol_application.go:77-80`；`D/contract_position_statistic_window_domain.go:32-35` | `At/contract_k_candle_application_test.go:166`（起點＝現在−30 天＋5 分） | asserts-oracle（起點比整三十天晚一格五分鐘，ARCH §8 已記為接受的代價） | produces-oracle | ✅ conforms |
| AC-44 | 每五分鐘接著上一次 | 只存 09:05 之後到現在的持倉統計 | `D/…window_domain.go:36-46` | `St/…statistic_service_test.go:84`；`Dt/…domain_test.go:120` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-45 | 啟動時補上停機期間 | 名單上每一檔補上兩天；補完才開始每五分鐘一輪 | `J/repeating_round.go:52-56`；`J/contract_position_statistic_ingestion_job.go:26-29` | `Jt/contract_series_jobs_test.go:147,273` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-46 | 停機超過三十天 | 只補最近三十天、沒有失敗紀錄 | `D/…window_domain.go:36-41` | `Dt/…domain_test.go:120`（上一次早於三十天也一樣） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-47 | 名單上有一檔從沒存過 | 從三十天前開始取 | `D/…window_domain.go:32-35` | `St/…statistic_service_test.go:101`；`Dt/…:157` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-48 | 名單是空的 | 沒向來源問任何東西 | `S/contract_position_statistic_service.go:57-76` | `St/…statistic_service_test.go:202` | asserts-oracle | produces-oracle | ✅ conforms |

### US-09 — 合約交易規格

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-49 | 加入名單時記下規格 | 加入成功；清單上 BTCUSDT 帶跳動 0.1、步進 0.001、最小名目 50 與確認時間 | `MD/binance_contract_symbol_lookup_proxy.go:133-168`；`S/contract_trading_symbol_service.go:124-137`；`E/contract_trading_symbol.go:54-75` | `MDt/binance_contract_symbol_lookup_proxy_test.go:217`；`St/contract_trading_symbol_service_test.go:186,313` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-50 | 每天刷新 | 跳動變 0.01、規格最後更新時間變成這一次 | `S/contract_trading_symbol_service.go:150-195`；`J/contract_trading_specification_refresh_job.go:19-35` | `St/…symbol_service_test.go:211`；`Pt/contract_trading_symbol_repository_test.go:243` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-51 | 已移出名單的標的照樣刷新 | 已移出但仍認得的標的規格照樣更新 | `S/…symbol_service.go:153-156`（FindAll） | `St/…symbol_service_test.go:211`（ETHUSDT 未追蹤仍刷新） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-52 | 來源不再列出的標的保留最後一次的規格 | 保留原規格、更新時間仍是上週 | `S/…symbol_service.go:177-181`；`MD/…lookup_proxy.go:190-194` | `St/…symbol_service_test.go:211`（OMGUSDT 不在寫入清單）、`:262` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-53 | 來源不答話 | 每個標的保留原規格、留失敗紀錄；下一次照常再試 | `S/…symbol_service.go:159-163`；`J/…refresh_job.go:27-31` | `St/…symbol_service_test.go:275`（來源不答話）；`Jt/contract_series_jobs_test.go:203`（交易規格來源不答話 + 下一輪照跑） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-54 | 沒特別列出結算間隔就是八小時 | 結算間隔記為八小時 | `D/contract_trading_specification_domain.go:15,63-66` | `Dt/contract_trading_specification_domain_test.go:27`；`St/…symbol_service_test.go:211` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-55 | 四小時結算的標的 | 結算間隔記為四小時 | `MD/…lookup_proxy.go:337-339`；`D/…specification_domain.go:64-66` | `MDt/…lookup_proxy_test.go:236`；`St/…:211` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-56 | 還沒刷新過的舊標的 | 它在清單上，交易規格顯示「沒有值」 | `E/contract_trading_symbol.go:38-45,60-72` | `St/…symbol_service_test.go:313`（ETHUSDT 規格為 nil） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-57 | 啟動時刷新一次 | 啟動後所有合約標的規格都刷新過一次 | `J/repeating_round.go:53`；`cmd/server/dependencies.go:692-695` | `Jt/contract_series_jobs_test.go:147`（交易規格刷新） | asserts-oracle | produces-oracle | ✅ conforms |

### US-10 — 四種資料彼此獨立，移除只停止追蹤

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-58 | 資金費率失敗不影響其他 | BTC 資金費率失敗時，其持倉統計與合約 K 線照常存入 | `cmd/server/dependencies.go:684-695`（各自一個 job）；三個 service 各自獨立的 round | —（`cmd/server/dependencies_test.go` 只數 job 個數；沒有測試讓一種資料失敗而斷言另一種照存） | no-test | produces-oracle | 🟡 partial |
| AC-59 | 移除只停止追蹤 | 之後每一輪都不再抓 BTC 的三種資料；已存的每一筆都還查得到 | `S/contract_trading_symbol_service.go:205-227`（只改 `IsWatched`）；三個 round 都讀 `FindWatched`（`S/contract_funding_rate_service.go:58`、`S/contract_position_statistic_service.go:57`） | `St/contract_trading_symbol_service_test.go:112`；`At/contract_k_candle_application_test.go:225` | shallow：只斷言「存成未追蹤」；沒有測試斷言移除後資金費率／持倉統計的 round 不再問 BTC，也沒有斷言已存的結算與統計仍查得到 | produces-oracle | 🟠 mis-asserted |
| AC-60 | 現貨不受影響 | 合約任一種資料抓取或失敗時，現貨 BTCUSDT 照常抓、資料不變 | 新資料各自獨立的表、proxy、pacer（`cmd/server/dependencies.go:207-227,735-740`） | —（本切片沒有測試；前一刀只有合約／現貨 K 線表分開的測試） | no-test | produces-oracle | 🟡 partial |

### US-11 — 查得到

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-61 | 查資金費率結算 | 回那九筆，依結算時間由早到晚 | `C/contract_funding_rate_settlement_controller.go:29-64`；`P/contract_funding_rate_settlement_repository.go:74-93` | `Pt/…settlement_repository_test.go:55`；`Ct/…settlement_controller_test.go:46` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-62 | 查持倉統計 | 回那十三筆，依統計時間由早到晚 | `C/contract_position_statistic_controller.go:27-62`；`P/contract_position_statistic_repository.go`（FindInRange） | `Pt/contract_position_statistic_repository_test.go:59`；`Ct/contract_position_statistic_controller_test.go:38` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-63 | 那段時間一筆都沒有 | 回空的，不算錯誤 | `S/contract_funding_rate_service.go:133-138` | `Ct/…settlement_controller_test.go:46`（empty stretch）；`Ct/…statistic_controller_test.go:38` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-64 | 沒指定合約標的 | 查詢被拒，說明「必須指定交易標的」 | `D/trading_symbol_domain.go:32-34`（經 `KCandleQueryDomain`）；`S/…funding_rate_service.go:112-118`、`S/…statistic_service.go:110-113` | `Ct/…settlement_controller_test.go:46`；`Ct/…statistic_controller_test.go:38`；`St/contract_funding_rate_service_test.go:285` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-65 | 查合約 K 線多帶兩組價格 | 四份齊全那一根回來時帶著指數價格與溢價指數的開高低收 | `E/k_candle_contract.go:86-93`；`dto/k_candle_contract_dto.go:29-36` | —（查詢測試只斷言**舊** K 線顯示 null：`Ct/contract_controllers_test.go:205`、`Pt/…HandsAnOldCandleOut…`；沒有任何測試斷言四份齊全那根在查詢結果裡帶出兩組數值） | no-test | produces-oracle | 🟡 partial |

### Core Business Rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | **缺一不存**：合約 K 線四份、持倉統計三份，以時間對齊，缺任何一份那一筆就不存、留紀錄、同一批其他照常 | 任一份缺（不論哪一份）→ 那一筆不存且**留紀錄**；同批其他照存 | `MD/binance_contract_market_data_proxy.go:89-95,125-145`（以價量那份決定哪些分鐘存在）；`MD/binance_contract_position_statistic_proxy.go:114-118,133-137`（以持倉量那份決定哪些時間點存在） | `St/…ingestion_service_test.go:141,165`；`St/…statistic_service_test.go:130` | asserts-oracle（僅就「缺次要那幾份」） | diverges：缺的是**主那一份**（價量或持倉量）時，另外幾份的那一分鐘／時間點被靜默丟棄，**沒有留下任何紀錄**（`MD/…market_data_proxy.go:40-43` 註解明言 dropped；統計 `:87-88` 同）。缺標記／指數／溢價或兩個多空比時才有紀錄 | 🔴 violation |
| BR-2 | **接著上一次**；從沒存過的，資金費率從上市第一天、持倉統計從三十天前 | 兩者都從上次存到之後接著問；空的起點分別為第一天／三十天前 | `S/contract_funding_rate_service.go:159-167`；`MD/…funding_rate_proxy.go:23,60-65`；`D/contract_position_statistic_window_domain.go:32-46` | `St/…funding_rate_service_test.go:79,102`；`MDt/…funding_rate_proxy_test.go:88,98`；`Dt/…statistic_domain_test.go:120,157` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | **已存過的原封不動**：再抓到同一筆時不覆蓋 | 同一筆再抓到時保留原值 | `P/contract_funding_rate_settlement_repository.go:39-44`；`P/contract_position_statistic_repository.go:40-45` | `Pt/…settlement_repository_test.go:25`；`Pt/…statistic_repository_test.go:31` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | **判斷一天齊不齊只算四份都有的合約 K 線**；補齊舊的只補缺的兩組，原有的數字不動 | 齊不齊只數四份齊全的；補舊的只寫兩組新價格，其餘不動 | `P/k_candle_contract_repository.go:115-133,83-107` | `Pt/k_candle_contract_repository_test.go`（CountsOnly…、FillsInOnly…、LeavesACompleteCandleExactlyAsItWas） | asserts-oracle | produces-oracle（見 Orphan O-7 的細節） | ✅ conforms |
| BR-5 | **沒有值與零是兩件事**：舊 K 線兩組新價格、早年結算標記價格、未刷新過的規格都是「沒有值」 | 三者對外都呈現「沒有值」（null），不是 0 | `E/k_candle_contract.go:51-59`；`E/contract_funding_rate_settlement.go:30`；`E/contract_trading_symbol.go:38-45,60-72` | `Pt/…HandsAnOldCandleOut…`；`Ct/…settlement_controller_test.go:46`；`St/…symbol_service_test.go:313` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | **規格只記現在這一份**；來源不再列出或不答話時保留原本的 | 只存最新一份；不再列出或失敗時原規格與更新時間不變 | `S/contract_trading_symbol_service.go:159-194`；`P/contract_trading_symbol_repository.go:88-109,121-143` | `St/…symbol_service_test.go:211,275`；`Pt/contract_trading_symbol_repository_test.go:218,243,280` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | **各自獨立**：四種資料、每個標的彼此獨立；任何失敗只留紀錄、下一輪自然重試 | 一個標的或一種資料失敗不影響其他；失敗只留紀錄；下一輪重試 | 逐標的 `sync.WaitGroup`（`S/contract_funding_rate_service.go:69-76`、`S/contract_position_statistic_service.go:67-74`）；各自 job（`cmd/server/dependencies.go:684-695`） | 逐標的：`St/…funding_rate_service_test.go:140`、`St/…statistic_service_test.go:150`；失敗不停 job：`Jt/contract_series_jobs_test.go:203` | 逐標的獨立有斷言；**跨資料種類**的獨立沒有任何測試（同 AC-58） | produces-oracle | 🟡 partial |

### Non-Functional Requirements

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-1 | **取用額度**：持倉統計的來源與 K 線來源的取用額度分開計算，各自守各自的節奏，互不擠占 | 持倉統計用自己的額度；K 線／資金費率／標的清單共用另一份 | `cmd/server/dependencies.go:223`（統計用 `cryptoContractStatistics`）、`:207-213`、`:735-740`；`internal/config/application_config.go`（`CONTRACT_MARKET_DATA_STATISTICS_REQUESTS_PER_MINUTE`） | —（`internal/config/tests/contract_ingestion_config_test.go` 只測設定讀取；沒有測試斷言兩個 proxy 拿到不同的 pacer） | no-test | produces-oracle | 🟡 partial |
| NFR-2 | **不重問**：每一輪只問上一次之後的部分；已經齊全的東西不重問 | 資金費率／持倉統計只問上次之後；齊全的日子不重問 | `S/contract_funding_rate_service.go:159-167`；`D/contract_position_statistic_window_domain.go:36-46`；`S/contract_k_candle_ingestion_service.go:247-250`；`MD/…market_data_proxy.go:97-102` | `St/…funding_rate_service_test.go:79`；`St/…statistic_service_test.go:84,118`；`St/…ingestion_service_test.go:395`；`MDt/…market_data_proxy_test.go:484` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | **資料量**：完整資金費率歷史數千筆一次補齊可接受；持倉統計三十天約八千多筆 | 一次補齊數千筆結算與八千多筆統計不失敗 | `P/contract_funding_rate_settlement_repository.go:27,44`；`P/contract_position_statistic_repository.go`（`CreateInBatches` 1000） | `Pt/…settlement_repository_test.go:133`；`Pt/…statistic_repository_test.go:115` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `MD/binance_contract_symbol_lookup_proxy.go:150-154` | 加入名單時多問一份結算間隔清單；那一份失敗會讓**整個加入被拒**（當作來源不可用），加入名單多了一個新的失敗理由 | undocumented |
| `S/contract_trading_symbol_service.go:131-135,166-171`；`MD/…lookup_proxy.go:159-162,199-202` | 來源回報的規格讀不懂或不合規則（跳動／步進／最小量須 > 0 等）時，靜默略過、**不留任何紀錄**；該標的就一直保留舊規格或維持沒有值 | undocumented |
| `D/contract_funding_rate_settlement_domain.go:36-39` | 資金費率結算多一條「結算時間不得指向未來」的規則 | undocumented |
| `D/contract_position_statistic_domain.go:44,76-79` | 持倉統計多兩條規則：持倉價值不得為負（訊息卻說「持倉量不得為負」）、多空比值不得為負 | undocumented |
| `S/contract_funding_rate_service.go:128-131`；`S/contract_position_statistic_service.go:121-124` | 兩種查詢超過單次上限時整個拒絕（「時間區間過大，請縮小區間」） | undocumented（ARCH 有提，PRD 沒有） |
| `S/contract_trading_symbol_service.go:153-156` | 規格刷新只涵蓋**已登錄**的標的；只因手動存過 K 線而出現在合約標的清單上的標的（`ListContractTradingSymbols` 會列出）永遠沒有規格。PRD Flow 寫的是「系統認得的每個合約標的」 | undocumented（和 Flow 用詞之間有落差，需要確認） |
| `P/k_candle_contract_repository.go:93-99` | 衝突時只要**任一組**為空，就把**兩組**一起以新值覆寫；已經有溢價指數、只缺指數價格的那根，它的溢價指數也會被改寫。domain 保證新寫入的兩組同在，所以目前走不到 | undocumented |

Out of Scope 檢核：沒有發現任何程式碼落在 PRD Out of Scope 的範圍內（見上方負面檢核清單），因此沒有 scope creep 違規。

## Summary

- Conforms: 65/75 clauses ✅（86.7%）
- Violations: AC-18, AC-34, BR-1（程式產生的結果不對）
- Mis-asserted: AC-59（綠燈測試斷言的比 oracle 弱）
- Partial: AC-17, AC-58, AC-60, AC-65, BR-7, NFR-1（沒有測試斷言 oracle）
- Gaps: 無
- Unclear: 無
- Orphans: 7（皆為 undocumented；沒有 out-of-scope 違規）

### 修正建議（依優先序）

1. **AC-34（持倉統計的洞永遠補不回來）**：視窗起點取「最後存到那一筆＋5 分鐘」，所以中間被跳過的時間點再也不會被問到。可以擇一處理：讓視窗往回多涵蓋幾格（像 K 線每分鐘那一輪一樣），或每一輪從最早一個被跳過的時間點問起；也可以把 PRD 改成「只有最新那一筆會自然補上」。改完後補一支測試：上一輪 09:05 被跳過而 09:10 已存，這一輪 09:05 到齊就要存入。
2. **AC-18（每分鐘那一輪會覆寫最近的舊 K 線）**：`Save` 衝突時覆寫 `contractFigureColumns`，其中含八個新欄，每分鐘視窗內的舊 K 線因此會被補上兩組並重寫價量。要嘛讓每分鐘那一輪不動「在這一刀之前存下的」列，要嘛修改 PRD 的 Then（接受每分鐘那一輪順手補齊最近 25 分鐘）。
3. **BR-1（缺主那一份時沒有紀錄）**：價量或持倉量缺了某個時間點、而其他份有值時，應該留下「缺價量／缺持倉量」的紀錄；不然就把 BR-1 改成「以價量／持倉量那份為準」，與 ARCH 的決定對齊。
4. **AC-59**：補測試，斷言移除後資金費率與持倉統計的 round 不再問該標的，且已存的結算與統計仍查得到。
5. **AC-17 / AC-58 / AC-60 / AC-65 / BR-7 / NFR-1**：各補一支直接斷言 oracle 的測試（缺指數價格時舊列不動並留紀錄；一種資料失敗時另一種照存；現貨不受影響；四份齊全的 K 線在查詢結果裡帶出兩組數值；統計用獨立的 pacer）。
