# 回測的交易模式 — Contract Verification Matrix

**Contract source:** `.sdd/2026-09-17-backtest-trading-mode/PRD.md`（Acceptance Criteria 為 oracle）
**Design map:** `.sdd/2026-09-17-backtest-trading-mode/ARCH.md`
**Glossary:** `.sdd/UL-MAP.md`
**Verified:** 2026-09-17
**Ceiling:** 靜態一致性稽核。逐條把**測試斷言**與**程式路徑**各自對照規格推出的 oracle，
不以「跑完全套變綠」當判準，也不自行發明並執行新的情境。

---

## Clauses

### US-01 — 發動回測時說得出這次用哪一種交易模式

| ID | Clause | Oracle（由規格推出） | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01 | 指定多空反手 | 這次回測照多空反手重演 | `trading_mode_domain.go:45` → `TargetFor:71` | `trading_mode_domain_test.go:12`、`backtest_simulation_domain_test.go:382` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 指定現貨 | 這次回測照現貨重演 | `trading_mode_domain.go:45` → `:78` | `trading_mode_domain_test.go:12`、`backtest_simulation_domain_test.go:365` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 完全沒說交易模式 | 照多空反手重演，與切片之前一字不差 | `trading_mode_domain.go:40` | `trading_mode_domain_test.go:12`、`backtest_domain_test.go:323` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 交易模式留白 | 照多空反手重演 | `trading_mode_domain.go:39-41`（先 TrimSpace） | `trading_mode_domain_test.go:12`（`"   "`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 認不得的值 | 整次拒絕、說出只能是多空反手或現貨、不回傳任何部分結果 | `trading_mode_domain.go:55` + `backtest_errors.go` 的 `tradingMode` 欄位 | `trading_mode_domain_test.go:55`、`backtest_controller_test.go:306`、助手 `:424`（斷言 outcome 為空） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 重演一份交易策略時同樣說得出來 | 照現貨重演 | `trading_strategy_backtest_domain.go:65` → 同一個 `NewBacktestDomain` | `backtest_domain_test.go:375`、助手 `:382` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 交易策略回測認不得的值同樣被拒，理由一字不差 | 拒絕，且措辭與重演一支策略腳本完全相同 | 同上（同一道關卡，非第二套規則） | `backtest_domain_test.go:356`（直接斷言兩個 `Error()` 字串相等） | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 — 現貨模式：買入只開多倉

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-08 | 空手時買入就開多倉 | 以該棒收盤價開一個多倉 | `backtest_account_domain.go:96-105` | `backtest_account_domain_test.go:146` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 已持多再買入等於沒聽到 | 倉位不動、開倉次數不增、可用資金不變 | `backtest_account_domain.go:69` | `backtest_account_domain_test.go:156` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 押不下去的那次開倉被跳過 | 維持空手、不算失敗、開倉次數不增 | `backtest_account_domain.go:90-93` | `backtest_account_domain_test.go:234` | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 — 現貨模式：賣出就平倉回現金

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-11 | 持有多倉時賣出就平倉 | 以該棒收盤價平掉、可用資金加回、之後空手、交易明細多一筆做多且損益正確 | `trading_mode_domain.go:78` → `backtest_account_domain.go:73-81` | `backtest_account_domain_test.go:168`（方向／兩端價格／損益）＋`:184`（現金） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 平倉之後手上是真的現金 | 資金曲線點不隨價格變動 | `backtest_account_domain.go:110`（空手即純現金） | `backtest_account_domain_test.go:184`、`backtest_simulation_domain_test.go:365`（曲線末兩點相等） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 賣出只平倉，不開新倉 | 這一棒沒有開任何新倉位 | `backtest_account_domain.go:83-85`（flat 出口） | `backtest_account_domain_test.go:168`（開倉次數停在 1） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 空手時聽到賣出什麼都不發生 | 不開空倉、開倉次數不增、交易明細不增、可用資金不變 | `backtest_account_domain.go:68`（無倉即跳過平倉）＋`:83` | `backtest_account_domain_test.go:196`、`:261` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 第一棒就說賣出 | 什麼都不發生，資金曲線點是初始資金 | 同 AC-14 | `backtest_account_domain_test.go:196`（全新帳戶第一次就賣） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 買進賣出再買進 | 開倉次數 2、交易明細 1 筆、最後那個還開著 | `backtest_account_domain.go:58-105` | `backtest_account_domain_test.go:218` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 現貨模式下永遠不會出現空倉 | 交易明細每一筆方向都是做多；結束時還開著的也是做多 | `trading_mode_domain.go:76-79`（現貨永不回傳持空） | `trading_mode_domain_test.go:71`（現貨三種信號的目標表）＋`backtest_account_domain_test.go:218`（逐筆斷言 long） | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 — 同一段歷史，兩種模式給出不同的成績單

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-18 | 現貨模式錯過了空頭那一段 | 最後剩 12,000；交易明細 1 筆做多 100 進 120 出 | `backtest_simulation_domain.go:57-63` | `backtest_simulation_domain_test.go:365` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 多空反手吃到了空頭那一段 | 最後剩 15,000（反手押的是平倉後的 12,000）；結束時還開著一個 120 進場的空倉 | 同上 | `backtest_simulation_domain_test.go:382` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 沒有任何賣出時兩種模式完全一樣 | 成績單、交易明細、資金曲線三者皆相同 | `trading_mode_domain.go:72-74`（買入／持平兩模式同解） | `backtest_simulation_domain_test.go:397`（三者逐一相等） | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 — 多空反手的行為一個字都不能變

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-21 | 空手時賣出就開空倉 | 以該棒收盤價開一個空倉 | `trading_mode_domain.go:81` → `backtest_account_domain.go:96` | `backtest_account_domain_test.go:261`（與現貨並排對照） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 持有多倉時賣出就在同一棒反手 | 平多倉、同一棒同一價開空倉、開倉次數 +1 | `backtest_account_domain.go:73-105` | `backtest_account_domain_test.go:80`（切片前既有，未改動） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 已持有空倉時再說賣出等於沒聽到 | 倉位不動、開倉次數不增 | `backtest_account_domain.go:69` | `backtest_simulation_domain_test.go:113`（切片前既有，未改動） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 持平在兩種模式下都什麼都不做 | 什麼都不發生 | `backtest_account_domain.go:62` | `backtest_account_domain_test.go:59`（多空反手）＋`:247`（現貨，持多時持平） | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 — 助手替我回測時說得出交易模式

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-25 | 我告訴助手我的帳戶不能放空 | 照現貨重演；交回助手的每一筆交易方向都是做多 | `trading_strategy_backtest_assistant_query.go:39`、`:57` | 助手測試 `:382` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 助手沒說交易模式 | 照多空反手重演，與使用者自己發動時一字不差 | `:39`（不在 required）→ 空字串 → `trading_mode_domain.go:40` | 助手測試 `:404`（斷言 15,000／開倉 2） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 助手說了一個認不得的交易模式 | 整次拒絕、理由交回助手、不拿到任何部分結果 | 同一道關卡 → `trading_mode_domain.go:55` | 助手測試 `:424`（斷言 error 且 outcome 為空） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 助手看得懂兩種模式差在哪 | 說明講得出兩種模式的差別與預設值 | `:163`（Description）、`:183`（schema enum） | 助手測試 `:451`（含 enum、兩種拼法、且不在 required） | asserts-oracle | produces-oracle | ✅ conforms |

### Core Business Rules（PRD §4）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| BR-01 | 交易模式二選一，沒有第三種 | 只有多空反手與現貨兩個取值 | `trading_mode_domain.go:12-15`、`trading_mode_vo.go` | `trading_mode_domain_test.go:12`、`:71` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 沒說即多空反手 | 留白、未填一律讀作多空反手 | `trading_mode_domain.go:40` | `trading_mode_domain_test.go:12` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 認不得整次拒絕，不默默退回預設 | 拒絕並列出兩種，絕不 fallback | `trading_mode_domain.go:55` | `trading_mode_domain_test.go:55` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-04 | 同一棒最多平一次、開一次 | 一棒內不會開兩個倉 | `backtest_account_domain.go:58-105`（單一直線路徑） | `backtest_account_domain_test.go:168`（現貨只平）＋`:80`（反手平一開一） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 平倉的錢怎麼回來，兩種模式一樣 | 可用資金加回押注金額與該筆損益 | `backtest_account_domain.go:76-79`（兩模式共用） | `backtest_account_domain_test.go:184`（現貨 11,000）＋`:80`（反手 11,000） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 現貨模式永遠不產生空倉 | 交易明細每一筆都是做多 | `trading_mode_domain.go:76-79` | `trading_mode_domain_test.go:71`、`backtest_account_domain_test.go:218` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 其餘一律沿用（收盤價成交、單一倉位、押多少、不自動平倉、成績單／曲線／明細／打架棒數） | 這些行為與切片前完全相同 | `backtest_position_domain.go`、`backtest_equity_curve_domain.go`、`position_sizing_domain.go` 皆未改動 | 切片前既有測試全數未改動仍綠；`backtest_simulation_domain_test.go:397` 的兩模式全等交叉驗證 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 三條路認得同一組交易模式，措辭一字不差 | 只有一處實作，不是三處各自對答案 | 三個入口皆匯入 `backtest_domain.go:85` | `backtest_domain_test.go:356`（字串相等）＋控制器 `:306`／`:248`＋助手 `:424` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-09 | 交易模式跟著這一次走，不寫回策略腳本或交易策略 | 任何 entity 都不新增這個欄位，不落地 | 只出現在 request DTO 與 domain model；`entities/` 完全未改動 | — | no-test | produces-oracle | 🟡 partial |

### Edge Cases（PRD §4）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| EC-01 | 現貨整段一次賣出都沒有 | 與多空反手結果完全相同 | `trading_mode_domain.go:72-74` | `backtest_simulation_domain_test.go:397` | asserts-oracle | produces-oracle | ✅ conforms |
| EC-02 | 現貨整段一次買入都沒有（全是賣出與持平） | 一筆交易都沒有，資金曲線恆為初始資金，**不是錯誤** | `backtest_account_domain.go:68`＋`:83` | `backtest_account_domain_test.go:196`（單棒層級） | shallow（無「整段重演」層級的斷言） | produces-oracle | 🟡 partial |
| EC-03 | 現貨賣出的下一棒立刻又買入 | 兩棒兩件事，中間那一瞬間是空手 | `backtest_account_domain.go:58-105` | `backtest_account_domain_test.go:218` | asserts-oracle | produces-oracle | ✅ conforms |
| EC-04 | 現貨平倉後可用資金不夠再開下一個倉（固定金額） | 跳過那次開倉，維持空手，重演繼續 | `backtest_account_domain.go:90-93` | `backtest_account_domain_test.go:234`（從頭就空手，非「平倉之後」） | shallow（未覆蓋平倉後那條路徑） | produces-oracle | 🟡 partial |
| EC-05 | 交易模式的大小寫與使用者打的不一樣 | 比照倉位大小模式的既有寬容度 | `trading_mode_domain.go:45`（`EqualFold`，與 `position_sizing_domain.go` 同一手法） | `trading_mode_domain_test.go:12`（`"SPOT"`） | asserts-oracle | produces-oracle | ✅ conforms |
| EC-06 | 現貨結束時還開著一個多倉 | 不自動平掉；不進交易明細，但算進最後剩多少與開倉次數 | `backtest_simulation_domain.go`（未改動）＋`backtest_account_domain.go:110` | `backtest_account_domain_test.go:218`（開倉 2／明細 1） | shallow（未斷言「最後剩多少含它」） | produces-oracle | 🟡 partial |

### Non-Functional（PRD §6）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| NFR-01 | 相容：沒說交易模式的既有呼叫，答案與切片前完全相同 | 同樣的成績單、交易明細、資金曲線 | `trading_mode_domain.go:40` | 切片前既有測試全數未改斷言仍綠；`backtest_domain_test.go:323`、助手 `:404` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-02 | 效能：重演成本不因交易模式增加 | 每棒多的只是一次常數時間判斷 | `trading_mode_domain.go:71`（無迴圈、無配置） | — | no-test | produces-oracle | 🟡 partial |
| NFR-03 | 安全：不改變任何既有可見性規則 | 看不到的交易策略仍然回覆「找不到」 | 可見性相關程式碼完全未改動 | 助手測試 `:256`（切片前既有，仍綠） | asserts-oracle | produces-oracle | ✅ conforms |

---

## Orphans

| # | Behavior | Site | Explained by | Judgement |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `TradingModeDomain.Value()` 這個對外讀取器，生產程式碼沒有任何呼叫端，只有測試在用 | `trading_mode_domain.go:60` | 無條款要求把模式讀回來 | ⚠️ 良性。與 `PositionSizingDomain.Mode()`（`position_sizing_domain.go:93`）完全同一個狀況，是切片前就有的慣例；一併保留或一併移除，不單獨處理 |
| 2 | `dto.TradingStrategyBacktestRequestDto.ToBacktestRequestDto` 這個新的轉換 | `trading_strategy_backtest_request_dto.go:63` | 無 PRD 條款；由 `ARCH.md` §2 記載 | ⚠️ 良性。重構產物，不新增任何對外行為；其正確性由 `backtest_domain_test.go:393` 逐條件釘住 |
| 3 | 移除了 `SignalDomain.WantedDirection()` | `signal_domain.go` | 無條款要求它存在 | ✅ 正確的移除。「信號想要哪個方向」現在是交易模式的答案，留兩個答案就是漂移的源頭 |

**Out of Scope 檢查**：PRD §1 列的六項（只做空的模式、算式說平倉、結束時強制平倉、
同次比較兩模式、把交易模式存起來、助手自己猜模式）**沒有任何一項有對應程式碼**。
特別確認：`entities/` 一個欄位都沒加（BR-09）、`SignalVo` 仍然只有三個取值、
`BacktestSimulationDomain.ToDto` 走完之後沒有任何強制平倉步驟。無越界。

---

## Summary

| Status | Count |
| :--- | :--- |
| ✅ conforms | 41 |
| 🔴 violation | 0 |
| 🟠 mis-asserted | 0 |
| 🟡 partial | 5 |
| ❌ gap | 0 |
| ❔ unclear | 0 |
| ⚠️ orphan | 2（皆良性）＋1 正確移除 |

**Clauses:** 46 · **Conformance:** 89%（41/46 完全一致；其餘 5 條程式行為正確、僅測試未釘到該層級，無任何行為錯誤）

### 值得記下來的一件事

規格與程式碼第一次對不上時，**錯的是規格**：PRD 的 US-04 原本寫多空反手最後剩 14,000，
而全押反手押下去的是**平倉後的 12,000** 而不是原本的 10,000，所以應該是 15,000。
oracle 先釘、再讀程式碼的順序，是這個錯被抓到的唯一原因——
若照著程式碼寫測試，這個數字會一路綠到上線，而 PRD 會永遠錯下去。
規格已更正（見 `docs(backtest): correct the reversal arithmetic in the agreed examples`）。
