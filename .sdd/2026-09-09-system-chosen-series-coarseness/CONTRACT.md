# 沒說要多粗就由系統挑 — Contract Verification

**Contract source:** `.sdd/2026-09-09-system-chosen-series-coarseness/PRD.md`
**Design map:** 同資料夾 `ARCH.md`
**Verified at:** 2026-09-09
**Ceiling:** 靜態符合性稽核。測試斷言與程式路徑各自對照 PRD 推出的 oracle，
**不以「跑起來是綠的」當判準**，也不自行撰寫或執行新的探針。

---

## 1. Clauses

### US-01 — 什麼都不說就拿到一種看得清楚的刻度

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-01 | 全天候市場問一週 → 十五分鐘 | 回覆說出的刻度為 `15m` | `k_candle_series_query_domain.go:127` 分支④ | `k_candle_series_query_domain_test.go` "a round-the-clock week is fifteen minutes" | ✅ conforms |
| AC-02 | 會收盤的市場問同樣一週 → 五分鐘 | 回覆說出的刻度為 `5m` | 同上＋`MarketDomain.TradingTimeBetween` | 同表 "the same week of a market that shuts is five minutes" | ✅ conforms |
| AC-03 | 三十分鐘 → 一分鐘 | 回覆說出的刻度為 `1m` | 同上 | 同表 "half an hour is still the finest there is" | ✅ conforms |
| AC-04 | 恰好 1000 分鐘交易時間 → 一分鐘 | 回覆說出的刻度為 `1m` | 同上 | 同表 "a stretch holding exactly as many minutes as the ceiling allows" | ✅ conforms |
| AC-05 | 1001 分鐘 → 五分鐘 | 回覆說出的刻度為 `5m` | 同上 | 同表 "one minute more than the ceiling allows steps to the next coarseness" | ✅ conforms |
| AC-06 | 十年 → 照原樣拒絕，說明區間過大 | 拒絕，訊息含「時間區間過大」 | 分支④取最粗 ＋ `:63` 既有上限檢查 | `TestNewKCandleSeriesQueryDomainStillRefusesWhatNoCoarsenessCanHold` | ✅ conforms |
| AC-07 | 整段落在週末 → 空序列，不是拒絕 | 不拒絕；序列為空 | 分支④（`SlotCount(0)` 為 1，故不觸發任何拒絕） | 同表 "a Saturday of a market that shuts settles on the finest and answers empty" | ✅ conforms |

### US-02 — 系統挑出來的刻度不會反過來被系統拒絕

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-08 | 一年、什麼都不說 → 答得出來 | 不拒絕；刻度為 `1d` | 分支④ | 同表 "a year is a day a candle, and is answered rather than refused" | ✅ conforms |
| AC-09 | 一年、自己指定一分鐘 → 拒絕 | 拒絕，訊息含「時間區間過大」 | 分支② ＋ 既有上限檢查（皆不改） | `TestNewKCandleSeriesQueryDomainKeepsRefusingACoarsenessTheCallerNamedItself` ＋ controller "naming a minute over a week is still refused" | ✅ conforms |

### US-03 — 自己指定刻度或說出可顯示根數那條路一字不變

| ID | Clause | Oracle | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- | :-- |
| AC-10 | 自己指定五分鐘 → 用五分鐘 | 回覆說出的刻度為 `5m` | 分支② | 既有測試（未改斷言即通過） | ✅ conforms |
| AC-11 | 說最多擺得下 400 根、一週 → 一小時 | 回覆說出的刻度為 `1h` | 分支③（`min(400, 1000)`） | `TestNewKCandleSeriesQueryDomainStillHonoursADisplayBudgetWhenOneIsNamed` | ✅ conforms |
| AC-12 | 同時說兩種 → 整次拒絕 | 拒絕，訊息含「只能挑一種」 | 分支① | 既有 `TestSeriesQueryRefusesBothWaysOfAskingAtOnce` | ✅ conforms |
| AC-13 | 指定七分鐘 → 拒絕並列出六種 | 拒絕，訊息列出六種刻度 | 分支②→`NewAggregationIntervalDomain` | 既有測試 ＋ controller "reports an interval nobody offers as a bad request" | ✅ conforms |
| AC-14 | 說擺得下零根 → 拒絕 | 拒絕，訊息含「可顯示根數必須大於零」 | 分支③ | 既有 `TestSeriesQueryRefusesADisplayThatHoldsNothing` | ✅ conforms |

### Business Rules

| ID | Clause | Impl | Test | Status |
| :-- | :-- | :-- | :-- | :-- |
| BR-1 | 兩種都不說即由系統挑，擺得下幾根以單次上限為準 | `:127` | AC-01～05 | ✅ conforms |
| BR-2 | 只數市場實際會成交的時間 | `MarketDomain.TradingTimeBetween`（不改） | AC-01 vs AC-02 對照 | ✅ conforms |
| BR-3 | 系統挑得出來的刻度不會被「一次要太多」拒絕 | `:127` 以上限為界 | AC-08 | ✅ conforms |
| BR-4 | 連最粗的都裝不下的區間仍然被拒絕 | 既有上限檢查 | AC-06 | ✅ conforms |
| BR-5 | 回覆一律說出實際用的刻度 | `KCandleSeriesDto.Interval`（不改） | 每一條 AC 都是靠它斷言的 | ✅ conforms |
| BR-6 | 自己指定刻度那條路一字不改 | 分支② | AC-09、AC-10、AC-13 | ✅ conforms |
| BR-7 | 整段沒有交易時回空序列 | 既有讀取路徑 | AC-07 | ✅ conforms |

### Non-Functional

| ID | Clause | 判定 |
| :-- | :-- | :-- |
| NFR-1 | 自己指定刻度、說出可顯示根數的既有用途行為一字不變 | ✅ conforms —— 既有測試**斷言一個字都沒改**即全數通過，那就是證據本身 |
| NFR-2 | 挑刻度最多走六次（既有做法不變） | ✅ conforms —— `NewFittingAggregationIntervalDomain` 未改 |
| NFR-3 | 無新增存取面 | ✅ conforms —— 無新路由、無新介面方法、DTO 未加欄位 |

---

## 2. Orphans

**無。** 這一輪沒有新增任何型別、方法或欄位；唯一的實質改動是 `intervalFor` 的分支結構。
特別確認：無休市日名單、無補洞、無造假 K 線、無市場分支、無新的哨兵錯誤。

---

## 3. Summary

```
Contract verification complete for "沒說要多粗就由系統挑".
Oracle: PRD Acceptance Criteria ＋ Business Rules ＋ NFR — 24 clauses.

✅ 24 conforms · 🔴 0 violations · 🟠 0 mis-asserted · 🟡 0 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 0 orphans
Conformance: 100%
```

第一輪稽核為 22 ✅ / 2 🟡（AC-07 與 AC-11 程式對但沒有專屬測試）。兩者各補一個案例後全數符合。

**實作期間規格被修正過一次。** 原本 AC-06 寫成「十年 → 用一天，這不是拒絕」，
實作時發現既有的「切太多根就拒絕」仍然會擋（一天一根切出 3650 根）。
**改的是規格不是程式**：那條拒絕是這個切片明確不動的既有規則，
而「一千天以上的 K 線圖」本來就不是任何人在看的東西。
畫面那一側因此有責任把可見區間收在一千天之內——這一點寫進了下一個切片的前提。

---

## 4. 留給下一個人的一件事

**分支③與分支④不可以合併。**

兩者今天算出同一個刻度（`min(400, 1000)` 與 `1000` 都走同一個挑法），
看起來像是可以寫成一行。但它們說的是不同的事：

- 分支③ = 「**我的畫面**只擺得下這麼多」——關於呼叫端的事實。
- 分支④ = 「我對粗細**沒有意見**」——關於呼叫端沒有意見的事實。

差別會在單次上限與畫面預算脫鉤那天顯現（例如上限調到 5000）：
④應該跟著上限走，③不該。合併之後那次調整會安靜地改掉其中一個的意思，
而沒有任何測試會紅——因為今天兩者的答案相同。
