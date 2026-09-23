# Contract Traceability Matrix — 合約行情補充資料（contract-market-supplements）

Contract: PRD.md（v1.0 Draft）
Design map: ARCH.md（Confirmed）
Implementation: branch `feat/contract-market-context`（`git diff main...HEAD`）
Oracle: Acceptance Criteria（84 clauses：AC 69 · BR 12 · NFR 3）
Revision: 第二輪，修正後重驗（基準 `5461c9c..2a1ac8f`）。場景文字沒變的沿用第一輪 ID，新場景與新規則接在後面（AC-66–69、BR-8–12）。

> 靜態符合度稽核：以 PRD 的驗收條件為 oracle，分別判斷「測試是否斷言 oracle」與「正式程式碼是否產生 oracle」，
> 不以整套測試綠燈為依據、也不執行自創情境。只對個別已對應的測試做過佐證執行，全部通過：
> 第一輪跑了 domain service 與 PostgreSQL repository 數支；第二輪跑了持倉統計的視窗／domain／service、移除、舊 K 線歷史同步、標的清單 proxy 與 venue pacer。
>
> 第二輪重驗的範圍：第一輪所有非 conforming 的條款、PRD 文字有改的條款（AC-18、AC-44、BR-1、BR-2）、新增條款（AC-66–69、BR-8–12），以及 Orphans。
> 其餘條款的程式與測試在 `5461c9c..HEAD` 之間沒有變動，沿用第一輪的判定。

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
| AC-17 | 來源已經問不到那一天的指數價格 | 那一天每一根維持原狀、照舊保留；留下「補不上」的紀錄 | `S/contract_k_candle_ingestion_service.go:253-273`（缺指數 → domain 判不合格 → 不進批次、計入跳過；整份失敗 → FetchFailureReason） | `St/contract_k_candle_ingestion_service_test.go:833`（LeavesOldCandlesAsTheyWereWhenTheVenueNoLongerHasTheirIndexPrice：交給儲存的批次為空、SkippedCount > 0） | asserts-oracle | produces-oracle（第一輪 🟡，測試已補） | ✅ conforms |
| AC-18 | 每分鐘那一輪不回頭處理舊的那幾根（**第二輪 PRD 已改寫**：Given 舊 K 線早於每分鐘那一輪本來就會重抓的最近幾分鐘；Then 只重抓最近那幾分鐘，那一根舊的依然沒有兩組新價格） | 只重抓最近幾分鐘；早於那段的舊 K 線原封不動、仍沒有指數價格與溢價指數 | `D/k_candle_ingestion_domain.go:94-102`（ScheduledWindow＝最近 25 根已收盤分鐘）；`S/contract_k_candle_ingestion_service.go:82-83,430` | `Dt/k_candle_ingestion_domain_test.go:63`（只問最近幾根——共用的既有測試） | 部分：「只重抓最近幾分鐘」有斷言；沒有測試放一根更早的舊 K 線、斷言每分鐘那一輪之後它仍沒有兩組新價格 | produces-oracle（第一輪 🔴 是 PRD 寫錯，PRD 已更正） | 🟡 partial |
| AC-66 | 每分鐘那一輪本來就會重抓的最近幾分鐘照舊重抓（**新增**） | 落在最近幾分鐘裡的舊 K 線，跟其他幾根一樣換成這次抓到的完整一根，帶著指數價格與溢價指數 | `S/contract_k_candle_ingestion_service.go:430`；`P/k_candle_contract_repository.go:18-25,51-58`（衝突時覆寫 `contractFigureColumns`，含八個新欄） | —（`Pt/k_candle_contract_repository_test.go:96` 兩次寫入都是完整 K 線，只斷言收盤價；沒有測試先放一根缺兩組的舊 K 線，再斷言 `Save` 之後兩組有值） | no-test | produces-oracle | 🟡 partial |

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
| AC-34 | 下一輪自然補上 | 上一輪因缺一份沒存的 09:05，這一次三份到齊時 09:05 被存入 | `D/contract_position_statistic_window_domain.go:16,43-50`（起點＝最後一筆＋5 分−1 小時，不早於三十天邊界）；`S/contract_position_statistic_service.go:145-185` | `St/contract_position_statistic_service_test.go:321`；`Dt/contract_position_statistic_domain_test.go:120` | asserts-oracle | produces-oracle（第一輪 🔴，已修正） | ✅ conforms |
| AC-67 | 被跳過的那一筆後面已經存了新的,下一輪照樣補上（**新增**） | 09:05 被存入；09:10 原封不動 | 同上 ＋ `P/contract_position_statistic_repository.go:40-45`（DoNothing） | `St/…statistic_service_test.go:321`（視窗 08:15 起、批次含 09:05）；`Pt/contract_position_statistic_repository_test.go:31`（已存的不覆蓋） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-35 | 持倉量那一份整份問不到 | 該標的本輪一筆都不存、留失敗紀錄 | `MD/…statistic_proxy.go:103-107`；`S/…statistic_service.go:159-165` | `MDt/…statistic_proxy_test.go:176`；`St/…statistic_service_test.go:150` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 已存過的同一筆原封不動 | 09:05 仍只有一筆、數字不變 | `P/contract_position_statistic_repository.go:40-45`（DoNothing） | `Pt/contract_position_statistic_repository_test.go:31` | asserts-oracle | produces-oracle | ✅ conforms |

### US-07 — 持倉統計的合法性

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-37 | 一般的一筆 | 多 0.47／空 0.53／比 0.89 被存入 | `D/contract_position_statistic_domain.go:25-94` | `Dt/contract_position_statistic_domain_test.go:31`（一般的一筆） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 一面倒 | 多 1／空 0 被存入 | `D/…statistic_domain.go:69-74` | `Dt/…:31`（一面倒） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | 佔比超過一 | 不存，留「佔比必須介於零與一之間」 | `D/…statistic_domain.go:69-74`；`S/…statistic_service.go:171-174` | `Dt/…:69`（佔比超過一）；`St/…:130`（跳過紀錄路徑） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-40 | 持倉量為負 | 不存，留「持倉量不得為負」 | `D/…statistic_domain.go:44-47` | `Dt/…:69`（持倉量為負） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-68 | 持倉價值為負（**新增**） | 不存，留「持倉價值不得為負」 | `D/contract_position_statistic_domain.go:48-51` | `Dt/contract_position_statistic_domain_test.go:69`（持倉價值為負） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-69 | 比值為負（**新增**） | 不存，留「大戶多空持倉比的比值不得為負」 | `D/…statistic_domain.go:80-83` | `Dt/…:69`（比值為負） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-41 | 統計時間不在五分鐘刻度 | 不存，留「統計時間必須落在五分鐘刻度」 | `D/…statistic_domain.go:34-38` | `Dt/…:69`（不在五分鐘刻度） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-42 | 統計時間指向未來 | 不存，留「統計時間不得指向未來」 | `D/…statistic_domain.go:39-42` | `Dt/…:69`（指向未來） | asserts-oracle | produces-oracle | ✅ conforms |

### US-08 — 持倉統計要持續錄

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-43 | 加入名單時補最近三十天 | 最近三十天的持倉統計被存下 | `A/contract_trading_symbol_application.go:77-80`；`D/contract_position_statistic_window_domain.go:32-35` | `At/contract_k_candle_application_test.go:166`（起點＝現在−30 天＋5 分） | asserts-oracle（起點比整三十天晚一格五分鐘，ARCH §8 已記為接受的代價） | produces-oracle | ✅ conforms |
| AC-44 | 每五分鐘接著上一次（**第二輪 Then 已改寫**：從 09:05 前一小時問到現在,只存下還沒存過的那幾筆） | 從 09:05 往前一小時問到現在；只有還沒存過的被存下 | `D/contract_position_statistic_window_domain.go:43-50`；`P/contract_position_statistic_repository.go:40-45` | `St/…statistic_service_test.go:84`（08:10–10:00）；`Dt/…domain_test.go:120` | asserts-oracle | produces-oracle（邊界註記：起點是 08:10，也就是結束於 09:05 的那一小時裡的 12 格。若「前一小時」要含 08:05 那一格，程式與測試都要往前移一格；兩種讀法都說得通，判為符合） | ✅ conforms |
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
| AC-59 | 移除只停止追蹤 | 之後每一輪都不再抓 BTC 的三種資料；已存的每一筆都還查得到 | `S/contract_trading_symbol_service.go:205-227`（只改 `IsWatched`）；三個 round 都讀 `FindWatched`（`S/contract_funding_rate_service.go:58`、`S/contract_position_statistic_service.go:57`）；`P/contract_trading_symbol_repository.go:46-60` | `At/contract_watchlist_removal_application_test.go:18`（兩個 proxy 沒設任何期待、round 報告為空、結算與統計照查得到）；`Pt/contract_trading_symbol_repository_test.go:60`（FindWatched 排除未追蹤）；合約 K 線那一輪沿用前一刀的測試 | asserts-oracle | produces-oracle（第一輪 🟠，測試已補） | ✅ conforms |
| AC-60 | 現貨不受影響 | 合約任一種資料抓取或失敗時，現貨 BTCUSDT 照常抓、資料不變 | 新資料各自獨立的表、proxy、pacer（`cmd/server/dependencies.go:207-227,735-740`） | —（本切片沒有測試；前一刀只有合約／現貨 K 線表分開的測試） | no-test | produces-oracle | 🟡 partial |

### US-11 — 查得到

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-61 | 查資金費率結算 | 回那九筆，依結算時間由早到晚 | `C/contract_funding_rate_settlement_controller.go:29-64`；`P/contract_funding_rate_settlement_repository.go:74-93` | `Pt/…settlement_repository_test.go:55`；`Ct/…settlement_controller_test.go:46` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-62 | 查持倉統計 | 回那十三筆，依統計時間由早到晚 | `C/contract_position_statistic_controller.go:27-62`；`P/contract_position_statistic_repository.go`（FindInRange） | `Pt/contract_position_statistic_repository_test.go:59`；`Ct/contract_position_statistic_controller_test.go:38` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-63 | 那段時間一筆都沒有 | 回空的，不算錯誤 | `S/contract_funding_rate_service.go:133-138` | `Ct/…settlement_controller_test.go:46`（empty stretch）；`Ct/…statistic_controller_test.go:38` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-64 | 沒指定合約標的 | 查詢被拒，說明「必須指定交易標的」 | `D/trading_symbol_domain.go:32-34`（經 `KCandleQueryDomain`）；`S/…funding_rate_service.go:112-118`、`S/…statistic_service.go:110-113` | `Ct/…settlement_controller_test.go:46`；`Ct/…statistic_controller_test.go:38`；`St/contract_funding_rate_service_test.go:285` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-65 | 查合約 K 線多帶兩組價格 | 四份齊全那一根回來時帶著指數價格與溢價指數的開高低收 | `E/k_candle_contract.go:86-93`；`dto/k_candle_contract_dto.go:29-36` | `Ct/contract_controllers_test.go:221`（hands a complete candle out with its index price and premium index：八個值都在回應裡） | asserts-oracle | produces-oracle（第一輪 🟡，測試已補） | ✅ conforms |

### Core Business Rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | **缺一不存**：合約 K 線四份、持倉統計三份，以時間對齊，缺任何一份那一筆就不存、留紀錄、同一批其他照常。（**第二輪補充**）**價量（合約 K 線）與持倉量（持倉統計）決定那一刻存不存在**：它們沒有的那一刻，其他幾份的答案直接丟掉、不留紀錄 | 缺次要那幾份 → 那一筆不存、留紀錄、同批其他照存；價量或持倉量沒有的那一刻 → 其他幾份靜默丟棄、沒有紀錄 | `MD/binance_contract_market_data_proxy.go:89-102,125-145`；`MD/binance_contract_position_statistic_proxy.go:114-118,133-137`；`D/contract_price_line_domain.go:30-35`；`D/contract_position_statistic_domain.go:65-70` | 缺次要那幾份：`St/…ingestion_service_test.go:141,165`、`St/…statistic_service_test.go:130`；缺主那一份：`MDt/…market_data_proxy_test.go:484`（沒成交就其他份都不問、結果為空）、`MDt/…statistic_proxy_test.go:150` | asserts-oracle | produces-oracle（第一輪 🔴，PRD 已依設計補上規則） | ✅ conforms |
| BR-2 | **接著上一次**；從沒存過的，資金費率從上市第一天、持倉統計從三十天前。（**第二輪補充**）持倉統計**另外重問上一筆之前的一小時** | 資金費率從上次之後接著問；持倉統計從上一筆前一小時起問；沒存過時的起點分別為上市第一天／三十天前 | `S/contract_funding_rate_service.go:159-167`；`MD/…funding_rate_proxy.go:23,60-65`；`D/contract_position_statistic_window_domain.go:16,40-50` | `St/…funding_rate_service_test.go:79,102`；`MDt/…funding_rate_proxy_test.go:88,98`；`Dt/…statistic_domain_test.go:120,157`；`St/…statistic_service_test.go:84,321` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | **已存過的原封不動**：再抓到同一筆時不覆蓋 | 同一筆再抓到時保留原值 | `P/contract_funding_rate_settlement_repository.go:39-44`；`P/contract_position_statistic_repository.go:40-45` | `Pt/…settlement_repository_test.go:25`；`Pt/…statistic_repository_test.go:31` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | **判斷一天齊不齊只算四份都有的合約 K 線**；補齊舊的只補缺的兩組，原有的數字不動 | 齊不齊只數四份齊全的；補舊的只寫兩組新價格，其餘不動 | `P/k_candle_contract_repository.go:115-133,83-107` | `Pt/k_candle_contract_repository_test.go`（CountsOnly…、FillsInOnly…、LeavesACompleteCandleExactlyAsItWas） | asserts-oracle | produces-oracle（見 Orphan O-7 的細節） | ✅ conforms |
| BR-5 | **沒有值與零是兩件事**：舊 K 線兩組新價格、早年結算標記價格、未刷新過的規格都是「沒有值」 | 三者對外都呈現「沒有值」（null），不是 0 | `E/k_candle_contract.go:51-59`；`E/contract_funding_rate_settlement.go:30`；`E/contract_trading_symbol.go:38-45,60-72` | `Pt/…HandsAnOldCandleOut…`；`Ct/…settlement_controller_test.go:46`；`St/…symbol_service_test.go:313` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | **規格只記現在這一份**；來源不再列出或不答話時保留原本的 | 只存最新一份；不再列出或失敗時原規格與更新時間不變 | `S/contract_trading_symbol_service.go:159-194`；`P/contract_trading_symbol_repository.go:88-109,121-143` | `St/…symbol_service_test.go:211,275`；`Pt/contract_trading_symbol_repository_test.go:218,243,280` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | **各自獨立**：四種資料、每個標的彼此獨立；任何失敗只留紀錄、下一輪自然重試 | 一個標的或一種資料失敗不影響其他；失敗只留紀錄；下一輪重試 | 逐標的 `sync.WaitGroup`（`S/contract_funding_rate_service.go:69-76`、`S/contract_position_statistic_service.go:67-74`）；各自 job（`cmd/server/dependencies.go:684-695`） | 逐標的：`St/…funding_rate_service_test.go:140`、`St/…statistic_service_test.go:150`；失敗不停 job：`Jt/contract_series_jobs_test.go:203` | 逐標的獨立有斷言；**跨資料種類**的獨立沒有任何測試（同 AC-58） | produces-oracle | 🟡 partial |
| BR-8 | **每分鐘那一輪只重抓最近幾分鐘**（既有行為）：落在那幾分鐘裡的舊合約 K 線會跟著被換成完整的一根；更早的只有歷史同步會補（**新增**） | 每分鐘那一輪只問最近幾分鐘；那段裡的舊 K 線變成完整一根；更早的只由歷史同步處理 | 同 AC-18、AC-66 | 同 AC-18、AC-66 | 部分：只有「只問最近幾分鐘」有斷言；舊 K 線被換成完整一根、更早的不動，兩者都沒有測試 | produces-oracle | 🟡 partial |
| BR-9 | **查詢上限**：資金費率結算與持倉統計沿用合約 K 線的單次上限，超過即整次拒絕並請縮小區間（**新增**） | 超過上限時整次拒絕，說明要縮小區間 | `S/contract_funding_rate_service.go:128-131`；`S/contract_position_statistic_service.go:121-124` | `St/contract_funding_rate_service_test.go:285`（區間裡超過上限）；`St/contract_position_statistic_service_test.go:292` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | **結算時間與統計時間都不得指向未來**（**新增**） | 指向未來的結算、統計那一筆不存 | `D/contract_funding_rate_settlement_domain.go:36-39`；`D/contract_position_statistic_domain.go:39-42` | `Dt/contract_funding_rate_settlement_domain_test.go:62`（結算時間指向未來）；`Dt/contract_position_statistic_domain_test.go:69`（指向未來） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | **交易規格的刷新涵蓋已登錄的合約標的**（在名單上或曾經在名單上）；只因手動存過合約 K 線而出現在清單上的標的沒有規格（**新增**） | 已登錄的（不論是否在追）都被刷新；只有 K 線、沒登錄的標的在清單上顯示沒有規格 | `S/contract_trading_symbol_service.go:153-156,177-182`（只走 FindAll）、`:72-75`（只有 K 線的標的不帶規格） | `St/contract_trading_symbol_service_test.go:211`（未登錄的 NOTREGISTEREDUSDT 不寫入、未追蹤的 ETHUSDT 照刷）；`St/…:45` 只斷言只有 K 線的 1000PEPEUSDT 在清單上且未追蹤，沒有斷言它的規格為空 | 部分：前半有斷言；「只有 K 線的標的沒有規格」沒有測試 | produces-oracle | 🟡 partial |
| BR-12 | **加入名單時確認代號與記下規格是兩件事**：確認可追蹤就加入；規格讀不懂、不合規則或結算間隔那份答案問不到時，照樣加入、先沒有規格，由每天的刷新補上（**新增**） | 這三種規格問題都不擋加入；加入後規格顯示沒有值，等每天的刷新補上 | `MD/binance_contract_symbol_lookup_proxy.go:155-167`；`S/contract_trading_symbol_service.go:131-137`；`P/contract_trading_symbol_repository.go:93-96`（沒帶規格就不動規格欄） | `MDt/binance_contract_symbol_lookup_proxy_test.go:247`（讀不懂）、`:258`（結算間隔問不到）；`At/contract_k_candle_application_test.go:166`（零值規格 → 存入的只有代號與追蹤）；`Pt/contract_trading_symbol_repository_test.go:218` | asserts-oracle | produces-oracle | ✅ conforms |

### Non-Functional Requirements

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-1 | **取用額度**：持倉統計的來源與 K 線來源的取用額度分開計算，各自守各自的節奏，互不擠占 | 持倉統計用自己的額度；K 線／資金費率／標的清單共用另一份 | `cmd/server/dependencies.go:223`（統計 proxy 用 `cryptoContractStatistics`）、`:735-740` | `cmd/server/venue_pacers_test.go:17`（`cryptoContract` 與 `cryptoContractStatistics` 的 limiter 不是同一個；額度讀自設定） | asserts-oracle（統計 proxy 接哪一個 pacer，是組裝根裡的一行，已讀碼確認） | produces-oracle（第一輪 🟡，測試已補） | ✅ conforms |
| NFR-2 | **不重問**：每一輪只問上一次之後的部分；已經齊全的東西不重問 | 資金費率／持倉統計只問上次之後；齊全的日子不重問 | `S/contract_funding_rate_service.go:159-167`；`D/contract_position_statistic_window_domain.go:36-46`；`S/contract_k_candle_ingestion_service.go:247-250`；`MD/…market_data_proxy.go:97-102` | `St/…funding_rate_service_test.go:79`；`St/…statistic_service_test.go:84,118`；`St/…ingestion_service_test.go:395`；`MDt/…market_data_proxy_test.go:484` | asserts-oracle | produces-oracle（註：持倉統計每輪重問最近一小時是 BR-2 明文的例外，PRD 說明它落在同一頁、不多花請求；NFR-2 的文字沒提到這個例外，建議補一句） | ✅ conforms |
| NFR-3 | **資料量**：完整資金費率歷史數千筆一次補齊可接受；持倉統計三十天約八千多筆 | 一次補齊數千筆結算與八千多筆統計不失敗 | `P/contract_funding_rate_settlement_repository.go:27,44`；`P/contract_position_statistic_repository.go`（`CreateInBatches` 1000） | `Pt/…settlement_repository_test.go:133`；`Pt/…statistic_repository_test.go:115` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| ~~`MD/binance_contract_symbol_lookup_proxy.go:150-154`~~ | 第一輪：結算間隔清單問不到時，整個加入被拒 | **已解決**：程式改為照樣加入、先沒有規格，並由 BR-12 涵蓋 |
| `S/contract_trading_symbol_service.go:166-171`；`MD/…lookup_proxy.go:199-205` | **刷新時**，某個標的的規格讀不懂或不合規則（跳動、步進、最小量須 > 0 等），就被靜默略過、不留任何紀錄；該標的一直保留舊規格。加入名單時的同一情況已由 BR-12 涵蓋，刷新這一側 PRD 只寫了「不再列出／不答話保留」 | undocumented（範圍比第一輪小） |
| ~~`D/contract_funding_rate_settlement_domain.go:36-39`~~ | 第一輪：結算時間不得指向未來 | **已解決**：由 BR-10 涵蓋 |
| ~~`D/contract_position_statistic_domain.go:44,76-79`~~ | 第一輪：持倉價值與比值的規則；持倉價值被拒時訊息錯寫成持倉量 | **已解決**：訊息改為「持倉價值不得為負」，由 AC-68、AC-69 涵蓋 |
| ~~`S/…funding_rate_service.go:128-131`；`S/…statistic_service.go:121-124`~~ | 第一輪：查詢單次上限 | **已解決**：由 BR-9 涵蓋 |
| ~~`S/contract_trading_symbol_service.go:153-156`~~ | 第一輪：刷新只涵蓋已登錄的標的 | **已解決**：由 BR-11 涵蓋 |
| `P/k_candle_contract_repository.go:93-99` | 衝突時只要**任一組**為空，就把**兩組**一起以新值覆寫；已經有溢價指數、只缺指數價格的那根，它的溢價指數也會被改寫。domain 保證新寫入的兩組同在，所以目前走不到 | undocumented（未變） |
| `D/contract_position_statistic_window_domain.go:43-50` ＋ `J/contract_series_round_reporter.go:34-38` | 新的重問一小時帶來的副作用：某一刻一直缺一份的話，每五分鐘那一輪都會再跳過它、再寫一次跳過紀錄，最多重複約 12 次；超過一小時還沒到齊的那一刻，之後就不再被問 | undocumented（新增，影響小：只是紀錄重複，PRD 的「一小時」也隱含了之後不再問） |

Out of Scope 檢核（第二輪重做）：`5461c9c..HEAD` 沒有新增任何路由、寫入路徑或新的資料種類，仍然沒有 scope creep 違規。

## Summary

第三輪（補測試後）：

- Conforms: 81/84 clauses ✅（96.4%）；第二輪 77/84（91.7%），第一輪 65/75（86.7%）
- Violations / Mis-asserted / Gaps / Unclear: 無
- Partial: AC-58, AC-60, BR-7——**刻意保留**。四種資料各是一個獨立的 job、各自的 service 與 repository，一種失敗在結構上碰不到另一種（也碰不到現貨）；補一支測試只會把組裝重寫一遍，不會多驗到任何行為。
- Orphans: 3 項 undocumented（刷新時讀不懂的規格不留紀錄；只缺一組時兩組一起覆寫——目前走不到；一直不齊的那一筆每輪重記一次、最多約十二次），皆非 Out of Scope 違規。

### 第三輪補上的測試

| ID | 第二輪 | 第三輪 | 測試 |
|----|--------|--------|------|
| AC-18 / BR-8 | 🟡 | ✅ | `St/contract_k_candle_ingestion_service_test.go` `TestContractRoundDoesNotReachBackToAnOldCandleBeforeItsRecentMinutes`——斷言每分鐘那一輪的視窗起點晚於舊 K 線，且存下的沒有一根是它 |
| AC-66 / BR-8 | 🟡 | ✅ | `Pt/k_candle_contract_repository_test.go` `TestKCandleContractRepositoryReplacesAnOldCandleInsideTheRecentMinutesWithTheCompleteOne` |
| BR-11 | 🟡 | ✅ | `St/contract_trading_symbol_service_test.go` `TestContractTradingSymbolServiceListsRegisteredAndHeldTogetherOnce` 斷言只有 K 線的標的沒有規格 |

PRD 措辭另外收緊兩處：AC-44 的 Then 明寫從 08:10 問起；NFR-2 註明持倉統計刻意重問最近一小時。

---

## 第二輪紀錄


第二輪（修正後重驗）：

- Conforms: 77/84 clauses ✅（91.7%）；第一輪為 65/75（86.7%）
- Violations: 無（第一輪的 AC-18、AC-34、BR-1 都已處理：AC-34 改了程式；AC-18 與 BR-1 是 PRD 更正成既有、刻意設計的行為）
- Mis-asserted: 無（第一輪的 AC-59 已補測試）
- Partial: AC-18, AC-66, AC-58, AC-60, BR-7, BR-8, BR-11（程式正確，但沒有測試斷言 oracle）
- Gaps: 無
- Unclear: 無
- Orphans: 2 項 undocumented 仍在、1 項新增（刪除線的 5 列已解決）

### 已解決（第一輪 → 第二輪）

| ID | 第一輪 | 第二輪 | 怎麼解決的 |
|----|--------|--------|------------|
| AC-34 | 🔴 | ✅ | 視窗改為從最後一筆往前一小時；新測試 `St/…statistic_service_test.go:321` |
| AC-18 | 🔴 | 🟡 | PRD 改寫成既有行為；程式符合新的 oracle，但沒有測試用舊 K 線驗證 |
| BR-1 | 🔴 | ✅ | PRD 補上「價量／持倉量決定那一刻存不存在」 |
| AC-59 | 🟠 | ✅ | `At/contract_watchlist_removal_application_test.go:18` |
| AC-17 | 🟡 | ✅ | `St/…ingestion_service_test.go:833` |
| AC-65 | 🟡 | ✅ | `Ct/contract_controllers_test.go:221` |
| NFR-1 | 🟡 | ✅ | `cmd/server/venue_pacers_test.go:17` |

### 剩下要補的（全部是補測試，程式不必改）

1. **AC-66 / BR-8**：repository 測試先用 `Save` 存一根缺兩組的舊 K 線，再用 `Save` 寫入完整的一根，斷言兩組新價格都有值。
2. **AC-18 / BR-8**：service 或 repository 層放一根比每分鐘視窗更早的舊 K 線，跑一輪每分鐘抓取之後，斷言它仍然沒有兩組新價格。或者直接斷言傳給 `FetchKCandles` 的視窗起點晚於那根舊 K 線。
3. **AC-58 / BR-7**：讓資金費率那一輪對 BTC 失敗，同時跑持倉統計（與合約 K 線）那一輪，斷言 BTC 照常存入。
4. **AC-60**：合約某一種資料失敗時，現貨 BTCUSDT 照常抓取。
5. **BR-11**：在清單測試裡斷言只有 K 線的 1000PEPEUSDT，交易規格為空。
6. （可選，改 PRD 文字）**AC-44**：講清楚「前一小時」含不含 08:05 那一格。**NFR-2**：補一句持倉統計重問最近一小時是刻意的例外。
