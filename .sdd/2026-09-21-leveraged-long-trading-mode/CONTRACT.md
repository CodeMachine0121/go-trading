# 槓桿做多這種交易模式 — Contract Verification

**Contract:** `.sdd/2026-09-21-leveraged-long-trading-mode/PRD.md`（Section 3 Acceptance Criteria 為 oracle）
**Design map:** `ARCH.md`
**Repos:** `go-trading`（後端，路徑無前綴）· `go-trading-frontend`（前綴 `FE:`）· `go-trading-mcp`（前綴 `MCP:`）

**Ceiling:** 這是一次**靜態**符合性稽核。它比對「規格推導出的預期結果」與「測試斷言的東西」
與「程式實際走出來的結果」，**不執行自己發明的情境**，也不以整份測試套件的綠燈作為判準。

---

## Clauses

### US-01 挑得到這個模式

| ID | Clause | Oracle（由規格推導） | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-01 | 建立一份槓桿做多的交易策略 | 存得起來，讀回來的交易模式是槓桿做多 | `trading_mode_domain.go:12-16`（可選清單）＋ `trading_strategy_domain.go` 委派 | `trading_strategy_domain_test.go`「declared long only but borrowing」（本次補上） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-02 | 現貨照舊 | 存得起來，讀回來是現貨 | 同上 | `trading_strategy_domain_test.go:322`「declared spot」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-03 | 多空反手照舊 | 存得起來，讀回來是多空反手 | 同上 | `trading_strategy_domain_test.go:327` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-04 | 沒填仍然是多空反手 | 讀回來是多空反手 | `trading_mode_domain.go:41-43` | `trading_strategy_domain_test.go:333`、`trading_mode_domain_test.go`「declaring nothing」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-05 | 認不得的值整份拒絕，並說出三種 | 整份拒絕；拒絕的話同時出現三個選項 | `trading_mode_domain.go:51-62` | 兩層皆斷言三個（`trading_strategy_domain_test.go` 於本次補上第三個） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-06 | 既有現貨交易策略不被自動改掉 | 讀它仍然是現貨 | **無遷移程式碼**（刻意） | 無 | `no-test` | `produces-oracle`（沒有任何寫入路徑會改它） | 🟡 partial |
| AC-07 | 把既有的改成槓桿做多 | 存得起來，讀回來是槓桿做多 | 修改路徑與建立共用同一個建構子 | 無專屬案例 | `no-test` | `produces-oracle` | 🟡 partial |

### US-02 倉位行為與現貨相同

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-08 | 空手買入開多倉 | 以收盤價開多倉 | `trading_mode_domain.go:151-153` | `trading_mode_domain_test.go`「long only but borrowing: buying asks to be long」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-09 | 已多倉再買忽略 | 倉位不動、不計開倉 | `backtest_account_domain.go`（模式無關，既有） | 既有帳戶測試（模式無關） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10 | 持多賣出平倉回現金 | 平掉多倉、錢回可用資金、之後空手、不開空倉 | `trading_mode_domain.go:170-171`（回 `Flat`） | `trading_mode_domain_test.go`「selling asks to be in cash」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-11 | 空手賣出什麼都不發生 | 不開空倉、不計開倉、資金不變 | 同上（`Flat` ＋ 既有帳戶邏輯） | 模式層 `asserts-oracle`；帳戶層沿用既有現貨案例 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-12 | 從頭到尾不會出現空倉 | 交易明細每一筆都是做多 | `TargetFor` 永不回 `Short` | 無重演層案例；Postman「重演：槓桿做多借得到錢」有斷言 | `asserts-oracle`（Postman） | `produces-oracle` | ✅ conforms |
| AC-13 | 不上槓桿時與現貨成績單相同 | 兩張成績單每個數字相同、明細與曲線相同 | 由 `TargetFor` 兩者同一行保證 | **無** | `no-test` | `produces-oracle` | 🟡 partial |

### US-03 只有借得到錢的才開得了槓桿

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-14 | 槓桿做多開得了槓桿（3 倍） | 跑完並交出成績單；曝險 3 倍 | `trading_mode_domain.go:100-103`、`backtest_leverage_domain.go:93-96` | `backtest_leverage_domain_test.go`「LetsLeveragedLongBorrow」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-15 | 現貨仍然被拒 | 整份拒絕；說現貨借不到錢 | `trading_mode_domain.go:115-122` | `backtest_leverage_domain_test.go`「spot has nobody to borrow from」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-16 | 多空反手照舊 | 跑完並交出成績單 | 同上 | `trading_mode_domain_test.go` CanUseLeverage 表 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-17 | 槓桿做多但留白 | 跑完；強平筆數 0 | `leverage_multiplier_domain.go:46-48` | `TestBacktestLeverageHoldsLeveragedLongToEveryOtherRule` 未涵蓋留白；`LeverageMultiplier` 表涵蓋「0 → 不借錢」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-18 | 槓桿做多但填一 | 跑完；強平筆數 0 | `leverage_multiplier_domain.go:54-56` | `leverage_multiplier_domain_test.go`「exactly one」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-19 | 槓桿做多但 0.5 | 整份拒絕；說不得小於一 | `leverage_multiplier_domain.go:50-52` | `TestBacktestLeverageHoldsLeveragedLongToEveryOtherRule`「half a times」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-04 強平算法一字不差

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-20 | 強平價 80.5 | 強平價 80.5 | `backtest_leverage_domain.go` `AdverseDistance`（未改） | `backtest_leverage_domain_test.go`「LetsLeveragedLongBorrow」斷言距離 19.5 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-21 | 跌到強平收回零 | 在 80.5 強制出場、收回 0、原因寫強平出場 | 既有模擬（未改） | 既有 `backtest_leveraged_simulation_test.go`（多空反手多倉）；槓桿做多無專屬案例 | `shallow`（未以新模式走過一次） | `produces-oracle`（同一段程式，模式只決定 `TargetFor`） | 🟠 mis-asserted |
| AC-22 | 止損比強平近則止損先 | 在 95 出場、原因止損出場 | 既有（未改） | 既有案例（模式無關） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-23 | 強平筆數 2 | 成績單強平出場筆數 2 | 既有（未改） | 既有案例（模式無關） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-24 | 沒槓桿強平 0 | 強平出場筆數 0 | 既有（未改） | Postman「重演：槓桿做多借得到錢」＋既有案例 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-25 | 沒有上方的強平價 | 每個強平價都在進場價下方 | 由 `TargetFor` 永不產生空倉保證 | 無直接案例（由 AC-12 間接涵蓋） | `shallow` | `produces-oracle` | 🟠 mis-asserted |

### US-05 機器人的槓桿受限於那份規則

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-26 | 現貨+1.8 整台拒絕，措辭同重演 | 整台拒絕；與重演一字不差的那句話 | `strategy_bot_domain.go:123-146` | `strategy_bot_domain_test.go`「RefusesBorrowingRulesCannotDo」＋`strategy_bot_application_test.go`「CreateRefusesBorrowing…」 | `asserts-oracle`（逐字斷言那句話） | `produces-oracle` | ✅ conforms |
| AC-27 | 槓桿做多+1.8 存得起來 | 存得起來，槓桿 1.8 | 同上 | 同上（domain 表＋application 存下 1.8） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-28 | 多空反手+1.8 照舊 | 存得起來 | 同上 | domain 表 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-29 | 現貨+留白 存得起來 | 存得起來 | `positionPlan.IsBorrowed()` 為假 | domain 表「spot suggesting no leverage at all」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-30 | 現貨+填一 存得起來 | 存得起來 | 同上 | domain 表「spot suggesting one times」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-31 | 0.5 照舊被擋 | 整台拒絕；說不得小於一 | `leverage_multiplier_domain.go:50-52` | `strategy_bot_domain_test.go`（既有 0.5 案例） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-32 | 既有機器人照常跑 | 照常跑完那一輪並送訊息，不被停也不被刪 | `strategy_bot_service.go:339-345` `PlanRoundPosition` **不設關卡** | `strategy_bot_run_application_test.go`「KeepsRunningABotSavedBeforeBorrowingWasGated」（本次補上，走完整一輪：現貨規則＋1.8 倍） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-33 | 再存一次才擋下來 | 整台拒絕；該機器人維持原狀 | 同 AC-26（存檔路徑） | 由 AC-26 涵蓋建立側；修改側無專屬案例 | `shallow` | `produces-oracle`（建立與修改共用建構子） | 🟠 mis-asserted |
| AC-34 | 改成槓桿做多後存得起來 | 存得起來 | 同 AC-27 | Postman「同一台機器人，改用槓桿做多的規則就存得起來」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-06 訊息講出借錢這件事

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-35 | 買入動詞＋印模式行 | 動詞「買入」；有一行寫交易模式是槓桿做多 | `strategy_bot_message_domain.go:157-158` | `TestStrategyBotMessageTellsABorrowingLongAccountToBuyRatherThanGoLong`＋`NamesTheModeWhereTheVerbDoesNotSayEverything` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-36 | 賣出動詞是出場 | 動詞「出場」，不是「做空」 | `strategy_bot_message_domain.go:97-112`（問 `CanGoShort`） | `TestStrategyBotMessageStillTellsABorrowingLongAccountToGetOut` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-37 | 印保證金 50000 與名目 150000 | 兩行都印出來 | `positionPlanLines()`（未改） | 既有案例（模式無關，槓桿 3 倍） | `shallow`（未以槓桿做多走過一次） | `produces-oracle` | 🟠 mis-asserted |
| AC-38 | 留白不印那兩行、模式行照印 | 不印槓桿與名目；仍有模式行 | 同上＋新條件 | **無**（兩半各有案例，未合在一起） | `no-test` | `produces-oracle` | 🟡 partial |
| AC-39 | 現貨訊息一字不變 | 動詞買入／出場；沒有模式行 | 條件為假 | `NamesTheModeWhereTheVerbDoesNotSayEverything` 的 NotContains | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-40 | 多空反手一字不變 | 動詞做多／做空；有模式行 | 條件為真 | 同上＋既有案例 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-41 | 來源段照樣寫買入賣出持有 | 每一行仍是三種信號之一 | `strategy_bot_message_domain.go:162-171`（未改） | `TestStrategyBotMessageStillTellsABorrowingLongAccountToGetOut` 末行 | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-07 三條路都挑得到

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-42 | 畫面上三個選項 | 看得到多空反手、現貨、槓桿做多 | `FE: trading-mode-vo.ts` `TRADING_MODES` → 兩個 service → `TradingStrategyWorkbench.vue:135` `v-for` | `FE: trading-mode-domain.spec.ts`「三種模式…」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-43 | 助手挑得到槓桿做多 | 那份交易策略的交易模式是槓桿做多 | `trading_strategy_write_assistant_arguments.go:144-156`、`MCP: tool_catalog_strategy.go:96-115` | `trading_strategy_assistant_queries_test.go`（讀出 enum 比對三值）＋`MCP: tool_catalog_test.go` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-44 | 腳本重演指定得了槓桿做多 | 跑完並交出成績單 | `trading_mode_domain.go:12-16`（那條路讀同一份清單） | Postman「重演：槓桿做多借得到錢」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-45 | 腳本重演拒絕時列出三種 | 拒絕的話同時出現三個選項 | `trading_mode_domain.go:51-62` | `trading_mode_domain_test.go` 斷言三個 | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### Business Rules

| ID | Clause | Oracle | Implementation | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|
| BR-1 | 三種合法組合、第四種不存在 | 模式恰為三個 | `vo/trading_mode_vo.go`、`trading_mode_domain.go:12-16` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| BR-2 | 沒填就是多空反手 | 同 AC-04 | `trading_mode_domain.go:41-43` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| BR-3 | 認不得的值拒絕並說出三種 | 同 AC-05 | 同上 | `asserts-oracle`（見 AC-05） | `produces-oracle` | ✅ conforms |
| BR-4 | 借不借得到錢，重演與機器人共用同一句 | 兩邊逐字相同 | `trading_mode_domain.go:115-122` 為唯一出處 | `asserts-oracle`（機器人測試逐字斷言那句話） | `produces-oracle` | ✅ conforms |
| BR-5 | 留白／零／一皆不算借錢 | 三種模式皆然、不適用 BR-4 | `leverage_multiplier_domain.go` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| BR-6 | 強平規矩全部沿用、一條不新增 | 既有數字逐字不變 | `backtest_leverage_domain.go` 算式未動 | `asserts-oracle`（既有套件全綠） | `produces-oracle` | ✅ conforms |
| BR-7 | 訊息寫法按兩個問題分開決定 | 三種模式的動詞與模式行如表 | `strategy_bot_message_domain.go` | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### Non-Functional

| ID | Clause | Oracle | Status | 說明 |
|---|---|---|---|---|
| NFR-1 | 無新增效能要求；存檔多讀一份本來就指名的交易策略 | 不新增外部查詢 | ✅ conforms | `tradingModeOfOwnedTradingStrategy` 沿用**既有那一次**查詢，只是不再丟掉結果 |
| NFR-2 | 擁有權規則不因多讀一次而放寬 | 讀到別人的仍回「找不到」 | ✅ conforms | `strategy_bot_application.go:170-190`；既有 `CreateRefusesATradingStrategyThisPersonCannotSee` 仍綠 |
| NFR-3 | 既有的每一份策略／機器人／重演行為與數字逐字不變 | 既有套件全綠、無數字變動 | 🟠 mis-asserted | **一處刻意的例外**，見 Orphans O-1 |

---

## Orphans

| ID | 行為 | 對應條款 | 判定 |
|---|---|---|---|
| O-1 | 存機器人時，**負數**槓桿倍數從「悄悄讀成一倍並存下」改為「整台拒絕」（`leverage_multiplier_domain.go:50-52`） | **無** — PRD 沒有這一條，且與 NFR-3「既有行為逐字不變」相抵觸 | ⚠️ **需使用者裁決**。這是 `/improve-codebase` 合併兩份重複規則時浮現的既有不一致：重演一向拒絕負數，存機器人一向接受。合併後只能有一個答案，選了拒絕（負倍數不存在，且前端本來就擋）。它由 `TestNewStrategyBotDomainRefusesANegativeMultiplierRatherThanReadingItAsOne` 釘住。**若要維持 NFR-3 字面，需改為在 PRD 補一條**，而不是退回舊行為 |
| O-2 | 新增共用模型 `LeverageMultiplierDomain` | 無條款（純內部重構，對外行為除 O-1 外不變） | ✅ 良性 orphan，`ARCH.md` §6「下一個需求」已預告此類收斂 |
| O-3 | 前端 `backtest-leverage-domain.ts` 由比對 `'spot'` 改為問 `canUseLeverage()` | 無條款（等價改寫） | ✅ 良性。**已知為等價突變**：目前三種模式下兩種寫法行為完全相同，突變測試無法區分。保留它的理由是 `ARCH.md` §6 第 1 條規範，不是行為差異 |

**Out of Scope 檢查**：PRD 列的六項（只做空能借錢的模式、拆成兩個設定、自動遷移既有策略、回頭作廢既有機器人、資金費率、強平規矩改動）**皆未出現**對應程式碼。無越界。

---

## Summary

```
Contract verification complete for "槓桿做多這種交易模式".
Oracle: PRD Acceptance Criteria — 45 AC + 7 BR + 3 NFR = 55 clauses.

✅ 45 conforms · 🔴 0 violations · 🟠 5 mis-asserted · 🟡 5 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 3 orphans
Conformance: 81.8%（無任何行為違規；扣分全部來自測試強度）

Violations:   （無）
Mis-asserted: AC-21, AC-25, AC-33, AC-37, NFR-3
Partial:      AC-06, AC-07, AC-13, AC-38
Orphans:      O-1 需裁決 · O-2 O-3 良性
```

### 本輪已修（第二次稽核）

| ID | 原判定 | 修法 | 現判定 |
|---|---|---|---|
| AC-01 | 🟠 | 交易策略層補一條「存得進槓桿做多」 | ✅ |
| AC-05 / BR-3 | 🟠 | 交易策略的拒絕改為逐一斷言三個選項（原本只斷言兩個，第三個沒被提供也照樣綠） | ✅ |
| AC-32 | 🟡 | 補一條走完整一輪的案例：現貨規則＋1.8 倍的既有機器人照常跑完、照常送訊息、照常留紀錄 | ✅ |

三條都做過突變驗證：關掉對應行為，對應測試確實轉紅。

### 第三次稽核：程式碼審查補上的四個洞

Code review 在 PR 上找到四處 — **三處是條款真的沒被實現，稽核第一輪沒抓到，
因為它們都在「規格沒有明說、但條款要成立就必須有」的縫裡。**

| 發現 | 影響的條款 | 為什麼第一輪漏掉 | 修法 |
|---|---|---|---|
| 助手的 system prompt 仍只列兩種模式，還寫著「不能放空就給 spot」 | **AC-43**（助手挑對模式）實際上是假的 | 稽核只追到工具 schema，沒追到**另一個**告訴助手模式的地方。而腳本重演那條路，system prompt 是**唯一**的出處 | 三種模式與情境都寫進 prompt；新測試從模式清單推導，並斷言每種的**區別描述**（只斷言拼法會被旁邊的句子矇混過去） |
| 交易模式關卡可繞過：建機器人用槓桿做多＋3 倍 → 停掉 → 把策略改成現貨 → 啟動 | **AC-26/US-05 的意圖**被架空 | 稽核逐條比對 AC，而這是**條款之間的縫**：沒有任何一條 AC 描述「改策略」這個動作 | 改交易模式時，若有機器人正在建議槓桿即拒絕（`ErrTradingStrategyBotBorrowing`），並點名那幾台 |
| MCP 腳本重演的交易模式欄位從未列出任何取值 | **AC-44**（腳本重演指定得了槓桿做多）經 MCP 這條路不成立 | 既有弱點，不是本刀造成，所以 diff 看不到 | 三種取值與區別寫進欄位說明；測試同樣斷言區別描述 |
| 沒有部位規劃的機器人開始存下 `leverage: 1`（原本 `0`） | **NFR-3**（既有行為逐字不變）的實際違反 | 這是 `/improve-codebase` 之後才出現的回歸，第一輪稽核在它之前 | `ToSettingsDto` 對沒有資金的計畫回傳全零 |

四處都做過突變驗證。前三處各補了**從清單推導、斷言區別描述**的測試——
與 AC-05 同一個教訓：**一個只斷言拼法的 Contains，在旁邊的散文裡照樣是綠的。**

**未修的 5 🟠 + 4 🟡 皆為補強性質**——它們的程式行為正確，
且都由「模式無關」的既有案例實際走過（強平算式、訊息版面），
只是沒有以槓桿做多的身分再走一次。判斷為可接受的殘餘風險：
那幾段程式碼這一刀一行都沒改，而 `TargetFor` 已保證新模式只產生多倉。

**整體判讀：沒有任何一條的程式行為是錯的**（0 violations、0 gaps）。
所有扣分都是同一類問題：**新模式在幾條路徑上沿用了「模式無關」的既有測試，
沒有自己走過一次**，於是那些綠燈證明不了新模式真的走得通。

**建議修正順序**

1. **AC-05 / BR-3**（最該修）：`trading_strategy_domain_test.go:362` 只斷言兩個選項，
   第三個沒被提供也照樣綠 — 這正是這整刀要消滅的那種「綠得不誠實」。
2. **AC-01**：補一條「交易策略存得進槓桿做多」的案例。
3. **AC-32**：補一條「既有現貨＋1.8 倍的機器人照常跑完一輪」 — 它是 PRD 明訂的承諾，
   目前完全沒有測試釘著，而它正是使用者現在線上那兩台的處境。
4. **O-1**：負數槓桿那條要你裁決（見上表）。
5. 其餘（AC-13、AC-21、AC-25、AC-33、AC-37、AC-38）為補強性質。
