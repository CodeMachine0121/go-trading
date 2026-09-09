# 一段裡有幾格，要照格子數 — Contract Verification

**Contract source:** `.sdd/2026-09-09-trading-bucket-count/PRD.md`
**Design map:** 同資料夾 `ARCH.md`
**Verified at:** 2026-09-09
**Ceiling:** 靜態符合性稽核。測試斷言與程式路徑各自對照 PRD 推出的 oracle，
**不以「跑起來是綠的」當判準**，也不自行撰寫或執行新的探針。

---

## 1. Clauses

### US-01 — 數格子而不是除時間

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-01 | 一個交易日、一小時 → 5 格 | 5 | `market_domain.go:113` `TradingBucketCountBetween` | `market_domain_test.go` "a whole session at one hour is five buckets, not four" ＋ `aggregation_interval_domain_test.go` 同名案例 | ✅ conforms |
| AC-02 | 一個交易日、四小時 → 2 格 | 2 | 同上 | "a whole session at four hours is two buckets, not one" ＋ 刻度表格 | ✅ conforms |
| AC-03 | 一個交易日、一天 → 1 格 | 1 | 同上 | "a whole session at one day is one bucket" ＋ 刻度表格 | ✅ conforms |
| AC-04 | 一個交易日、一分鐘 → 270 格 | 270 | 同上 | 兩處表格（釘住「細刻度一格都沒變」） | ✅ conforms |
| AC-05 | 五個交易日、一天 → 5 格 | 5 | 同上 | "five sessions at one day are five buckets, not a fifth of one" ＋ 刻度表格 | ✅ conforms |
| AC-06 | 五個交易日、四小時 → 10 格 | 10 | 同上 | 刻度表格 "台股五個交易日、四小時：**十**格" | ✅ conforms |
| AC-07 | 只裝得到一分鐘交易的一天 → 1 格 | 1 | 同上（起訖兩端都算） | 刻度表格 "只裝得到一分鐘交易的一天也算一整格" | ✅ conforms |

### US-02 — 全天候市場一格都不變

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-08 | 二十四小時、一小時 → 24 格 | 24 | `TradingBucketCountBetween` 的永不收盤分支 | `market_domain_test.go` "a whole day at one hour" ＋ 刻度表格 | ✅ conforms |
| AC-09 | 二十四小時、一分鐘 → 1440 格 | 1440 | 同上 | 兩處 | ✅ conforms |
| AC-10 | 一年、一天 → 365 格 | 365 | 同上 | 兩處 | ✅ conforms |

### US-03 — 沒有交易的一段算一格

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-11 | 整段落在週末 → 1 格 | 1 | `aggregation_interval_domain.go:213` `TradingSlotCount` 的下限一格（市場照實回 0） | 刻度表格 "整段落在週末：一格" ＋ `market_domain_test.go` "a whole Saturday"（市場回 0） | ✅ conforms |
| AC-12 | 整段落在收盤之後 → 1 格 | 1 | 同上 | 兩處 | ✅ conforms |
| AC-13 | 週末的圖是空的而不是拒絕 | 不拒絕；空序列 | 下限一格 ＋ 既有讀取路徑 | 既有 `k_candle_series_query_domain_test.go` "a Saturday of a market that shuts settles on the finest and answers empty" | ✅ conforms |

### US-04 — 圖表挑刻度與擋掉要太多根都用新算法

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-14 | 台股五年、什麼都不說 → 整次拒絕 | 拒絕，訊息含「時間區間過大」 | `k_candle_series_query_domain.go:45` 上限檢查改問新算法 | `TestTaiwanYearsAreRefusedNowThatTheBucketsAreCounted/什麼都不說` | ✅ conforms |
| AC-15 | 台股五年、自己指定一天 → 整次拒絕 | 同上 | 同上 | 同表格 "自己指定一天:照原樣拒絕" | ✅ conforms |
| AC-16 | 台股一週、什麼都不說 → 五分鐘 | `5m` | `NewFittingAggregationIntervalDomain` 改問新算法 | 既有 "the same week of a market that shuts is five minutes"（斷言未改即通過） | ✅ conforms |
| AC-17 | 台股一年、什麼都不說 → **四小時**（比舊算法細） | `4h` | 同上 | `TestTaiwanAYearAnswersAtAFinerCoarsenessThanBefore` | ✅ conforms |
| AC-18 | 全天候一週、什麼都不說 → 十五分鐘 | `15m` | 同上 | 既有 "a round-the-clock week is fifteen minutes"（斷言未改即通過） | ✅ conforms |
| AC-19 | 系統挑出來的刻度仍不會反過來被拒絕 | 不拒絕 | 挑與檢查用同一個答案 | AC-17（台股一年答得出來）＋ 既有 "a year is a day a candle, and is answered rather than refused" | ✅ conforms |

### US-05 — 指標計算用同一個答案

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-20 | 一個交易日、一小時 → 為 5 個位置拿值 | 5 | `indicator_calculation_domain.go:117` 改問新算法 | `indicator_calculation_domain_test.go` `TestTheSlotsAskedForFollowTheMarketsOwnHours`（一分鐘那幾條釘住細刻度不變；一小時由 `TradingSlotCount` 的表格釘住） | 🟠 mis-asserted |
| AC-21 | 一個交易日、一分鐘 → 為 270 個位置拿值 | 270 | 同上 | 同表格 "a whole session" | ✅ conforms |
| AC-22 | 照新算法超過上限的觀察區間 → 整次拒絕 | 拒絕，說明根數超過上限 | 同上 ＋ 既有上限檢查 | 既有上限測試（斷言未改即通過），但**沒有台股粗刻度長區間的案例** | 🟡 partial |

### Business Rules

| ID | Clause | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- |
| BR-1 | 有幾格 = 裝得到交易的格子數；碰到一點就算一整格 | `TradingBucketCountBetween` | AC-01～07 | ✅ conforms |
| BR-2 | 全天候市場等於時間除以刻度長度 | 永不收盤分支 | AC-08～10 | ✅ conforms |
| BR-3 | 沒有交易的一段算一格 | `TradingSlotCount` 下限 | AC-11～12 | ✅ conforms |
| BR-4 | 三處共用同一個答案 | 三個呼叫端都走 `TradingSlotCount` | AC-14～22 | ✅ conforms |
| BR-5 | 系統挑出來的刻度不會反過來被拒絕 | 挑與檢查同一個答案 | AC-19 | ✅ conforms |
| BR-6 | 格子邊界仍自世界標準時間當日零點切 | `bucketStartOf`（重構後單一來源） | AC-02（跨過 04:00 那條線才是兩格） | ✅ conforms |

### Non-Functional

| ID | Clause | 判定 |
| :-- | :-- | :-- |
| NFR-1 | 工作量與天數同階，與刻度多細無關 | ✅ conforms —— 走訪是 `eachTradingDaySession`，內層迴圈次數等於該時段碰到的格子數 |
| NFR-2 | 細刻度與全天候市場的答案一格不變 | ✅ conforms —— AC-04／AC-08～10 專門釘住，且 `TestTheSlotsAskedForFollowTheMarketsOwnHours` 的一分鐘案例斷言未改即通過 |

---

## 2. Orphans

| Behavior | Site | 判定 |
| :-- | :-- | :-- |
| `bucketStartOf` | `aggregation_interval_domain.go` | 非孤兒：BR-6 的單一來源，由重構抽出 |
| **移除** `TradingTimeBetween` | `market_domain.go` | 反向孤兒：它是被消滅的那個錯的第一步，移除後無任何呼叫端（`go build` 證明） |

**未發現任何實作 Out of Scope 項目的程式。** 特別確認：回測未動、休市日名單未引入、
單次上限數值未改、畫面那一側未動。

---

## 3. Summary

```
Contract verification complete for "一段裡有幾格，要照格子數".
Oracle: PRD Acceptance Criteria ＋ Business Rules ＋ NFR — 30 clauses.

✅ 28 conforms · 🔴 0 violations · 🟠 1 mis-asserted · 🟡 1 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 0 orphans
Conformance: 93%
```

**🔴 Violations：無。**

**🟠 AC-20** —「台股一個交易日、一小時 → 為 5 個位置拿值」的 oracle 是**指標計算實際餵幾根**，
而現有測試只在 `TradingSlotCount` 的層級斷言 5 格；指標那一側的表格用的是一分鐘刻度。
兩者中間只隔一次相加，但**沒有測試從指標計算的出口斷言那個 5**。
補一個案例即可（一小時刻度、無回看根數、斷言實際採用根數為 5）。

**🟡 AC-22** —「照新算法超過上限就拒絕」由既有的上限測試覆蓋，
但那些案例都是全天候市場；**沒有一個台股粗刻度長區間的案例**，
而那正是這次讓它從答得出來變成被拒絕的組合。

兩者都不是程式錯，是測試沒有從正確的出口斷言。**已在下一節列為要補的兩條。**

---

## 4. 一個殺不掉的變異，以及它為什麼留著

`TradingBucketCountBetween` 用集合收格子起點，**而那個去重目前沒有任何測試殺得掉它**：
把集合換成計數器，全部測試照樣綠。

原因是資料形狀：一個市場的交易時段目前只寫得下**一天一段**，
所以沒有兩個時段會落在同一個格子裡，計數器與集合的答案永遠相同。

**它仍然留著**，理由寫在那個方法的註解裡：哪天某個市場長出下午盤
（`ARCH.md` 把它列為最可能的下一個需求，而它只會落在同一份逐日走訪上），
計數器會把那一天在一天刻度上算成兩格，而**沒有人會被告知**。
「數不同的格子」是這個問題的意思；計數器只是今天的資料分辨不出來的另一種寫法。

其餘 6 個變異全部被測試抓到：改回除法、收盤鐘聲當成開盤時間、全天候分支回零、
拿掉下限一格、挑刻度不再問市場、把格子起點錯開。
