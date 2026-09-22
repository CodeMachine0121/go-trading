# Contract Traceability Matrix — 合約 K 線資料抓取

Contract: `PRD.md`
Design map: `ARCH.md`（Traceability 42/42，本次逐條重新獨立判定）
Implementation: `internal/`（見各列 `file:line`）
Oracle: Acceptance Criteria — **61 clauses**（AC 44 · BR 12 · NFR 5）

> **審計天花板**：這是一次**靜態**符合性稽核。它拿 PRD 的預期結果分別對照「測試斷言什麼」與
> 「程式碼產出什麼」，**不以整套測試綠燈作為判準**，也不自行撰寫或執行新的探針。
> 每一條的 Spec-expected 都在讀任何程式碼之前，只從 PRD 文字推導出來。

## Clauses

### US-01 自動把合約 K 線抓回來

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 兩份資料都齊,存成一根完整的合約 K 線 | 那一分鐘的合約 K 線被存下來,帶著標記價格與成交筆數 | `contract_k_candle_ingestion_service.go:387` | `contract_k_candle_ingestion_service_test.go:TestContractRoundStoresTheCandlesTheVenueAnsweredWith` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 標記價格獨缺那一根,那一根整根不存 | 那一根不被存入並留下紀錄說明缺的是標記價格；同一批其他兩份都齊的照常存入 | `k_candle_contract_domain.go:85`＋`contract_k_candle_ingestion_service.go:455` | `…TestContractRoundSkipsTheMinuteWhoseMarkPriceNeverArrived` | asserts-oracle（斷言 stored=1/skipped=1/openTime/理由含「標記價格不得留白」） | produces-oracle | ✅ conforms |
| AC-3 | 標記價格整份問不到,這一輪一根都不存 | 該標的這一輪一根都不存並留下失敗紀錄；下一輪自然重試 | `binance_contract_market_data_proxy.go:76`＋`…service.go:404` | `…TestContractRoundStoresNothingWhenTheVenueWillNotAnswer`；重試由 `…TestTheContractJobWritesDownWhatWentWrongWithoutStopping` 佐證 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 標記價格附帶的零是佔位,不是成交量 | 存下來那一根的成交量取自價量那一份,不是零；標記那份只有四個價格被採用 | `binance_contract_wire.go:64` | `binance_contract_market_data_proxy_test.go:TestContractProxyDropsThePlaceholderZerosOnTheMarkPriceAnswer` | asserts-oracle（逐項斷言 55.714 / 4767877.86870 / 34.469 / 2949728.83040） | produces-oracle | ✅ conforms |
| AC-5 | 進行中那一根兩份都不存 | 最新那一根不被存入；存下來最新的是前一根已經走完的 | `contract_k_candle_ingestion_service.go:468` | `…TestContractRoundDoesNotStoreTheMinuteStillRunning` | asserts-oracle（斷言實際存入的 openTime 清單） | produces-oracle | ✅ conforms |
| AC-6 | 永續全天候,凌晨照抓 | 照常抓取,不被當成休市 | `contract_k_candle_ingestion_service.go`（刻意未注入休市帳本） | `…TestContractRoundKeepsFetchingThroughTheNightAndNeverPresumesAHoliday` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 來源回覆卻沒東西,不推定休市 | 這一輪什麼都不存但不推定當日休市；下一輪照常再抓一次 | 同上 | 同上（斷言連續兩輪都抓、第二輪無失敗原因） | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 合約與現貨並存而互不干擾

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-8 | 同代號同起始時間,兩邊各存一根 | 兩根同時存在；現貨那根收盤價仍然是 100 | `k_candle_contract.go:29`（獨立表與獨立唯一索引） | `k_candle_contract_repository_test.go:TestKCandleContractRepositoryKeepsContractAndSpotCandlesApart` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 查現貨只拿得到現貨 | 回的是現貨那根,收盤價 100；合約那根不會出現在結果裡 | 同上（兩張表天然隔離） | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 只有合約才有的代號加得進去 | 它被加進去並開始被抓；現貨不存在不影響 | `contract_trading_symbol_service.go:102` | `contract_trading_symbol_service_test.go:TestContractTradingSymbolServiceAddsAListedContract`；端到端另以真實 `1000PEPEUSDT` 驗證 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 名字相近的兩個代號是兩個毫不相干的標的 | 分別存放；不做任何換算、不建立任何關聯 | 設計中無任何換算元件 | `…TestKCandleContractRepositoryTreatsLookAlikeSymbolsAsUnrelated`；`binance_contract_symbol_lookup_proxy_test.go:TestContractSymbolLookupDoesNotTreatALookAlikeNameAsAMatch` | asserts-oracle（正向斷言分別存放＋負向斷言不誤配） | produces-oracle | ✅ conforms |

### US-03 管理合約追蹤名單

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-12 | 同一個代號兩邊各自追蹤 | 兩邊都在追,各抓各的存成各自的 K 線 | `contract_trading_symbol.go:26`（獨立表） | `contract_trading_symbol_repository_test.go:TestContractTradingSymbolRepositoryHoldsTheSameNameAsTheSpotList` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 加進去就立刻回補那一檔 | 登錄並標記為追蹤中；系統立刻回補那一檔；下一輪起照常抓 | `contract_trading_symbol_application.go:53` | `contract_k_candle_application_test.go:TestContractApplicationCatchesAContractUpTheMomentItIsAdded` | asserts-oracle（斷言 Save 帶 IsWatched=true，且 FetchKCandles 恰好一次） | produces-oracle | ✅ conforms |
| AC-14 | 移除只停止追蹤,一根都不刪 | 合約不再抓；現貨照常抓；兩邊已存下的 K 線一根都沒有被刪掉 | `contract_trading_symbol_service.go:135` | `…TestContractTradingSymbolServiceStopsFollowingWithoutForgettingTheContract`（「一根都不刪」以 gomock 未宣告 Delete 保證）＋`contract_trading_symbol_repository_test.go:TestContractTradingSymbolRepositoryStoresStoppingToFollow` | asserts-oracle ⚠️ 見備註 A | produces-oracle | ✅ conforms |
| AC-15 | 合約不認得的代號加不進去 | 加入被拒絕並說明找不到這個代號 | `contract_trading_symbol_service.go:117` | `…TestContractTradingSymbolServiceRefusesAContractTheVenueDoesNotList`（另斷言 Save 未被呼叫） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15a | 已經停止交易的合約加不進去 | 加入被拒絕並說明找不到這個代號；那個代號沒有被登錄 | `binance_contract_symbol_lookup_proxy.go:107`（`status == TRADING`） | `binance_contract_symbol_lookup_proxy_test.go:TestContractSymbolLookupRefusesAContractNobodyCouldFollow/已經停止交易的合約` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15b | 有交割日的合約加不進去 | 同上 | `binance_contract_symbol_lookup_proxy.go:108`（永續種類允許清單） | `…/有交割日的季度合約`＋`…TestContractSymbolLookupRefusesAKindItDoesNotRecognise`＋`…TestContractSymbolLookupAcceptsAPerpetualOnATraditionalFinanceUnderlying` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 來源不可用時不先加了再說 | 加入被拒絕並說明稍後再試；那個代號沒有被登錄 | `contract_trading_symbol_service.go:112` | `…TestContractTradingSymbolServiceAddsNothingWhenTheVenueCannotBeReached` | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 查得到合約 K 線

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-17 | 讀一段合約 K 線 | 依起始時間由早到晚回給我；每一根都帶著標記價格與成交筆數 | `k_candle_contract_repository.go:146`（排序）＋`k_candle_contract_service.go:65`（形狀） | 排序：`…TestKCandleContractRepositoryReadsARangeEarliestFirst`；欄位：`k_candle_contract_service_test.go:TestKCandleContractServiceReadsARangeAndRefusesOneTooWide` | asserts-oracle ⚠️ 見備註 B | produces-oracle | ✅ conforms |
| AC-18 | 那段沒有資料就是沒有,不是錯誤 | 回的是空的結果,不算錯誤 | `k_candle_contract_service.go:65` | `…TestKCandleContractServiceAnswersAnEmptyRangeWithNothingRatherThanAFailure` | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 指名一段合約歷史

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-19 | 先拿到一筆輪次,不等抓完 | 立刻回一筆同步輪次,不等抓完；拿編號能回來看進度 | `contract_k_candle_ingestion_service.go:148` | `…TestContractHistorySyncAnswersBeforeItHasFetchedAnything`（把來源擋在門口後仍取得 running 的輪次） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 已經齊全的那幾天連問都不問 | 那一天完全不向來源詢問；報告裡那一天新增零根 | `contract_k_candle_ingestion_service.go:238` | `…TestContractHistorySyncDoesNotAskAboutADayItAlreadyHoldsWhole`（FetchKCandles `.Times(0)`＋StoredCount 0） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 只存了一半的那天,兩份資料都重問 | 兩份都重新詢問；只有系統沒有的那幾根被存進去；已經有的一根都沒有被寫過 | `…service.go:238`＋`k_candle_contract_repository.go:63`（DoNothing） | `…TestContractHistorySyncAsksAgainAboutADayItOnlyHoldsHalfOf`＋`…TestKCandleContractRepositorySaveAllIfAbsentLeavesHeldCandlesUntouched`＋`…TestContractProxyMergesTheTwoAnswersIntoOneCandle` | asserts-oracle ⚠️ 見備註 B | produces-oracle | ✅ conforms |
| AC-22 | 同一個代號同時只跑一趟 | 這一次被拒絕並說明已經有一趟在跑；其他代號不受影響 | `k_candle_contract_history_sync_run_repository.go:52`＋`schema_migrator.go`（部分唯一索引） | `…TestContractHistorySyncRefusesASecondRunOnTheSameContract`＋`contract_trading_symbol_repository_test.go:TestContractHistorySyncRunRepositoryRefusesASecondRunOnTheSameSymbol`（另斷言別的代號成功） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 回溯天數說不通就拒絕,超過上限要說出上限 | 這一次被拒絕；超過上限時訊息說出上限是多少 | `contract_k_candle_ingestion_service.go:137`（`KCandleHistoryLookbackDomain`） | `…TestContractHistorySyncRefusesALookbackThatIsNotAStretch`（斷言訊息含 `3650`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 起點比這個合約存在的時間還早 | 來源有幾根就存幾根；那一趟不算失敗 | `contract_k_candle_ingestion_service.go:246` | `…TestContractHistorySyncSucceedsWhenTheContractDidNotYetExist` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 整段期間這個合約還沒上市 | 一根都沒有被存下來；那一趟仍然算走完並說明來源沒有那段資料 | `…service.go:246`＋`contract_k_candle_history_sync_runner.go:100` | `…TestContractHistorySyncSucceedsWhenTheContractDidNotYetExist`＋`…TestContractHistorySyncWritesWhatTheVenueRefusedWithoutFailingTheRun`（斷言 succeeded ＋ fetchFailureReason 有值、failureReason 空） | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 手動放一根合約 K 線進去

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-26 | 給齊了就存進去 | 它被存下來 | `k_candle_contract_domain.go:31` | `k_candle_contract_domain_test.go:TestNewKCandleContractDomainStoresEveryFigureWhenAllAreGiven`（逐欄斷言）；HTTP 端 `contract_controllers_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 同代號同起始時間即覆蓋 | 收盤價變成 120；那個代號那個時間仍然只有一根 | `k_candle_contract_repository.go:41` | `…TestKCandleContractRepositoryOverwritesTheSameSymbolAndOpenTime`（斷言 count=1 且值為後寫入） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 沒被追蹤的代號也塞得進去 | 它被存下來；系統不要求這個代號先被追蹤 | `k_candle_contract_service.go:44`（未注入追蹤名單） | `k_candle_contract_service_test.go:TestKCandleContractServiceStoresACandleWithoutConsultingTheWatchlist` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 沒給標記價格就不是一根合約 K 線 | 新增被拒絕,並說明合約 K 線必須帶標記價格 | `k_candle_contract_domain.go:85` | `…TestNewKCandleContractDomainRejectsAMissingFigure/沒給標記價格…`＋HTTP 400 斷言 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 沒給成交筆數也拒絕,留白不能當成零 | 新增被拒絕,並說明成交筆數必填 | `k_candle_contract_domain.go:72` | 同上表列＋HTTP 400 斷言 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 全部合規就收下 | 它被存下來 | `k_candle_contract_domain.go:31` | `…TestNewKCandleContractDomainStoresEveryFigureWhenAllAreGiven` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | 標記價格自己的高低關係要成立 | 新增被拒絕,並說明標記的最高價不得低於最低價 | `k_candle_contract_domain.go:101` | `…TestNewKCandleContractDomainRejectsAnUnlawfulFigure/標記的最高價低於最低價` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | 標記價格不得為負 | 新增被拒絕,並說明標記價格不得為負 | `k_candle_contract_domain.go:90` | 同上表列 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | 成交筆數不得為負 | 新增被拒絕,並說明成交筆數不得為負 | `k_candle_contract_domain.go:77` | 同上表列 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-35 | 成交筆數為零是合法的 | 它被存下來 | `k_candle_contract_domain.go:77`（只擋負數） | `…TestNewKCandleContractDomainAcceptsAZeroTradeCount`（另斷言存下來的值是 0） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 起始時間說不通就拒絕 | 新增被拒絕,並說明是哪一條不合 | `k_candle_contract_domain.go:45`（委派共用 K 線規則） | `…TestNewKCandleContractDomainRejectsAnUnlawfulFigure/起始時間指向未來｜不在一分鐘刻度上` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-37 | 最新價與標記價格差很遠照收 | 它被存下來 | `k_candle_contract_domain.go`（刻意無跨組比較） | `…TestNewKCandleContractDomainAcceptsAMarkPriceFarFromTheLastPrice`（100 vs 100000） | asserts-oracle | produces-oracle | ✅ conforms |

### US-07 改掉或刪掉一根合約 K 線

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-38 | 改掉那一根的數字 | 收盤價變成 120,標記價格一併換成新的 | `k_candle_contract_repository.go:105`（含 mark_* 欄位） | `…TestKCandleContractRepositoryUpdatesTheFiguresOfAHeldCandle`（斷言 close/markClose/tradeCount 三者皆換） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | 改的是哪一根由指名決定 | 修改被拒絕,並說明改的對象由指名決定 | `k_candle_contract_controller.go:127` | `contract_controllers_test.go:「refuses a body naming another candle」`（代號與起始時間兩側都測） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-40 | 改一根不存在的 | 系統回覆找不到那一根 | `k_candle_contract_repository.go:117` | `…TestKCandleContractRepositoryReportsNotFound`＋HTTP 404 斷言 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-41 | 刪掉合約那根,現貨那根不受影響 | 合約那一根不見了；現貨那根仍然在,數字一個字都沒變 | `k_candle_contract_repository.go:208` | `…TestKCandleContractRepositoryDeleteLeavesTheSpotCandleAlone` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-42 | 刪一根不存在的 | 系統回覆找不到那一根 | `k_candle_contract_repository.go:216` | `…TestKCandleContractRepositoryReportsNotFound`＋HTTP 404 斷言 | asserts-oracle | produces-oracle | ✅ conforms |

### Section 4 — Core Business Rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | 一根合約 K 線由兩份資料合成,缺一不存 | 缺的那一根逐根跳過並留下紀錄,同一批其餘照常存入 | `binance_contract_market_data_proxy.go:60`＋`k_candle_contract_domain.go:85` | AC-2 之測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 標記價格那一份附帶的量是佔位的零 | 只採用四個價格；量一律丟棄 | `binance_contract_wire.go:64` | AC-4 之測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 每一項都不得留白 | 任一項留白即拒絕 | `k_candle_contract_domain.go:62-95` | `…TestNewKCandleContractDomainRejectsAMissingFigure`（五項全測：標記價格／成交筆數／成交額／主動買入量／主動買入額） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 合約與現貨互不覆蓋 | 同代號同起始時間兩邊各存一根 | `k_candle_contract.go:29` | AC-8 之測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 兩邊的代號宇宙不重疊也不對應 | 不做任何換算或關聯 | 無換算元件 | AC-11 之測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 兩份追蹤名單各自獨立 | 加入、移除、抓取彼此不影響 | `contract_trading_symbol.go:26` | AC-12 之測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6a | 「在清單上」不等於「跟得了」 | 已停止交易、還沒開始、有交割日、認不得的種類，一律拒絕 | `binance_contract_symbol_lookup_proxy.go:100-112` | AC-15a／AC-15b 之測試（四種形狀全測，含允許清單外的種類） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 永續全天候,永不推定休市 | 來源正常回覆卻沒東西只是這一次沒東西 | 未注入休市帳本 | AC-6／AC-7 之測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 標記價格與最新價各自成立 | 各自高低要對,兩者之間不比大小 | `k_candle_contract_domain.go:101` | AC-32＋AC-37 之測試（兩側都測） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 合約的回補上限與回溯天數上限各自一份設定 | 合約用自己的數字,不沿用現貨那組 | `application_config.go:87`（`ContractIngestionConfig`）＋`dependencies.go` | `contract_ingestion_config_test.go:TestTheTwoVenuesAreSettledApart`（八項全設成非預設值後，斷言合約端全部生效、現貨端一項都沒被波及） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 合約的同步輪次編號自己一串 | 與現貨那串互不相干 | `k_candle_contract_history_sync_run.go:21`（獨立表） | `…TestContractHistorySyncRunRepositoryIsUnaffectedByASpotRunOnTheSameSymbol` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 「這一天齊不齊」只要數合約 K 線有幾根就夠了 | 只做一次計數即可判定,不必分別數兩份 | `contract_k_candle_ingestion_service.go:232`（只呼叫 `CountInRange`） | AC-20 之測試（mock 只設 `CountInRange`，未設任何第二份計數） | asserts-oracle | produces-oracle | ✅ conforms |

### Section 6 — Non-Functional Requirements

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-1 | 抓得動長歷史（一天一段、已齊全的不問、照來源節奏打） | 長區間跑得完 | `k_candle_ingestion_domain.go:196`（`HistoryChunks`，重用）＋`request_pacer.go` | `…TestContractHistorySyncAsksForTheWholeStretchTheCallerNamedOneDayAtATime`（斷言各段首尾相接、合起來剛好涵蓋所求區間、中間不漏任何一分鐘）＋AC-20（已齊全的不問）＋`binance_market_data_proxy_test.go:TestEveryProxyReachingOneVenueSharesItsPace`（節奏） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 不佔住呼叫端（歷史同步先回輪次不等抓完） | 抓取未完成時呼叫已經返回 | `contract_k_candle_ingestion_service.go:148` | `…TestContractHistorySyncAnswersBeforeItHasFetchedAnything` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 來源額度不得被打爆；合約與現貨分開計算 | 合約有自己的取用節奏,不與現貨共用 | `dependencies.go:venuePacers.cryptoContract` | `cmd/server/venue_pacers_test.go:TestEachVenueIsPacedOnItsOwnAllowance`（斷言三個場所握的**不是同一份** limiter）＋`…TestEachVenuesPaceComesFromItsOwnSetting` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 記憶體是平的（不隨區間長度成長） | 同時在手上的資料量與回溯長度無關 | `contract_k_candle_ingestion_service.go:217`（逐段抓、逐段存） | **無測試**（結構性性質） | no-test | produces-oracle | 🟡 partial |
| NFR-5 | 安全性 N/A | 本切片不涉及身分與權限 | 無任何驗證接線 | — | — | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `k_candle_symbol_ingestion_report_domain.go`（全檔） | 把「抓取報告怎麼累加」的規則從兩支 service 抽成一個 Domain Model；**行為與抽取前一字不差**，並非新行為 | 良性 orphan（重構產物，非契約行為） |
| `binance_contract_market_data_proxy.go:135`（`fetchRows` 分頁） | 視窗寬於一頁時持續向來源要下一頁 | 良性 orphan（PRD 未提分頁，屬來源細節；由 `TestContractProxyWalksAWindowWiderThanOnePage` 涵蓋） |

**Out of Scope 反向檢查（機械驗證）**：PRD 明文排除的八項——即時跟盤、彙總刻度、資金費率、
未平倉量、指標計算／回測／交易策略／機器人、現貨既有行為、幣本位合約／選擇權、身分驗證——
在合約程式碼中**全部找不到對應實作**；`backtest_service.go`／`indicator_calculation_service.go`／
`trading_strategy_service.go`／`strategy_bot_service.go` 完全沒有出現 `Contract` 字樣。
現貨側唯一的變動是 `k_candle_ingestion_service.go` 改用共用的報告模型（-67/+24 行，全部落在報告累加）。
**無越界。**

## 備註

- **備註 A（AC-14）**：三個部分中，「合約不再抓」與「一根都不刪」由 service 測試斷言（後者以
  gomock 未宣告 `Delete` 保證任何刪除都會使測試失敗）；「現貨照常抓」則由
  `TestContractTradingSymbolRepositoryHoldsTheSameNameAsTheSpotList` 間接斷言。
  **沒有單一測試涵蓋整條 clause**，但每一部分都有斷言。
- **備註 B（AC-17、AC-21）**：同樣是一條 clause 的各部分散在不同層的測試裡（排序在 repository、
  欄位在 service、合成在 proxy）。判定為 conforms，但若日後要改動其中一層，
  需注意這條 clause 的保護網不在同一處。

## Summary

### 第一輪（修正前）

- Conforms: 54 / 58（93.1%）· Violations 0 · Mis-asserted 1（`NFR-1`）· Partial 3（`BR-9`、`NFR-3`、`NFR-4`）· Gaps 0 · Unclear 0

### 第二輪（依第一輪回饋修正後）

- **Conforms: 60 / 61 clauses ✅（98.4%）**
- Violations: **無**
- Mis-asserted: **無** —— `NFR-1` 已補上直接斷言切段邊界的測試
- Partial: `NFR-4`（記憶體不隨區間長度成長）。**刻意不補**：它是逐段抓、逐段存這個結構本身的性質，
  要驗證得寫成效能量測而不是行為斷言，而那會是一條依機器狀態而定、遲早會誤報的測試。
  程式碼路徑（`contract_k_candle_ingestion_service.go:217`，一段抓完存完才進下一段）由
  `NFR-1` 的切段測試間接佐證。
- Gaps: **無**
- Unclear: **無**
- Orphans: 2（皆良性，非越界）

### 第三輪（code review 回饋後）

外部 code review 提出兩點，**兩點都以真實的幣安回應實測確認為真**，兩點都已處理：

**1. 代號確認只比對名字（medium）** —— 合約目錄同時帶著 `status` 與 `contractType`，而原本的
比對只看名字。實測 905 筆裡有 **131 筆不可交易**（130 `SETTLING`、1 `PENDING_TRADING`）與
**4 筆季度交割合約**，全都加得進一份主題是「永續合約」的名單。跟進去的下場是：每分鐘抓一次、
永遠存不到東西，而因為這一刀刻意決定「永不推定休市」，它看起來跟市場很安靜**完全一樣**。
已加上兩道（可交易、是永續），並把 `TRADIFI_PERPETUAL`（黃金、白銀、個股的永續）明確納入——
它們的交割日是 2100 年的哨兵值，是真的永續。認不得的種類一律拒絕，不預設它是永續。
→ 新增 `AC-15a`、`AC-15b`、`BR-6a`。

**2. 「這一天齊不齊」的理由寫錯了（low/medium）** —— `IKCandleContractRepository.CountInRange`
的註解寫著「兩條來源序列要嘛都到、要嘛都不到」，**而 proxy 並不提供這個保證**。實測：
BTCUSDT 永續的價量回得到 2019-09，標記價格要到 **2019-12-31** 才開始有——中間三個多月，
每一分鐘都有價量、沒有標記價格，因此永遠存不進來，那幾天也就永遠湊不滿，捷徑永遠不生效。
代價是請求數而非正確性。**不成立的那句保證已改寫**，限制寫進介面與使用現場，行為以
`TestContractHistorySyncStoresNothingForAStretchWithNoMarkPriceAtAll` 釘住，
實際可用歷史起點寫進 README。
**沒有補「記住哪些分鐘填不了」的機制**——那需要一份這個系統還沒有的負向紀錄，是下一刀的事。

### 修正過程中另外發現的一件事（不在 clause 之列）

新增的合約 proxy 測試讓 `marketdata` 這個 package 變忙之後，**既有的兩條節奏測試開始間歇失敗**
（main 分支 12 次 0 失敗、本分支 12 次 2 失敗）。原因不是節奏壞了，而是那兩條測試把時間戳記
取在 **server 端 handler 進入的時刻**，每一段間隔因此夾著一次 HTTP 往返的抖動，而它們只留了
2 毫秒餘裕。已放寬到 15 毫秒並把理由寫進註解；放寬後分別以「拿掉節奏」與「換成兩份獨立 pacer」
兩個突變確認守衛仍然有效，整包連跑 120 次 0 失敗。
