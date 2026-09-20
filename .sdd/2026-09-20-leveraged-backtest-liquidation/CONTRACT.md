# 回測的槓桿與強制平倉 — Contract Conformance Matrix

**Oracle:** `PRD.md` §3 Acceptance Criteria（42 個情境）、§4 Core Business Rules（8 條）、§6 Non-Functional Requirements（4 條）
**Audited:** 2026-09-20
**Ceiling:** 這是一次**靜態一致性稽核**。每一條都先只從規格推出預期結果，再**各自獨立**判斷
「測試有沒有斷言這個結果」與「程式有沒有產出這個結果」。它不撰寫新探針、不執行自己發明的情境；
判決來自與 oracle 的比對，不是來自整套測試綠不綠。

---

## Clauses

`T` ＝ 測試稽核（`asserts-oracle` / `mis-asserted` / `shallow` / `no-test`）·
`C` ＝ 程式稽核（`produces-oracle` / `diverges` / `unclear` / `not-implemented`）

### US-01 — 說出這一次要開幾倍槓桿

| ID | 情境 | Oracle（只從規格推出） | 實作 | 測試 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01 | 說出槓桿倍數就會模擬強制平倉 | 槓桿 5 被接受，且這一次有強制平倉價 | `backtest_leverage_domain.go:54` | `backtest_leverage_domain_test.go` `…PlacesTheLiquidationPriceAgainstThePosition` | asserts-oracle | produces-oracle | ✅ |
| AC-02 | 槓桿倍數留白就當作沒有槓桿 | 接受；無強制平倉；數字與這一刀之前逐字相同 | `backtest_leverage_domain.go:74` | `…TreatsNothingZeroAndOneAsBorrowingNothing` ＋ `TestUnleveragedReplayIsUntouched` | asserts-oracle | produces-oracle | ✅ |
| AC-03 | 槓桿倍數是零就當作沒有槓桿 | 同上 | 同上 | 同上（"nothing declared"／"0"） | asserts-oracle | produces-oracle | ✅ |
| AC-04 | 槓桿倍數是一也當作沒有槓桿 | 同上；理由是一倍沒有借錢 | `backtest_leverage_domain.go:88` | 同上（"one, written out"／"1.0"） | asserts-oracle | produces-oracle | ✅ |
| AC-05 | 槓桿倍數小於一整份拒絕 | 拒絕，訊息「槓桿倍數不得小於 1 倍」；指向槓桿那一組 | `backtest_leverage_domain.go:82`、欄位對映 `backtest_domain.go:118` | `…RefusesFiguresItCannotTrade`（0.5／−2） | asserts-oracle | produces-oracle | ✅ ¹ |
| AC-06 | 現貨開不了槓桿 | 拒絕，訊息含「現貨」；指向槓桿那一組 | `backtest_leverage_domain.go:93` | domain 表 ＋ `backtest_controller_test.go` `…CarriesTheLeverage`（field＝`leverage`） | asserts-oracle | produces-oracle | ✅ |
| AC-07 | 現貨不說槓桿照常 | 接受；數字逐字相同 | `backtest_leverage_domain.go:74` | `…LetsSpotThroughWhenNothingIsBorrowed` ＋ 既有現貨重演整組 | asserts-oracle | produces-oracle | ✅ ² |
| AC-08 | 重演一份交易策略也說得出槓桿 | 接受；會模擬強制平倉 | `trading_strategy_backtest_request_dto.go`、`trading_strategy_backtest_request.go` | `trading_strategy_backtest_controller_test.go` `…CarriesTheLeverage`（15000） | asserts-oracle | produces-oracle | ✅ |
| AC-09 | 助手也說得出槓桿 | 接受；規則與使用者自己發動一字不差 | `trading_strategy_backtest_assistant_query.go` | `…AssistantQueryCanBorrow`（15000／11000／schema） | asserts-oracle | produces-oracle | ✅ |

¹ 訊息由 domain 表逐字斷言；「指向槓桿那一組」是 `BacktestLeverageField` 這**單一**對映，
由 AC-06、AC-13 的 endpoint 測試各斷言一次——同一行程式碼，兩個方向。
² 「逐字相同」由既有的現貨重演測試整組（未修改、全綠）背書，而非單一新測試。

### US-02 — 說出撐不住的界線在哪

| ID | 情境 | Oracle | 實作 | 測試 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-10 | 說出維持保證金率 | 槓桿 5 ＋ 1% 被接受 | `backtest_leverage_domain.go:98` | `…WorksOutHowFarAPositionMayFall` | asserts-oracle | produces-oracle | ✅ |
| AC-11 | 留白就用市場常見值 | 維持保證金率為 0.5%（強平距離與明寫 0.5 相同） | `backtest_leverage_domain.go:100` | `…FallsBackToTheRateVenuesUse` | asserts-oracle | produces-oracle | ✅ |
| AC-12 | 不得為負 | 拒絕，訊息說不得為負 | `backtest_leverage_domain.go:67` | `…RefusesFiguresItCannotTrade`（−1） | asserts-oracle | produces-oracle | ✅ |
| AC-13 | 大到開倉當下就撐不住 | 拒絕；訊息說出 5 倍下最多能填多少（< 20%） | `backtest_leverage_domain.go:112` | domain 表（20／25，逐字）＋ endpoint（含「20%」） | asserts-oracle | produces-oracle | ✅ |
| AC-14 | 19% 剛好還開得成 | 接受；強平距離 1% | `backtest_leverage_domain.go:110` | `…WorksOutHowFarAPositionMayFall`（"the rate eats into it"） | asserts-oracle | produces-oracle | ✅ |
| AC-15 | 沒有槓桿時不影響任何結果 | 接受；數字與沒說時相同 | `backtest_leverage_domain.go:74` | `…IgnoresTheRateWhenNothingIsBorrowed` ＋ `TestUnleveragedReplayIgnoresAMaintenanceMarginRate` | asserts-oracle | produces-oracle | ✅ |

### US-03 — 賺賠照曝險算

| ID | 情境 | Oracle | 實作 | 測試 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-16 | 開倉扣保證金、承擔曝險 | 可用資金扣 10000；曝險 50000 | `backtest_position_terms_domain.go:79`、`backtest_account_domain.go:138` | `…MultipliesWhatAStakeExposes`（50000）＋ `…ChargesTheExposureRatherThanTheStake`（現金歸零） | asserts-oracle | produces-oracle | ✅ |
| AC-17 | 順著走賺曝險的比例 | 價 110 時值 15000 | `backtest_position_domain.go:112` | `TestLeveragedReplayMovesTheExposureRatherThanTheStake` | asserts-oracle | produces-oracle | ✅ |
| AC-18 | 逆著走賠曝險的比例 | 價 90 時值 5000 | 同上 | `TestLeveragedReplayLosesTheExposureJustAsFast` | asserts-oracle | produces-oracle | ✅ |
| AC-19 | 沒有槓桿時賺賠沒變 | 價 110 時值 11000 | `backtest_leverage_domain.go:140`（乘一） | `TestUnleveragedReplayIsUntouched` | asserts-oracle | produces-oracle | ✅ |

### US-04 — 撐不住時如實被打掉

| ID | 情境 | Oracle | 實作 | 測試 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-20 | 多倉強平價在下方 | 80.5 | `backtest_exit_levels_domain.go:99` | `…PlacesTheLiquidationPriceAgainstThePosition` | asserts-oracle | produces-oracle | ✅ |
| AC-21 | 空倉強平價在上方 | 119.5 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ |
| AC-22 | 槓桿越低撐得越遠 | 2 倍 → 50.5 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ |
| AC-23 | 槓桿越高撐得越近 | 20 倍 → 95.5 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ |
| AC-24 | 沒有槓桿就沒有強平價 | 沒有強平價；永不被強制出場 | `backtest_leverage_domain.go:155` | `…HasNoLiquidationPriceWithoutALoan` ＋ `TestReportCardCountsNoLiquidationsWithoutALoan` | asserts-oracle | produces-oracle | ✅ |
| AC-25 | 碰到就全沒了 | 在 80.5 出場；收回 0；原因＝強平 | `backtest_position_domain.go:290`、`:237` | `TestLeveragedReplayIsWipedOutWhenTheLoanIsCalledIn` | asserts-oracle | produces-oracle | ✅ |
| AC-26 | 正好碰到算碰到 | 最低價 80.5 → 被強制出場 | `backtest_position_domain.go:207`（`>=`／`<=`） | `…CountsTouchingTheLiquidationPriceAsReachingIt` | asserts-oracle | produces-oracle | ✅ |
| AC-27 | 差一點就繼續撐著 | 最低價 80.6 → 沒出場，繼續開著 | 同上 | `…HoldsOnJustShortOfTheLiquidationPrice`（權益 500） | asserts-oracle | produces-oracle | ✅ |
| AC-28 | 跳空崩跌也賠不過押下去的錢 | 仍在 80.5 出場；收回 0；可用資金非負 | `backtest_position_domain.go:290` | `TestLeveragedReplayCannotLoseMoreThanTheStake` | asserts-oracle | produces-oracle | ✅ |

### US-05 — 止損比強平近時，止損先救

| ID | 情境 | Oracle | 實作 | 測試 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-29 | 止損近就永遠止損出場 | 在 95 出場，原因止損，沒有被強平 | `backtest_exit_levels_domain.go:92` | `…TakesWhicheverAdverseExitIsNearer`（含兩個計數） | asserts-oracle | produces-oracle | ✅ |
| AC-30 | 止損遠就永遠輪不到它 | 在 80.5 出場，原因強平 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ |
| AC-31 | 沒設止損就由強平接手 | 在 80.5 出場，原因強平 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ |
| AC-32 | 同一根碰到兩邊時逆向先算 | 在 95 出場，原因止損 | `backtest_position_domain.go:169`（逆向先問） | `…StillReadsTheAdverseSideFirst` | asserts-oracle | produces-oracle | ✅ |
| AC-33 | 沒有槓桿時止損行為沒變 | 在 95 出場，原因止損 | `backtest_exit_levels_domain.go:88` | 出場價位表（"a stop with no loan behind it"）＋ 既有 exit 重演整組 | asserts-oracle | produces-oracle | ✅ |

### US-06 — 成本照曝險收

| ID | 情境 | Oracle | 實作 | 測試 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-34 | 進場成本收曝險 | 押 10000、5 倍、0.1% → 50 | `backtest_position_domain.go:78` | `TestBorrowedPositionChargesBothEndsOnTheExposure` | asserts-oracle | produces-oracle | ✅ |
| AC-35 | 沒有槓桿時進場成本沒變 | 同條件無槓桿 → 10 | 同上（乘一） | `TestUnborrowedPositionChargesTheStakeAsItAlwaysDid` | asserts-oracle | produces-oracle | ✅ |
| AC-36 | 全押縮到付得起 | 押注＋成本 ≤ 現金；現金非負 | `backtest_transaction_costs_domain.go:119` | `…ChargesTheExposureRatherThanTheStake`（成本 200、權益 10000） | asserts-oracle | produces-oracle | ✅ |
| AC-37 | 固定金額付不起就跳過 | 不開倉、不中斷、不計為一次開倉 | `position_sizing_domain.go:159`、`backtest_position_terms_domain.go:85` | `…SkipsAnOpeningItCannotAffordTheChargeFor` | asserts-oracle | produces-oracle | ✅ |
| AC-38 | 出場成本收放大後的成交金額 | 500 單位 × 110 × 0.1% ＝ 55 | `backtest_position_domain.go:240` | `TestBorrowedPositionChargesBothEndsOnTheExposure` | asserts-oracle | produces-oracle | ✅ |

### US-07 — 成績單講得出強平

| ID | 情境 | Oracle | 實作 | 測試 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-39 | 被強平的那次算在強平那一格 | 強平 1、止損 0 | `backtest_simulation_domain.go`（`ExitCountFor`） | `TestReportCardSeparatesLiquidationsFromStops` ＋ `…TakesWhicheverAdverseExitIsNearer` | asserts-oracle | produces-oracle | ✅ |
| AC-40 | 被止損救下的那次算在止損那一格 | 止損 1、強平 0 | 同上 | `…TakesWhicheverAdverseExitIsNearer`（"a stop inside…"） | asserts-oracle | produces-oracle | ✅ |
| AC-41 | 沒開槓桿強平筆數恆為零 | 0 | 同上 | `TestReportCardCountsNoLiquidationsWithoutALoan` | asserts-oracle | produces-oracle | ✅ |
| AC-42 | 交易明細指得出是哪一筆 | 那一筆出場原因＝強平 | `vo/trade_exit_reason_vo.go:24` | `TestLeveragedReplayIsWipedOutWhenTheLoanIsCalledIn` | asserts-oracle | produces-oracle | ✅ |

### Core Business Rules

| ID | 規則 | Oracle | 實作 | T | C | 狀態 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| BR-1 | 槓桿的開關（留白／0／1 ＝ 沒有槓桿） | 三者行為相同 | `backtest_leverage_domain.go:74,88` | asserts-oracle | produces-oracle | ✅ |
| BR-2 | 曝險 ＝ 押注 × 槓桿 | 乘法；無槓桿時乘一 | `backtest_leverage_domain.go:140` | asserts-oracle | produces-oracle | ✅ |
| BR-3 | 強平距離 ＝ (1÷槓桿) − 維持保證金率 | 四組數字全中 | `backtest_leverage_domain.go:155` | asserts-oracle | produces-oracle | ✅ |
| BR-4 | 強平價多下空上，**開倉當下算好不再變動** | 方向正確；同一注的價位不隨行情改變 | `backtest_exit_levels_domain.go:102`、`backtest_position_domain.go:80`（開倉當下存進 `exitPrices`） | 方向 asserts-oracle；「不再變動」 shallow | produces-oracle | 🟠 ¹ |
| BR-5 | 誰先到：同邊比距離，近的先 | 三種情況正確 | `backtest_exit_levels_domain.go:92` | asserts-oracle | produces-oracle | ✅ |
| BR-6 | 不穿倉 | 收回 0；可用資金非負 | `backtest_position_domain.go:290` | asserts-oracle | produces-oracle | ✅ |
| BR-7 | 可押上限分母含槓桿 | 無槓桿時退回原式 | `backtest_transaction_costs_domain.go:129` | asserts-oracle | produces-oracle | ✅ |
| BR-8 | 逐字相容（上位約束） | 沒說槓桿時每個數字不變 | 零值貫穿全域 | asserts-oracle | produces-oracle | ✅ ² |

¹ 「開倉當下算好不再變動」沒有專屬測試——它是結構性不變式（價位在建構子寫入 struct，
之後沒有任何路徑重算）。既有的「被掃出場後再開一注，價位從新的進場價量起算」測試
間接覆蓋了止損那一半。判為 🟠 而非 ✅ 是因為**斷言不存在**，不是因為程式有疑慮。
² 由既有測試整組未修改且全綠背書，加上四個明寫的對照案例
（AC-02、AC-19、AC-33、AC-35）。

### Non-Functional Requirements

| ID | 要求 | Oracle | 判定 | 狀態 |
| :--- | :--- | :--- | :--- | :--- |
| NFR-1 | 出場價位開倉算一次，不逐棒重算 | 每根 K 線只做比大小 | `ExitOn` 只讀 `exitPrices`，無重算；同 BR-4，無專屬斷言 | 🟠 |
| NFR-2 | 沒說槓桿時輸出逐字不變 | 同 BR-8 | 既有測試整組綠 ＋ 四個對照案例 | ✅ |
| NFR-3 | 金額與價格一律精確小數 | 不出現 `float64` 運算 | 新增碼全為 `decimal`；`vo.KCandleVo` 的 `High`／`Low` 仍是既有的 `float64`，在 `ExitOn` 進場處轉換（既有做法，未改動） | ✅ |
| NFR-4 | 安全性無新增 | 不留存、不跨使用者 | 兩個欄位皆隨呼叫走，未進任何 entity | ✅ |

---

## Orphans

| 行為 / 型別 | 說明 | 判定 |
| :--- | :--- | :--- |
| `BacktestPositionTermsDomain` | `/improve-codebase` 階段加入，**不帶任何新業務行為**：把倉位大小模式、出場價位、槓桿設定、交易成本合成一次回答。已補進 `UL-MAP.md` 與 `ARCH.md` §3／§6。 | 非孤兒（結構性） |
| `BacktestPositionDomain.Stake()` | 帳戶要扣的錢由倉位回答，取代原本帳戶自己持有的區域變數。 | 非孤兒 |

**Out of Scope 反向檢查**（皆未實作，無越界）：資金費率、逐倉／全倉、分批強平、
機器人的建議部位（僅呼叫點簽名跟著改，行為由既有測試證明不變）、一次掃多標的、任何留存。

---

## Summary

| | 數量 |
| :--- | ---: |
| ✅ conforms | 50 |
| 🔴 violations | 0 |
| 🟠 mis-asserted / shallow | 2（BR-4、NFR-1，同一件事） |
| 🟡 partial | 0 |
| ❌ gaps | 0 |
| ❔ unclear | 0 |
| ⚠️ orphans | 0 |

**Conformance: 50 / 52 ＝ 96%**

### 本次稽核發現並已修正的一件事

原 PRD 的 US-07 第一個情境寫著「其中三注被強制出場、兩注被止損掃出場」。
**那個狀態做不出來**：止損距離與強平距離在一次重演裡都是常數，所以「哪一個近」
對每一注都一樣，一次重演裡的逆向出場只會有一種。

而這符合現實——5 倍槓桿配 5% 停損就是永遠不會爆，配 30% 停損就是永遠輪不到停損。
所以錯的是規格，不是程式。該情境已拆成 AC-39 / AC-40 兩面，並在 PRD 裡寫明原因；
`…TakesWhicheverAdverseExitIsNearer` 三個案例各補上兩個計數的斷言，
讓任一格都不可能安靜地收走另一格的交易。

### 剩下的兩條（同一件事）

BR-4 與 NFR-1 講的都是「出場價位在開倉當下算好，之後不再重算」。程式確實如此
（價位在建構子寫入 struct，之後沒有任何路徑重算），但**沒有一條斷言會因為它被破壞而變紅**。
要把它們轉成 ✅，需要一個「同一注在兩根不同的 K 線上被問，價位相同」的行為測試。
判為 🟠 而非 ✅ 是因為斷言不存在，不是因為程式有疑慮。
