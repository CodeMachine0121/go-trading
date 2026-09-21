# 只做空這種交易模式 — Contract Verification

**Contract:** `.sdd/2026-09-21-short-only-trading-mode/PRD.md`（Section 3 驗收情境為 oracle）
**Design map:** `.sdd/2026-09-21-short-only-trading-mode/ARCH.md`
**Repos:** `go-trading`、`go-trading-frontend`、`go-trading-mcp`
**Method:** 靜態一致性稽核。每一條先從規格推出期望結果，再**各自獨立**判斷
「測試有沒有斷言那個結果」與「程式有沒有產出那個結果」。**不以測試綠燈作為判準。**

---

## Clauses

`T` = 測試稽核　`C` = 程式稽核

### US-01 交易模式有哪幾種

| ID | 條款 | Oracle | 實作 | 測試 | T | C | 狀態 |
|:---|:---|:---|:---|:---|:---|:---|:---|
| AC-01 | 存一份只做空的交易策略 | 存得起來，讀回來仍是只做空 | `trading_mode_domain.go:12`（可選清單） | `trading_mode_domain_test.go` ReadsWhatWasDeclared／"short only, spelled out" | asserts-oracle | produces-oracle | ✅ |
| AC-02 | 沒填仍是多空反手 | 讀作多空反手 | `trading_mode_domain.go:41` | 同上／"declaring nothing…" | asserts-oracle | produces-oracle | ✅ |
| AC-03 | 填認不得的值整份拒絕，列出四種 | 拒絕，且四種拼法都出現在那句話裡 | `trading_mode_domain.go:61` | RefusesWhatItCannotRead（四個 `Contains`） | asserts-oracle | produces-oracle | ✅ |
| AC-04 | 既有交易策略不會被自動改掉 | 仍是多空反手 | 無資料轉換；取值為字串 | AC-02 同一組（預設值未動） | asserts-oracle | produces-oracle | ✅ |
| AC-05 | 其餘三種照舊存得起來 | 存得起來，行為逐字相同 | `trading_mode_domain.go:12` | ReadsWhatWasDeclared 前三列 | asserts-oracle | produces-oracle | ✅ |

### US-02 倉位行為是槓桿做多的鏡像

| ID | 條款 | Oracle | 實作 | 測試 | T | C | 狀態 |
|:---|:---|:---|:---|:---|:---|:---|:---|
| AC-06 | 空手＋賣出 → 開空倉 | 以收盤價開空倉 | `TargetFor` → `BacktestAccountDomain.Apply` | `backtest_account_domain_test.go` UnderShortOnlyRules／"selling while flat opens a short" | asserts-oracle | produces-oracle | ✅ |
| AC-07 | 已持空＋賣出 → 當作沒聽到 | 倉位不動、不計開倉 | `backtest_account_domain.go:93` | 同上／"a repeated sell is heard once" | asserts-oracle | produces-oracle | ✅ |
| AC-08 | 持空＋買入 → 平倉回現金 | 平掉、錢回可用資金、之後空手、不開多倉 | `TargetFor`→Flat →`settleOpenPosition` | 同上／"buying closes the short back to cash…" | asserts-oracle | produces-oracle | ✅ |
| AC-09 | 空手＋買入 → 什麼都不發生 | 不開倉、次數不變、資金不變 | `Apply` 的 `!wantsPosition` 早退 | 同上／"buying while flat does nothing at all" ＋ `BuyingWhileFlatSplitsTheModes` | asserts-oracle | produces-oracle | ✅ |
| AC-10 | 持有 → 倉位不動 | 不動、不記交易 | `TargetFor`→Unchanged | `TargetFor`／"short only: holding…" | asserts-oracle | produces-oracle | ✅ |
| AC-11 | 交易明細每一筆都是做空 | 全部方向為做空 | 同 AC-06 | 同上／"no trade it ever books faces long" | asserts-oracle | produces-oracle | ✅ |
| AC-12 | 與變通辦法跑出同一張成績單 | 兩張成績單完全一樣 | 兩者產生同一串目標倉位 | 無——需要組出一份「永遠不成立的買入條件」的交易策略，那是交易策略層而非模式層的組裝 | no-test | produces-oracle | 🟡 partial |

### US-03 借得到錢與強制平倉

| ID | 條款 | Oracle | 實作 | 測試 | T | C | 狀態 |
|:---|:---|:---|:---|:---|:---|:---|:---|
| AC-13 | 只做空＋槓桿 3 → 接受 | 接受，曝險 3 倍，模擬強平 | `CanUseLeverage`（含只做空） | `CanUseLeverage`／"short only borrows…" | asserts-oracle | produces-oracle | ✅ |
| AC-14 | 槓桿留白 → 強平筆數 0 | 接受，強平 0 | `BacktestLeverageDomain` | `NeverLiquidatesAnUnborrowedShortOnlyPosition` | asserts-oracle | produces-oracle | ✅ |
| AC-15 | 槓桿 1 → 強平筆數 0 | 同上 | 同上 | 同上（留白與 1 走同一條路） | asserts-oracle | produces-oracle | ✅ |
| AC-16 | 槓桿 0.5 → 拒絕 | 拒絕並說不得小於一 | `BacktestLeverageDomain`（未改，與模式無關） | 既有槓桿測試（模式無關） | asserts-oracle | produces-oracle | ✅ |
| AC-17 | 現貨＋槓桿 3 → 拒絕 | 拒絕並說現貨借不到錢 | `BorrowingRefusal`（未改） | 既有 `BorrowingRefusal` 測試 | asserts-oracle | produces-oracle | ✅ |
| AC-18 | 只做空的強平價在進場價上方 | 高於進場價 | `BacktestPositionDomain`（依倉位方向） | `WipesOutAShortOnlyPositionOnTheWayUp`（斷言 119.5 > 進場 100） | asserts-oracle | produces-oracle | ✅ |
| AC-19 | 最高價越過強平價 → 強制出場、收回 0 | 強平出場、收回 0 | 同上 | 同上（出場原因＋最終權益 0） | asserts-oracle | produces-oracle | ✅ |
| AC-20 | 止損比強平近 → 止損先出場 | 止損出場 | 同上 | `TakesTheNearerExitAboveAShortOnlyEntry` | asserts-oracle | produces-oracle | ✅ |
| AC-21 | 沒上槓桿 → 強平筆數 0 | 0 | 同上 | `NeverLiquidatesAnUnborrowedShortOnlyPosition` | asserts-oracle | produces-oracle | ✅ |

### US-04 機器人存檔

| ID | 條款 | Oracle | 實作 | 測試 | T | C | 狀態 |
|:---|:---|:---|:---|:---|:---|:---|:---|
| AC-22 | 引用只做空＋槓桿 1.8 → 存得起來 | 存得起來 | `StrategyBotDomain` → `BorrowingRefusal` | `strategy_bot_domain_test.go`／"short only borrows because shorting is borrowing" | asserts-oracle | produces-oracle | ✅ |
| AC-23 | 引用只做空＋槓桿留白 → 存得起來 | 存得起來 | 同上 | 同上／"short only suggesting no leverage at all" | asserts-oracle | produces-oracle | ✅ |
| AC-24 | 引用只做空＋槓桿 0.5 → 拒絕 | 拒絕並說不得小於一 | 同上（與模式無關） | 既有測試 | asserts-oracle | produces-oracle | ✅ |
| AC-25 | 引用現貨＋槓桿 1.8 → 拒絕 | 拒絕並說現貨借不到錢 | 同上 | `strategy_bot_domain_test.go:326` | asserts-oracle | produces-oracle | ✅ |

### US-05 機器人訊息

| ID | 條款 | Oracle | 實作 | 測試 | T | C | 狀態 |
|:---|:---|:---|:---|:---|:---|:---|:---|
| AC-26 | 只做空＋賣出 → 寫「做空」 | 結論寫 `做空` | `HeadlineVerbFor` | `TellsAShortOnlyAccountToGetOutRatherThanGoLong`／sell | asserts-oracle | produces-oracle | ✅ |
| AC-27 | 只做空＋買入 → 寫「出場」，不寫做多也不寫買入 | 結論寫 `出場` | 同上 | 同上／buy（斷言整行相等） | asserts-oracle | produces-oracle | ✅ |
| AC-28 | 有槓桿 → 印保證金與名目 | 印 50000 與 150000 | `positionPlanLines`（未改） | 既有測試（模式無關） | asserts-oracle | produces-oracle | ✅ |
| AC-29 | 槓桿留白 → 不印那兩行，仍印交易模式 | 不印／印 | `positionPlanLines` + `ActNeedsTheModeNamed` | `NamesShortOnlyEvenWithNoMultiplier`（帶真實部位規劃） | asserts-oracle | produces-oracle | ✅ |
| AC-30 | 空倉止損寫「往上」 | 方向為往上 | `exitDirectionInWords(suggestsShort)` | `PutsAShortOnlyStopAbove` | asserts-oracle | produces-oracle | ✅ |
| AC-31 | 其餘三種模式訊息一字不改 | 逐字相同 | `HeadlineVerbFor` 前九格 | `HeadlineVerbFor` 全十二格 + 既有訊息測試 | asserts-oracle | produces-oracle | ✅ |
| AC-32 | 各來源怎麼說照舊 | 仍寫買入／賣出／持有 | `SignalDomain.InWords` | `QuotesTheScriptsUnderShortOnlyRules` | asserts-oracle | produces-oracle | ✅ |

### US-06 使用者挑得到

| ID | 條款 | Oracle | 實作 | 測試 | T | C | 狀態 |
|:---|:---|:---|:---|:---|:---|:---|:---|
| AC-33 | 畫面上四個選項 | 四個 | `trading-mode-vo.ts` `TRADING_MODES` | `trading-mode-domain.spec.ts`／"四種模式…" | asserts-oracle | produces-oracle | ✅ |
| AC-34 | 既有三個位置不變，只做空排最後 | 順序 longShort, spot, leveragedLong, shortOnly | 同上 | 同上（`toEqual` 整個陣列） | asserts-oracle | produces-oracle | ✅ |
| AC-35 | 助手聽懂「只想做空」 | 挑只做空 | `tool_catalog_strategy.go:117`、`trading_strategy_write_assistant_arguments.go:151` | `tool_catalog_test.go`／"只想做空"；assistant queries test | asserts-oracle | produces-oracle | ✅ |
| AC-36 | 助手不用變通辦法 | 不產生「永遠不成立的買入條件」寫法 | 同上（明文警告） | `tool_catalog_test.go`／"永遠不成立的買入條件" | asserts-oracle | produces-oracle | ✅ |
| AC-37 | 重演策略腳本認得只做空 | 接受 | 同一個建構子 | `TestReplayingAScriptSaysWhichModesItMayBeTold`（四種各驗定義句） | asserts-oracle | produces-oracle | ✅ |
| AC-38 | 重演策略腳本填錯列出四種 | 拒絕並列出四種 | `trading_mode_domain.go:61` | AC-03 同一測試 | asserts-oracle | produces-oracle | ✅ |

### Business Rules

| ID | 規則 | Oracle | 狀態 |
|:---|:---|:---|:---|
| BR-1 | 四選一、留白即多空反手、認不得整份拒絕 | 同 AC-01/02/03 | ✅ |
| BR-2 | 每一棒信號→目標倉位的四×三表 | `TargetFor` 十二格＋零值三格全測 | ✅ |
| BR-3 | 誰借得到錢（只做空借得到） | `CanUseLeverage` 四模式全測 | ✅ |
| BR-4 | 強平邊由倉位方向決定，不由模式決定 | `WipesOutAShortOnlyPositionOnTheWayUp` 以只做空進入並斷言強平在上方 | ✅ |
| BR-5 | 動詞由「做得了多／做得了空」兩個問題共同決定 | `HeadlineVerbFor` 十二格＋零值三格全測 | ✅ |
| BR-6 | 既有行為不變 | 前三模式九格＋既有訊息測試逐字未動 | ✅ |

### Non-Functional

| ID | 要求 | 狀態 |
|:---|:---|:---|
| NFR-1 | 效能：無新增要求 | ✅（僅多一個列舉取值） |
| NFR-2 | 安全：無新增要求 | ✅ |
| NFR-3 | 相容：既有資料不需轉換 | ✅（字串欄位、無 migration、無預設值變動） |
| NFR-4 | 分析追蹤：N/A | ✅ |

---

## Orphans

| 行為 | 位置 | 對應條款 | 判定 |
|:---|:---|:---|:---|
| `TradingModeDomain.ActNeedsTheModeNamed()` | `trading_mode_domain.go:142` | BR-5 的附帶條件（PRD「印交易模式那一行」） | **非 orphan**——它是 AC-29 與既有 AC-31 的實作處，重構時從呼叫端搬過來，行為逐字不變 |
| `SignalDomain.InWords()` | `signal_domain.go:52` | AC-32 | **非 orphan**——由訊息端搬家而來，行為一字未改 |
| `TargetFor` 對零值買入的答案由「持多」改為「不變」 | `trading_mode_domain.go:194` | 無條款 | **修正而非越界**——既有註解本就宣稱「未知模式什麼都不做」，舊程式與那句話矛盾且無測試。已補測試釘住 |

**Out of Scope 檢查**：PRD 列的六項（不理信號的開關、兩邊都做但不反手、自動改寫既有策略、
強平規矩、資金費率、真的下單）在三個 repo 都**找不到對應程式碼**，無越界。

---

## Summary

```
Contract verification complete for "只做空這種交易模式".
Oracle: PRD Acceptance Criteria — 38 AC + 6 BR + 4 NFR = 48 clauses.

✅ 47 conforms · 🔴 0 violations · 🟠 0 mis-asserted · 🟡 1 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 0 orphans
Conformance: 97.9%
```

**沒有 violation、沒有 gap、沒有 orphan。**

第一次稽核時有 8 條 mis-asserted、9 條 partial，形狀完全一致：只做空在
`TargetFor` 與訊息這兩層測得很密，但**帳戶層、強平層、機器人存檔層從來沒有以
只做空進入過一次**。那三層的程式碼這一刀一行都沒改，所以行為本來就是對的——
但「對」是靠推論，不是靠斷言。已補上：

- 帳戶層 6 個只做空案例（開空／同向忽略／買入平倉回現金／空手買入無事／明細全做空／與現貨分岔）
- 強平層 4 個（強平在上方、下跌不強平、止損比強平近、沒借錢不強平）
- 機器人存檔 2 個（1.8 倍、留白）
- 訊息 2 個（不上槓桿仍印交易模式、空倉止損寫往上）

補的過程中抓到自己寫的一條**虛假測試**：原本用沒有部位規劃的 round 去斷言
「不印名目」，那在任何邏輯下都會通過。已改成帶真實部位規劃、只把倍數關掉。

三個 mutation（賣出不開空倉、買入開多倉、交易模式那一行永不印）全部被新測試抓到。

### 唯一剩下的 partial

**AC-12「與變通辦法跑出同一張成績單」。** 它要組一份「多空反手＋永遠不成立的買入
條件」的交易策略來對照，而那是**交易策略層**的組裝，不是交易模式層的。
兩者產生同一串目標倉位，這一點由 `TargetFor` 的十二格真值表涵蓋；
真要釘死它，該寫在交易策略回測那一層，不在這裡。**刻意留著，不硬塞。**

**Ceiling:** 靜態稽核。它比對規格期望與程式路徑、並檢查測試是否斷言那個期望，
**不執行自己發明的情境**，也不以整套測試綠燈作為判準。
