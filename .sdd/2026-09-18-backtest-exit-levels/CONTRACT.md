# 回測的止損止盈 — Contract Verification Matrix

**Contract source:** `.sdd/2026-09-18-backtest-exit-levels/PRD.md`（Acceptance Criteria 為 oracle）
**Design map:** `.sdd/2026-09-18-backtest-exit-levels/ARCH.md`
**Glossary:** `.sdd/UL-MAP.md`
**Scope:** `go-trading`（後端）
**Verified:** 2026-09-18
**Ceiling:** 靜態一致性稽核。逐條把**測試斷言**與**程式路徑**各自對照規格推出的 oracle，
不以「跑完全套變綠」當判準。

---

## Clauses

### US-01 — 發動回測時說得出兩個出場距離

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01.1 | 兩個都是選填百分點 | 零或留白＝沒有那個出場 | `backtest_exit_levels_domain.go:66`、`:71`（`IsPositive()` 決定有沒有） | `backtest_exit_levels_domain_test.go:61` | ✅ conforms |
| AC-01.2 | 兩個都不填 | 成績單與這一刀之前逐字相同 | `BacktestExitLevelsDomain{}` 零值；`:53` 零值案例 | **既有的每一條回測測試一條斷言都沒改** ＋ `backtest_exit_levels_domain_test.go:53`、`backtest_exit_simulation_test.go:100`（不填那一半算出 11000） | ✅ conforms |
| AC-01.3 | 只填止損 | 只算止損 | 同 AC-01.1 | `backtest_exit_levels_domain_test.go:61`、`backtest_exit_simulation_test.go:83` | ✅ conforms |
| AC-01.4 | 只填止盈 | 只算止盈 | 同上 | `backtest_exit_levels_domain_test.go:61`、`backtest_exit_simulation_test.go:150` | ✅ conforms |
| AC-01.5 | 兩種回測一字不差 | 腳本重演與交易策略重演行為相同 | `trading_strategy_backtest_request_dto.go:75`（`ToBacktestRequestDto` 帶過去，之後**單一路徑**） | `backtest_domain_test.go:438`、`:445`（交易策略重演的拒絕表格多兩列，指向同一欄位名） | ✅ conforms |
| AC-01.6 | 距離為負 → 整次拒絕 | 句子說出是哪一個距離 | `position_plan_domain.go:103`（**沿用，句子未改動**） | `backtest_exit_levels_domain_test.go:82`（四列：止損負、止損超、止盈負、止盈超） | ✅ conforms |
| AC-01.7 | 距離超過 100 → 整次拒絕 | 理由是價格會變成負數 | `position_plan_domain.go:107` | 同上 | ✅ conforms |
| AC-01.8 | 正好 100 送得出去 | 止損價正好是零 | `validatedDistance` 用 `GreaterThan` 而非 `GreaterThanOrEqual` | `backtest_exit_levels_domain_test.go:74`（斷言止損價字串為 `"0"`） | ✅ conforms |
| AC-01.9 | 拒絕指向「出場距離」這一組 | 一個欄位名，句子說出是哪一格 | `backtest_errors.go:39`、`backtest_domain.go:107` | `backtest_domain_test.go:438`／`:445`（兩個不同的距離出錯，回同一個欄位名） | ✅ conforms |

### US-02 — 被停損掃出場就是被掃出場

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-02.1 | 做多止損 2%、低 97 | 以 98 出場、虧 200、帳上 9800 | `backtest_exit_levels_domain.go:70`＋`backtest_position_domain.go:108` | `backtest_exit_simulation_test.go:83`（四個數字逐字斷言） | ✅ conforms |
| AC-02.2 | 低正好 98 | 出場 | `backtest_position_domain.go:146`（`LessThanOrEqual`） | `backtest_exit_simulation_test.go:117`（第一列） | ✅ conforms |
| AC-02.3 | 低 99 | 不出場 | 同上 | `backtest_exit_simulation_test.go:117`（第二列，斷言帳上 10100） | ✅ conforms |
| AC-02.4 | 止盈 5%、高 106 | 以 105 出場、賺 500、帳上 10500 | `backtest_exit_levels_domain.go:75`＋`backtest_position_domain.go:131` | `backtest_exit_simulation_test.go:150` | ✅ conforms |
| AC-02.5 | 用高低點判，不用收盤價 | 低 97 收 101 照樣出場 | `backtest_position_domain.go:125`（讀 `kCandle.High`／`.Low`） | `backtest_exit_simulation_test.go:83`（那一棒**收 101**，仍以 98 出場） | ✅ conforms |
| AC-02.6 | 成交在價位本身 | 不計滑點 | `backtest_position_domain.go:129`（直接傳 `StopLossPrice`） | 同 AC-02.1（斷言 `exitPrice == "98"`） | ✅ conforms |
| AC-02.7 | 距離從進場價量 | 非參考價 | `backtest_position_domain.go:53`（`PricesFrom(direction, entryPrice)`） | `backtest_exit_simulation_test.go:249`（重新進場後止損價是 98.98，不是 98） | ✅ conforms |
| AC-02.8 | 做空完全反過來 | 止損 102、止盈 95 | `backtest_exit_levels_domain.go:64` 的 `isShort`（兩處反向） | `backtest_exit_levels_domain_test.go:43`、`backtest_exit_simulation_test.go:175`（兩個子案例） | ✅ conforms |
| AC-02.9 | 現貨照樣算 | 只有一種排列 | 未加任何模式分支——出場只認方向 | `backtest_exit_simulation_test.go:201`（現貨模式跑同一段，以 98 出場） | ✅ conforms |

### US-03 — 那兩條時序規則

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-03.1 | 進場那一棒不檢查 | 第一棒低 96、止損 98 → 不出場 | `backtest_simulation_domain.go:81`（`ApplyExitLevels` 在 `Apply` 之前） | `backtest_exit_simulation_test.go:215`（斷言零筆平倉、帳上 11000） | ✅ conforms |
| AC-03.2 | 同一棒兩個都碰到 → 止損 | 以 98 出場、虧 200 | `backtest_position_domain.go:126`（止損先問，先回傳） | `backtest_exit_simulation_test.go:164` | ✅ conforms |
| AC-03.3 | 出場後那一棒信號照常判 | 以止損價出場，再以收盤價重新進場 | `backtest_simulation_domain.go:81`／`:82` 兩次獨立呼叫 | `backtest_exit_simulation_test.go:228`（斷言開倉次數 2、一筆止損平倉） | ✅ conforms |
| AC-03.4 | 出場後信號是持有 | 那一棒權益曲線點是純現金 | `Apply` 對 `TargetPositionUnchanged` 直接 return | `backtest_exit_simulation_test.go:83`（第二棒收 101 而帳上 9800——若信號被吃或部位還在，數字會不同） | 🟡 partial |
| AC-03.5 | 重新進場止損價重算 | 從新進場價量 | `backtest_position_domain.go:53`（新部位自己算） | `backtest_exit_simulation_test.go:249`（兩筆出場價 98 與 98.98） | ✅ conforms |
| AC-03.6 | 每一棒順序固定 | 先價位、再信號 | `backtest_simulation_domain.go:74-82`（註解寫明這是規則本身） | AC-03.1／03.3 兩條各從一側釘住它 | ✅ conforms |

**AC-03.4 為何 partial**：權益曲線那一個點沒有被逐點斷言，斷言的是**最終權益**。
兩者在這個案例裡等價（最後一棒就是出場那一棒），但沒有一條測試直接讀
「出場那一棒的曲線點」。既有的資金曲線測試涵蓋「每根一點」與計價方式，
而這一刀沒有改動它們。

### US-04 — 成績單說得出出場是怎麼發生的

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-04.1 | 每一筆帶出場原因 | 三選一 | `closed_trade_vo.go`（`ExitReason`）、`closed_trade_dto.go`、`backtest_position_domain.go:162` | `backtest_exit_simulation_test.go:83`／`:150`／`:267`（三種原因各一） | ✅ conforms |
| AC-04.2 | 成績單多兩個數字 | 止損／止盈出場筆數 | `backtest_account_domain.go:180`、`backtest_simulation_domain.go:94` | `backtest_exit_simulation_test.go:83`（1 與 0） | ✅ conforms |
| AC-04.3 | 都沒填 → 兩個都 0、原因都是訊號 | — | `ExitCountFor` 數明細；沒有價位就沒有那種原因 | `backtest_exit_simulation_test.go:267` | ✅ conforms |
| AC-04.4 | 三筆混合 | 止損 2、止盈 0 | 同 AC-04.2 | `backtest_exit_simulation_test.go:249`（兩筆止損，斷言筆數 2） | 🟡 partial |
| AC-04.5 | 結束時還開著的沒有原因 | 不進明細 | 未改動（`ClosedTradeDtos` 只走 `closedTrades`） | `backtest_exit_simulation_test.go:215`（斷言明細為空而權益 11000） | ✅ conforms |
| AC-04.6 | 其餘數字一併反映 | 勝率、回落、報酬率 | 未改動——它們讀同一段歷史 | `backtest_exit_simulation_test.go:100`（總報酬率從 +10% 變 −2%）、Postman（`totalReturnRate` −0.02） | ✅ conforms |

**AC-04.4 為何 partial**：釘住的是「兩筆止損、零筆止盈」，不是「一筆訊號＋兩筆止損」
的三筆混合。三種原因各自出現在其他案例裡，但沒有一個案例同時包含三種。
補一個混合案例只會重覆已被三條各自釘住的同一段程式路徑。

### US-07 — 訊息裡那一行警告改寫

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-07.1 | 不刪掉 | 仍然印一行 | `strategy_bot_message_domain.go:206` | `strategy_bot_message_domain_test.go`（三處斷言那一行存在，措辭同步更新） | ✅ conforms |
| AC-07.2 | 說得出下一步 | 「重演時把這兩個距離填上」 | 同上 | 同上 | ✅ conforms |
| AC-07.4 | 沒有部位規劃的機器人一個字沒變 | 不印那一行 | 未改動的 `HasStopLoss || HasTakeProfit` 條件 | `strategy_bot_message_domain_test.go:420`（`NotContains`） | ✅ conforms |

**AC-07.3（MCP 那句同義警告）** 在 `go-trading-mcp` 的切片文件裡驗證。
**US-05（助手）**、**US-06（畫面）** 同理，各在自己的專案。

### Core Business Rules（PRD §4）

| BR | Implementation | Test | Status |
| :--- | :--- | :--- | :--- |
| BR-01 選填、留白即不模擬 | 零值 | 既有測試全綠且**斷言未改** | ✅ |
| BR-02 拒絕的兩句話 | `validatedDistance`（共用一份） | `backtest_exit_levels_domain_test.go:82` | ✅ |
| BR-03 正好 100 允許 | `GreaterThan` | `:74` | ✅ |
| BR-04 從進場價量 | `PricesFrom(direction, entryPrice)` | `:249` | ✅ |
| BR-05 用高低點 | `kCandle.High`／`.Low` | `:83`（收盤價 101 不救它） | ✅ |
| BR-06 成交在價位本身 | 直接傳價位 | `:83` | ✅ |
| BR-07 進場那一棒不檢查 | **順序**，非判斷 | `:215` | ✅ |
| BR-08 兩個都碰到算止損 | 止損先問 | `:164` | ✅ |
| BR-09 出場後信號照常判 | 兩次獨立呼叫 | `:228` | ✅ |
| BR-10 重新進場重算 | 新部位自己算 | `:249` | ✅ |
| BR-11 原因與筆數 | `ExitReason`、`ExitCountFor` | `:83`／`:267` | ✅ |
| BR-12 還開著的沒有原因 | 未改動 | `:215` | ✅ |

### Edge Cases（PRD §4）

| 情境 | 驗證 | Status |
| :--- | :--- | :--- |
| 第一棒低點已在止損價下 | `:215` | ✅ |
| 同一棒止損掃出＋信號反手 | `:228`（同棒重新買入，開倉次數 2） | 🟡 partial — 測的是同向重新進場，不是反手開空 |
| 止損掃出後現金不足 | 未改動的 `StakeFor` 分支 | 🟡 partial — 沒有專屬案例；走的是切片前既有路徑 |
| 止損距離 100%（價位 0） | `:74` 斷言價位為 0；價格不可能 ≤ 0 故永不觸發 | 🟡 partial — 沒有「跑一整段都不觸發」的模擬案例 |
| 最後一棒才碰到 | `:83`（兩棒，第二棒就是最後一棒） | ✅ |
| 整段都沒碰到 | `:117` 第二列、`:215` | ✅ |

### Non-Functional（PRD §6）

| 要求 | 驗證 | Status |
| :--- | :--- | :--- |
| **相容性：既有數字逐字不變** | 既有回測測試**一條斷言都沒改**（只有建構子多一個引數的機械式改動）；全套綠 | ✅ conforms |
| 正確性以數字表釘住 | 新增兩個測試檔共 23 個案例，全部是數字 | ✅ conforms |
| 同一個走訪不多一圈 | `backtest_simulation_domain.go:81` 在同一個 `for` 裡 | ✅ conforms |
| 精度 | 全程 `decimal`；`High`／`Low` 由 `decimal.NewFromFloat` 轉入，與既有 `Close` 一致 | ✅ conforms |

---

## Orphans

實作了規格沒有明說的東西：

| 項目 | 判斷 |
| :--- | :--- |
| `reachedBy` 一個表達式蓋四個方向 | **合理**。四個案例是「價位在上或在下」的同一個問題，寫四次必有一次比較寫反 |
| `settleOpenPosition` 抽成私有 helper | **合理**。兩個公開呼叫者（`Apply`、`ApplyExitLevels`），過得了單一呼叫者的門檻；漏掉其中一行是錢憑空出現或消失 |
| `ExitOn` 回傳一整筆已平倉交易而非三個值 | **合理**。呼叫端接下來三件事讀的都是這個形狀的欄位 |
| `ExitCountFor(reason)` 一個方法而非兩個欄位 | **合理**，與勝率同一個手法：計數器是同一個事實的第二個住處 |

規格說了而沒實作的：**無**（US-05／US-06 屬其他兩個專案）。

---

## Summary

| | 數量 |
| :--- | ---: |
| ✅ conforms | 31 |
| 🟡 partial | 6 |
| ❌ violates | 0 |
| 未實作 | 0 |

**六個 partial 全部是「測試覆蓋的顆粒度」，沒有一個是行為不符。**
其中四個（現金不足、反手開空、100% 不觸發、出場那一棒的曲線點）走的都是
**這一刀沒有改動的既有程式路徑**，補案例只會重覆既有測試已經釘住的東西。

### 值得記下來的兩件事

**一、那條時序規則在程式裡找不到。**
「進場那一棒不檢查」沒有任何一行程式碼對應它——它是
`ApplyExitLevels` 排在 `Apply` 前面這件事。這是這一刀最好的一個決定，也是
最容易被後來的人破壞的一個：把那兩行對調，`backtest_exit_simulation_test.go:215`
會立刻變紅，而那是唯一的守門員。兩處註解都寫明了。

**二、相容性的證據是「沒有改任何斷言」。**
既有的回測測試檔只多了一個建構子引數，沒有一條 `assert` 被動過。
那是「不填就什麼都沒變」唯一站得住的證明——改了任何一條，這句話就再也證明不了。
