# Contract Traceability Matrix — 策略回測

Contract: `PRD.md`（同一資料夾）
Design map: `ARCH.md`（同一資料夾）
Implementation: `internal/domain/models/domains/backtest_*.go`、`internal/domain/models/domains/signal_domain.go`、`internal/domain/models/domains/position_sizing_domain.go`、`internal/domain/service/backtest_service.go`、`internal/infrastructure/script/yaegi_indicator_script_proxy.go`、`internal/controller/backtest_controller.go`
Oracle: Acceptance Criteria — 42 個 Gherkin scenario、15 條 Core Business Rules、6 條 Non-Functional Requirements（共 63 clauses）

> **Ceiling.** 這是一份**靜態一致性稽核**：它拿 PRD 的預期結果去讀測試斷言與程式路徑，
> 不撰寫新的探針、也不執行自己發明的情境。判定來自「與 oracle 比對」，不是來自跑測試看綠燈。

---

## Clauses — US-01 信號的讀法

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 信號是 `1` → 買入 | 這一棒的意見是買入 | `signal_domain.go:48` | `signal_domain_test.go:19`（one means buy） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 信號是 `-1` → 賣出 | 這一棒的意見是賣出 | `signal_domain.go:51` | `signal_domain_test.go:19`（minus one means sell） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 信號是 `0` → 持平、倉位不動 | 意見是持平；倉位不動 | `signal_domain.go:55` | `signal_domain_test.go:19`（zero means flat）＋`backtest_simulation_domain_test.go:137` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 結果裡沒有「信號」→ 持平，與 `0` 一字不差 | 意見是持平，與信號為 0 的結果相同 | `signal_domain.go:39` | `signal_domain_test.go:19`（a result without the signal name） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 從沒放過信號的舊算式 → 零交易、算式不必改 | 開倉次數 0、已平倉 0 筆 | `signal_domain.go:39` | `backtest_simulation_domain_test.go:146` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 信號是 `0.5` → 買入 | 意見是買入 | `signal_domain.go:48` | `signal_domain_test.go:19`（any positive number） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 信號是 `-2` → 賣出 | 意見是賣出 | `signal_domain.go:51` | `signal_domain_test.go:19`（any negative number） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 信號不是有限數字 → 持平，重演不中斷 | 意見是持平；重演繼續 | `signal_domain.go:44` | `signal_domain_test.go:19`（NaN／Inf 兩列） | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — US-02 一個倉位與反手

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-9 | 空手 + 買入 → 以該棒收盤價開多倉 | 多倉開在該棒收盤價 | `backtest_account_domain.go:76` | `backtest_simulation_domain_test.go:63` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 空手 + 賣出 → 以該棒收盤價開空倉 | 空倉開在該棒收盤價 | `backtest_account_domain.go:76` | `backtest_simulation_domain_test.go:75` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 已是多倉 + 買入 → 倉位不動、開倉次數不增加 | 倉位原封不動；開倉次數不變 | `backtest_account_domain.go:53` | `backtest_simulation_domain_test.go:86`＋`backtest_account_domain_test.go:46` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 已是空倉 + 賣出 → 同上 | 倉位原封不動；開倉次數不變 | `backtest_account_domain.go:53` | `backtest_simulation_domain_test.go:98` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 空倉 + 買入（收盤 100）→ 以 100 平空、同一棒以 100 開多 | 留下一筆出場價 100 的已平倉交易；多倉進場價亦為 100 | `backtest_account_domain.go:52-85` | `backtest_simulation_domain_test.go:107`＋`backtest_account_domain_test.go:58` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 多倉 + 賣出（收盤 100）→ 對稱 | 同上，方向相反 | `backtest_account_domain.go:52-85` | `backtest_simulation_domain_test.go:126` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 空手 + 持平 → 什麼都不發生 | 沒有倉位被開、沒有交易被記下 | `backtest_account_domain.go:47` | `backtest_simulation_domain_test.go:137`＋`backtest_account_domain_test.go:37` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15b | 收盤價為 0 + 買入 → 跳過這次開倉，重演不中斷、不計次 | 手上仍無倉位；重演繼續；開倉次數不變 | `backtest_position_domain.go:35`＋`backtest_account_domain.go:78` | `backtest_account_domain_test.go:84`＋`backtest_position_domain_test.go`（a price of zero opens nothing） | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — US-03 倉位大小模式

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-16 | 可用 10,000、全押 → 押 10,000 | 押注金額 10,000 | `position_sizing_domain.go:110` | `position_sizing_domain_test.go:104`（all in） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 可用 10,000、百分比 50 → 押 5,000 | 押注金額 5,000 | `position_sizing_domain.go:112` | `position_sizing_domain_test.go:104`（a percentage）＋`backtest_simulation_domain_test.go:165` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 百分比 100 → 押 10,000，等同全押 | 押注金額 10,000 | `position_sizing_domain.go:112` | `position_sizing_domain_test.go:104`（a hundred percent） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 可用 10,000、固定 3,000 → 押 3,000，剩 7,000 不動 | 押 3,000；其餘不動 | `position_sizing_domain.go:106` | `position_sizing_domain_test.go:104`＋`backtest_simulation_domain_test.go:175` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 可用 2,000、固定 3,000 → 跳過這次開倉，不失敗、不中斷、不計次 | 手上仍無倉位；重演繼續；開倉次數不變 | `position_sizing_domain.go:107`＋`backtest_account_domain.go:70` | `backtest_simulation_domain_test.go:186`＋`backtest_account_domain_test.go:72` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 百分比 0 → 整次拒絕，說明範圍 | 整次拒絕；訊息提到百分比範圍 | `position_sizing_domain.go:78` | `position_sizing_domain_test.go:13`＋`backtest_controller_test.go` 同族 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 百分比 150 → 整次拒絕 | 整次拒絕 | `position_sizing_domain.go:78` | `position_sizing_domain_test.go:13` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 固定金額 0 或負數 → 整次拒絕 | 整次拒絕 | `position_sizing_domain.go:83` | `position_sizing_domain_test.go:13`（兩列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 固定金額 30,000 > 初始資金 10,000 → 接受，每次開倉都被跳過、開倉次數 0 | 正常執行；開倉次數 0 | `position_sizing_domain.go:83`（無上限檢查） | `position_sizing_domain_test.go:13`＋`backtest_simulation_domain_test.go:198` | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — US-04 資金曲線

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-25 | 10,000 起、前三棒持平 → 三個點都是 10,000 | 三點皆 10,000 | `backtest_equity_curve_domain.go:40` | `backtest_simulation_domain_test.go:209` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 多倉進場 100、本棒收 110 → 該點反映以 110 估的未實現獲利 | 該點為 11,000（含未實現） | `backtest_account_domain.go:88`＋`backtest_position_domain.go:75` | `backtest_simulation_domain_test.go:221` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | N 根 K 線 → 曲線 N 個點，各帶該棒起始時間 | 點數＝根數；每點帶該棒 openTime | `backtest_simulation_domain.go:73` | `backtest_simulation_domain_test.go:232` | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — US-05 成績單

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-28 | 10,000 → 12,500 → 總報酬率 25% | 0.25 | `backtest_equity_curve_domain.go:89` | `backtest_simulation_domain_test.go:247`＋`backtest_equity_curve_domain_test.go:69` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 曲線 10,000→12,000→9,000→11,000 → 最大回撤 25% | 0.25 | `backtest_equity_curve_domain.go:59` | `backtest_simulation_domain_test.go:256`＋`backtest_equity_curve_domain_test.go`（表格首列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 每點都低於初始資金、最低 9,000 → 最大回撤 10% | 0.10（初始資金是第一個高點） | `backtest_equity_curve_domain.go:33` | `backtest_equity_curve_domain_test.go`（the starting capital is the first peak） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 四筆已平倉、三筆賺 → 勝率 75% | 0.75 | `backtest_account_domain.go:124` | `backtest_simulation_domain_test.go:282` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | 一筆都沒平倉 → 勝率**不適用**，不是 0% | 勝率沒有值（JSON `null`） | `backtest_account_domain.go:125`＋`backtest_simulation_domain.go` 摘要組裝 | `backtest_simulation_domain_test.go:307`、`backtest_account_domain_test.go:95`、`backtest_controller_test.go:135`（斷言 `"winRate":null`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | 兩筆、一筆賺一筆同價進出 → 勝率 50% | 0.5（打平不算贏） | `closed_trade_vo.go` `IsWin`（`Profit.IsPositive()`） | `backtest_simulation_domain_test.go:295`＋`backtest_position_domain_test.go`（breaking even is not a win） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | 開三次倉、兩次平倉、最後一個還開著 → 開倉次數 3、明細 2 筆、勝率只看那 2 筆 | 3／2／依 2 筆計 | `backtest_account_domain.go:83`＋`:102` | `backtest_simulation_domain_test.go:315` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-35 | 還開著的多倉進場 100、最後一棒收 120 → 「最後剩多少」含未實現 | 12,000（含未實現） | `backtest_account_domain.go:88` | `backtest_simulation_domain_test.go:325` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 每筆已平倉交易交代方向／進出場時間與價格／賺賠 | 六個欄位齊全且正確 | `backtest_position_domain.go:85` | `backtest_simulation_domain_test.go:336`＋`backtest_position_domain_test.go`（ClosedAt） | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — US-06 一次重演要交代哪些事

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-37 | 第 N 棒看得到第一棒到第 N 棒（含），愈往後愈長 | 第 3 棒看 3 根、第 5 棒看 5 根 | `yaegi_indicator_script_proxy.go` `ExecuteForEachCandle`（`kCandles[:candleCount]`） | `yaegi_indicator_script_per_candle_test.go:48`、`:60` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 進出場一律用該棒的收盤價 | 成交價＝該棒收盤價 | `backtest_simulation_domain.go`（`fillPrice` 唯一決定處） | `backtest_simulation_domain_test.go:63`、`:107`（進出場價皆等於該棒收盤） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | 還在走的刻度區間不被取用 | 該格不出現在重演的 K 線中 | `backtest_domain.go:83`＋`:171` | `backtest_domain_test.go:154`、`:215`、`:166` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-40 | 重演的結果不留存；再問一次就重算 | 沒有任何寫入；沒有舊結果被取出 | 無 repository 寫入路徑（`backtest_service.go` 只呼叫 `FindInRange`） | 全部 application／controller 測試（gomock 嚴格 mock：任何未預期的寫入呼叫即失敗） | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — US-07 重演不了的時候

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-41 | 剛好兩根 → 正常執行 | 不拒絕，交出成績單 | `backtest_domain.go:175` | `backtest_domain_test.go:233` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-42 | 只有一根 → 拒絕「湊不出足夠的 K 線」 | 整次拒絕；訊息說湊不出足夠的 K 線 | `backtest_domain.go:176` | `backtest_domain_test.go:247`＋`backtest_application_test.go:195`＋`backtest_controller_test.go:173`（400） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-43 | 一根都沒有 → 同一句拒絕 | 同上 | `backtest_domain.go:176` | `backtest_domain_test.go:259`＋`backtest_application_test.go:205` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-44 | 起訖顛倒 → 同一句拒絕 | 同上 | `backtest_domain.go:90` | `backtest_domain_test.go`（a stretch that ends before it starts）＋`backtest_application_test.go:215`（且不讀儲存） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-45 | 初始資金 0 → 拒絕「初始資金必須大於零」 | 整次拒絕；訊息提到初始資金 | `backtest_domain.go:57` | `backtest_domain_test.go`（starting capital of zero）＋`backtest_controller_test.go:156`（斷言訊息含「初始資金」） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-46 | 初始資金 −1 → 同上 | 同上 | `backtest_domain.go:57` | `backtest_domain_test.go`（negative starting capital） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-47 | 算式取用未宣告的參數名稱 → 拒絕並指出是哪個名字 | 整次拒絕；回應帶該參數名稱 | `yaegi_indicator_script_proxy.go` `runOver`（`UndeclaredParameter`）＋`backtest_controller.go`（400 + `parameterName`） | `yaegi_indicator_script_per_candle_test.go:156`＋`backtest_application_test.go:251`＋`backtest_controller_test.go:183` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-48 | 算式無法解讀／某一棒執行失敗 → 拒絕，說法與指標預覽一致 | 整次拒絕；與指標計算同一種錯誤形狀（422） | `yaegi_indicator_script_proxy.go` `prepare`／`runOver`＋`backtest_controller.go` | `yaegi_indicator_script_per_candle_test.go:112`、`:121`、`:131`＋`backtest_controller_test.go:196` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-49 | 刻度區間數超過單次查詢筆數上限 → 拒絕，說出要用幾根、上限幾根 | 整次拒絕；訊息含兩個數字 | `backtest_domain.go:94` | `backtest_domain_test.go`（a stretch needing more buckets than one read allows） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-50 | 交易標的空白 → 拒絕，說明不得為空 | 整次拒絕 | `backtest_domain.go`（`NewTradingSymbolDomain`） | `backtest_domain_test.go`（a missing symbol is refused） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-51 | 彙總刻度不在五種之內 → 拒絕 | 整次拒絕 | `backtest_domain.go`（`NewAggregationIntervalDomain`） | `backtest_domain_test.go`（an unrecognised aggregation interval） | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — Core Business Rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | 信號的讀法（正負號／缺名／非有限數） | 見 AC-1…AC-8 | `signal_domain.go` | `signal_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 一次重演的條件（八項） | 八項齊備且各自驗證 | `backtest_request_dto.go`＋`backtest_domain.go` | `backtest_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 只取已走完的刻度區間；終點指向未來視同現在 | 還在走的那格不取；未來終點＝現在 | `backtest_domain.go:83`（`effectiveEndTime`）＋`:171` | `backtest_domain_test.go:166`、`:215` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 算式每棒看到起點到當棒；**不另設看得到的根數上限** | 窗口逐棒變長；沒有第二個上限 | `yaegi_indicator_script_proxy.go`（無額外上限）＋`backtest_domain.go:94`（唯一上限） | `yaegi_indicator_script_per_candle_test.go:48` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 成交價＝該棒收盤價 | 見 AC-38 | `backtest_simulation_domain.go` | `backtest_simulation_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 倉位規則（單一倉位、忽略同向、反向反手） | 見 AC-9…AC-15 | `backtest_account_domain.go:44` | `backtest_account_domain_test.go`、`backtest_simulation_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6b | 押不下去的兩種情形：資金不夠，或該棒收盤價不大於零 | 兩者都是「這一次開倉沒有發生」，重演照常往下走 | `backtest_account_domain.go:70`＋`backtest_position_domain.go:35` | `backtest_account_domain_test.go:72`、`:84` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 押多少（三模式、資金不足跳過） | 見 AC-16…AC-20 | `position_sizing_domain.go:102` | `position_sizing_domain_test.go:104` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 押注參數合法範圍（百分比 (0,100]、固定 >0、允許 > 初始資金、初始資金 >0） | 見 AC-21…AC-24、AC-45 | `position_sizing_domain.go:74`＋`backtest_domain.go:57` | `position_sizing_domain_test.go:13` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 資金曲線一棒一點、含未平倉估值 | 見 AC-25…AC-27 | `backtest_simulation_domain.go`＋`backtest_equity_curve_domain.go:40` | `backtest_simulation_domain_test.go:209`… | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 總報酬率＝（最後剩多少 − 初始資金）÷ 初始資金；含未平倉 | 見 AC-28、AC-35 | `backtest_equity_curve_domain.go:89` | `backtest_equity_curve_domain_test.go:69`、`:83` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 最大回撤；初始資金算第一個高點 | 見 AC-29、AC-30 | `backtest_equity_curve_domain.go:33`、`:59` | `backtest_equity_curve_domain_test.go`（表格四列） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 勝率＞0 才算贏；一筆都沒平倉即不適用 | 見 AC-31…AC-33 | `backtest_account_domain.go:124`＋`closed_trade_vo.go` | `backtest_account_domain_test.go:95`、`:104` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-13 | 開倉次數 vs 交易明細 | 見 AC-34 | `backtest_account_domain.go:83`、`:102` | `backtest_simulation_domain_test.go:315` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-14 | 不留存 | 見 AC-40 | 無寫入路徑 | 嚴格 mock（見 AC-40） | asserts-oracle | produces-oracle | ✅ conforms |

## Clauses — Non-Functional Requirements

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-1 | 算式只被解讀一次，之後每一棒重複執行 | 同一份解讀跨棒沿用（跨棒可見的狀態不重置） | `yaegi_indicator_script_proxy.go` `prepare`／`preparedScript` | `yaegi_indicator_script_per_candle_test.go:83`（跨棒累計 1／2／3） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 沿用既有算式執行時間上限，逐棒各自計時；逾時即整次中止 | 每一棒有完整允許時間；逾時整次失敗 | `yaegi_indicator_script_proxy.go` `runOver`（每次 `WithTimeoutCause`） | 逾時本身由既有 `yaegi_indicator_script_proxy_test.go` 覆蓋；**逐棒各自計時**這一點沒有專屬斷言 | shallow | produces-oracle | 🟠 mis-asserted |
| NFR-3 | 刻度區間數不得超過單次查詢筆數上限；超過即拒絕並說出數字 | 見 AC-49 | `backtest_domain.go:94` | `backtest_domain_test.go`（a stretch needing more buckets） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 算式仍只能做純運算，重演沒有放寬任何一條 | 可用套件清單未變 | `yaegi_indicator_script_proxy.go` `allowedPackages`（未改動） | 既有 `yaegi_indicator_script_proxy_test.go`（拒絕 `os`／`net/http`／`time`） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-5 | 金額用精確小數；比率用一般數字 | 金額欄位皆 `decimal`；三項比率為 `float64` | `backtest_summary_dto.go`、`closed_trade_dto.go`、`equity_point_dto.go` | `backtest_controller_test.go:99`（金額以字串回傳＝精確小數） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-6 | 重演不寫入任何資料 | 見 AC-40 | 無寫入路徑 | 嚴格 mock | asserts-oracle | produces-oracle | ✅ conforms |

---

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `backtest_equity_curve_domain.go:56`（高點 ≤ 0 時不計回撤） | 同上：對零做除法的守門，且「比零還低多少」不是一個問題 | 同上，已有測試（`backtest_equity_curve_domain_test.go:99`） |
| `backtest_simulation_domain.go`（某棒沒有對應的指標結果即讀作持平） | PRD 假設兩份清單一樣長；這是長度不齊時的收斂行為 | 已有測試（`backtest_simulation_domain_test.go:362`）。屬實作面的防禦，非業務規則 |
| `backtest_request.go`／`backtest_controller.go`（body 無法解讀回 400） | 一般性的請求解析，非本切片的業務規則 | 慣例行為，已有測試（`backtest_controller_test.go:148`） |

**Out of Scope 反向檢查**：PRD 列出的六項（手續費與滑點、止損止盈、槓桿與保證金、一次多個標的、留存與歷史查詢、下一棒開盤成交）在程式碼中**皆無對應實作**——沒有手續費欄位、沒有出場規則、沒有保證金、`BacktestRequestDto` 只收一個 `Symbol`、沒有任何 repository 寫入、成交價唯一決定處寫死為當棒收盤。無 scope creep。

---

## Summary

- **Conforms: 62 / 63 clauses ✅（98%）**
- Violations: 無
- Mis-asserted: `NFR-2` 🟠 —— 逐棒逾時的行為是對的（每一棒各自取得完整允許時間），但**沒有任何測試斷言「每一棒各自計時」**；既有的逾時測試只證明單次執行會逾時。若哪天改成整次共用一份預算，現有測試不會變紅。
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 3（皆為防禦性收斂行為，均有測試；無一觸及 Out of Scope）

### 待辦

1. **NFR-2**：補一個測試，讓一支「每一棒都花掉接近允許時間」的算式在多棒重演下仍然跑完——那會在共用預算的實作下失敗。目前刻意不補，因為要讓它跑得夠久又不拖慢整包測試，需要把允許時間設得極短，而那會讓測試對機器速度敏感（不穩定的測試比沒有測試更糟）。以本文件記錄此一取捨。
2. ~~把「進場價不大於零即不開倉」補進 PRD~~ —— **已完成**：本次稽核把它從 orphan 升級成 AC-15b / BR-6b，程式與測試皆已就位。
