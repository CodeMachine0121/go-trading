# 交易策略的交易模式 — Contract Verification Matrix

**Contract source:** `.sdd/2026-09-18-trading-strategy-trading-mode/PRD.md`（Acceptance Criteria 為 oracle）
**Design map:** `.sdd/2026-09-18-trading-strategy-trading-mode/ARCH.md`
**Glossary:** `.sdd/UL-MAP.md`
**Verified:** 2026-09-18
**Ceiling:** 靜態一致性稽核。逐條把**測試斷言**與**程式路徑**各自對照規格推出的 oracle，
不以「跑完全套變綠」當判準，也不自行發明並執行新的情境。

---

## Clauses

### US-01 — 一份交易策略記得它是寫給哪一種帳戶的

| ID | Clause | Oracle（由規格推出） | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01 | 建立一份現貨交易策略 | 存起來，讀回來是現貨 | `trading_strategy_domain.go:74` → `:116` → `trading_strategy.go:43` | `trading_strategy_domain_test.go:314`、`trading_strategy_repository_test.go:285`、`trading_strategy_controller_test.go:359` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 建立一份多空反手交易策略 | 讀回來是多空反手 | 同上 | `trading_strategy_domain_test.go:314`（`longShort` 一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 完全沒填交易模式 | 存成多空反手，與回測預設值一字不差 | `trading_mode_domain.go:40`（未改動的空字串分支，兩處共用） | `trading_strategy_domain_test.go:314`（`""` 一列）、`trading_strategy_repository_test.go:314` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 交易模式留白 | 存成多空反手 | `trading_mode_domain.go:39-41`（先 `TrimSpace`） | `trading_strategy_domain_test.go:314`（`"   "` 一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 大小寫與系統寫法不同（`SPOT`） | 存成現貨 | `trading_mode_domain.go:45`（`EqualFold`，未改動） | `trading_strategy_domain_test.go:314`（`"SPOT"` 一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 交易模式認不得 | 整份拒絕、說出只能是多空反手或現貨、什麼都沒存 | `trading_mode_domain.go:60` → `trading_strategy_domain.go:76`（包成 `ErrTradingStrategyValidation`） | `trading_strategy_domain_test.go:363`（兩個拼法都在訊息裡）、`trading_strategy_controller_test.go:388`（400，且 repository 無 `Save` 期望 ⇒ 沒存） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 改掉一份交易策略的交易模式 | 讀回來是新的那一個 | 同一個建構子（建立與修改同一條路）＋`trading_strategy_repository.go:62`（`Select("name","trading_mode")`） | `trading_strategy_repository_test.go:285`（現貨→多空反手，讀回來確認） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 修改時填錯，措辭與建立時一字不差 | 同一句話；原本那一份一個欄位都沒變 | 同一個建構子——不是第二套規則；驗證在寫入之前，整份拒絕 | `trading_strategy_domain_test.go:363` | asserts-oracle | produces-oracle | 🟡 partial |
| AC-09 | 有機器人在跑時改不動 | 拒絕並說出是哪幾台在跑，與改任何其他欄位一字不差 | 交易策略既有的修改關卡（未改動）——它擋的是**整份寫入**，新欄位自動被涵蓋 | `trading_strategy_assistant_queries_test.go:216`（切片前既有，仍綠） | asserts-oracle | produces-oracle | 🟡 partial |

**AC-08 為何 partial：** 「措辭一字不差」由**同一個建構子**保證（建立與修改共用 `NewTradingStrategyDomain`），
測試斷言的是建立那一次的措辭。要直接斷言修改那一次，得再走一次同一個建構子——
那會是一個永遠不可能失敗的斷言。「原本那一份一個欄位都沒變」同理：
驗證發生在 repository 被呼叫之前，沒有東西可以變。

**AC-09 為何 partial：** 關卡本身未改動、擋的是整份寫入，所以交易模式自動被涵蓋。
沒有「只改交易模式也擋得住」的專屬測試——那個測試會與既有那一條走完全相同的程式路徑。

### US-02 — 這一刀之前存在的交易策略讀作多空反手

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-10 | 讀一份切片前建立的交易策略 | 交易模式是多空反手 | `trading_strategy.go:43` 的 `default:'longShort'`——**AutoMigrate 加欄位時既有每一列都拿到它**，不必寫搬遷程式 | `trading_strategy_repository_test.go:314`（寫一列不帶模式，讀回來是 `longShort`；跑在真實 PostgreSQL 上） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 重演一份切片前建立的交易策略 | 照多空反手重演，成績單與切片前完全相同 | `trading_strategy_backtest_application.go:68` → `trading_mode_domain.go:40` | `trading_strategy_backtest_assistant_query_test.go:404`（斷言 15,000／開倉 2，與切片前同一組數字） | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 — 重演一份交易策略時不必再講交易模式

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-12 | 現貨交易策略自動照現貨重演 | 照現貨重演，不是照預設的多空反手 | `trading_strategy_backtest_application.go:68` | `trading_strategy_backtest_assistant_query_test.go:381`（12,000／開倉 1／逐筆做多）、`trading_strategy_backtest_controller_test.go:249` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 多空反手交易策略照多空反手重演 | 照多空反手重演 | 同上 | `trading_strategy_backtest_assistant_query_test.go:404` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 請求裡硬塞交易模式等於沒塞 | 照現貨重演 | `trading_strategy_backtest_request.go` 已無該欄位（Go 的 JSON 解碼忽略不認識的鍵） | `trading_strategy_backtest_controller_test.go:249`（body **真的**送了 `"tradingMode":"longShort"`，斷言 12,000 而非 15,000） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 助手也不必講交易模式 | 照現貨重演 | `trading_strategy_backtest_assistant_query.go` 的 arguments 已無該欄位 | `trading_strategy_backtest_assistant_query_test.go:381`（參數完全沒提模式） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 助手硬給會被自己的參數關卡擋下 | 那個參數不在參數表裡，而參數表不收多餘欄位 | `ArgumentSchema()` 的 `additionalProperties:false`（未改動）＋刪掉的那一筆 property | `trading_strategy_backtest_assistant_query_test.go:447`（斷言 schema 不含 `tradingMode` 且含 `"additionalProperties":false`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 助手看得懂交易模式現在從哪裡來 | 說明講得出由那一份決定、這裡給不了；並講得出不能放空時該去改那一份 | `trading_strategy_backtest_assistant_query.go` 的 `Description()` | `trading_strategy_backtest_assistant_query_test.go:447`（斷言含兩個拼法、「不能放空」、「交易策略」） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 改掉交易模式之後重演的數字跟著變 | 現貨 12,000／多空反手 15,000 | 同一個 `BacktestAccountDomain`（未改動）＋不同的來源 | `trading_strategy_backtest_assistant_query_test.go:381`（12,000）與 `:404`（15,000），同一段三棒歷史 | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 — 重演一支策略腳本那條路一個字都沒變

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-19 | 重演一支策略腳本仍然自己講交易模式 | 照現貨重演 | `models.BacktestRequest`（未改動）→ `backtest_domain.go:85` | `backtest_domain_test.go:322`（切片前既有，斷言未改動仍綠） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 沒講交易模式 | 照多空反手重演 | `trading_mode_domain.go:40` | `backtest_domain_test.go:322` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 講了認不得的值，措辭與切片前一字不差 | 整次拒絕，欄位名 `tradingMode`，句子與切片前相同 | `trading_mode_domain.go:60` → `backtest_domain.go:91` 包回 `BacktestValidationFailure(BacktestTradingModeField, …)`。`backtestFieldError.Error()` 印 `"backtest validation failed: <reason>"`，reason 即那句話原文 ⇒ **逐字相同** | `backtest_domain_test.go:322`（斷言欄位名與句子，**斷言未改動**）、`trading_mode_domain_test.go:56` | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 — 現貨機器人的訊息一個字都沒變

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-22 | 現貨機器人判出買入 | 標題寫「買入」，整則與切片前逐字相同 | `strategy_bot_message_domain.go:81`（動詞起點就是信號自己的話）＋`:83` 的 `CanGoShort()` 為假 ⇒ 走「都不成立」那條路 | `strategy_bot_message_domain_test.go:51`（**整行相等**，非 contains）、`:132`；`strategy_bot_run_application_test.go:345` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 現貨機器人判出賣出 | 標題寫「賣出」，逐字相同 | 同上 | `strategy_bot_message_domain_test.go:39`（整行相等）、`:51` | asserts-oracle | produces-oracle | ✅ conforms |

**「逐字相同」怎麼被守住的：** `strategy_bot_message_domain_test.go` 與
`strategy_bot_run_application_test.go` 的 fixture 都改成**現貨**，而**檔內每一條既有斷言一字未動**。
現貨那條路產出的字串因此被切片前寫下的斷言原封釘住——不是一個要另外維護的相容性承諾，
而是這個形狀的必然結果。

### US-06 — 多空反手機器人的訊息講我真正要做的動作

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-24 | 多空反手判出買入 | 標題寫「做多」，不是「買入」 | `strategy_bot_message_domain.go:83-88` | `strategy_bot_message_domain_test.go:174`（整行相等）、`strategy_bot_run_application_test.go:252` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 多空反手判出賣出 | 標題寫「做空」，不是「賣出」 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 訊息講得出這份交易策略的交易模式 | 有一行講出交易模式是多空反手 | `strategy_bot_message_domain.go:122-124` → `trading_mode_domain.go:89` | `strategy_bot_message_domain_test.go:216`、`strategy_bot_run_application_test.go:252`（斷言「交易模式 多空反手」） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 各來源怎麼說仍然寫信號 | 那兩行仍然寫「買入」與「持有」，標題寫「做多」 | `strategy_bot_message_domain.go:138`（`signalInWords`，不在任何分叉裡） | `strategy_bot_message_domain_test.go:204`（斷言逐行寫信號、且**不**寫做多做空）、`strategy_bot_run_application_test.go:252` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 讀不到參考價時兩種模式說法一樣 | 那一行與現貨一字不差 | 參考價那兩行不在任何分叉裡 | `strategy_bot_message_domain_test.go:247`（兩種模式各跑一次，逐字斷言同一句） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 切片前建立的交易策略走多空反手那一種寫法 | 標題寫「做空」 | `trading_mode_domain.go:40` → `CanGoShort()` 為真 | `strategy_bot_message_domain_test.go:225`、`strategy_bot_run_application_test.go:316` | asserts-oracle | produces-oracle | ✅ conforms |

### Core Business Rules（PRD §4）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| BR-01 | 交易模式是交易策略的一個欄位：建立時給、修改時可改、跟著它存 | 三件都成立 | `trading_strategy.go:43`、`trading_strategy_domain.go:74`、`trading_strategy_repository.go:62` | `trading_strategy_repository_test.go:285`、`trading_strategy_controller_test.go:359` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 取值二選一，與回測**完全同一組** | 沒有第三種，也不是第二份清單 | `selectableTradingModes`（`trading_mode_domain.go:12`）只有一份 | `trading_mode_domain_test.go:13`、`:193` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 沒填即多空反手 | 留白、未填一律讀作多空反手 | `trading_mode_domain.go:40` | `trading_strategy_domain_test.go:314`、`trading_strategy_assistant_queries_test.go:408` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-04 | 認不得整份拒絕，不默默退回預設 | 拒絕並列出兩種，絕不 fallback | `trading_mode_domain.go:60`（只有一個回傳路徑，沒有 fallback 分支） | `trading_strategy_domain_test.go:363`、`trading_strategy_controller_test.go:388` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 大小寫寬容度沿用回測既有那一套 | 不另立一種 | `trading_mode_domain.go:45`（同一個 `EqualFold`，未改動） | `trading_strategy_domain_test.go:314`（`"SPOT"`） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 既有的交易策略一律讀作多空反手 | 不猜、不改行為 | `trading_strategy.go:43` 的 DB 預設值 | `trading_strategy_repository_test.go:314` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 重演一份交易策略時模式來自那一份；請求（HTTP 與助手）沒有那個欄位 | 形狀本身說不出那句話 | `trading_strategy_backtest_application.go:68`；`trading_strategy_backtest_request.go` 與 `tradingStrategyBacktestAssistantArguments` 皆無該欄位 | `trading_strategy_backtest_controller_test.go:249`（塞了也沒用）、`trading_strategy_backtest_assistant_query_test.go:447`（schema 不含它） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 重演一支策略腳本那條路完全不變 | 使用者當次指定、沒講即多空反手、認不得即整次拒絕 | `models.BacktestRequest`、`backtest_domain.go:85` 皆未改動語意 | `backtest_domain_test.go:322`（**斷言一字未改**仍綠） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-09 | 機器人訊息照模式分兩種寫法，現貨那一種逐字等於切片前 | 兩種寫法；現貨逐字不變 | `strategy_bot_message_domain.go:81-88`、`:122` | 見 US-05／US-06 的落點 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 「各來源怎麼說」永遠寫信號 | 兩種模式都一樣 | `strategy_bot_message_domain.go:138` 不在任何分叉裡 | `strategy_bot_message_domain_test.go:204` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 信號仍然只有三種取值 | 做多／做空是訊息的措辭，不是第四、第五種信號 | `vo/signal_vo.go` 與 `domains/signal_domain.go` **完全未改動**；「做多」「做空」兩個字串只出現在 `strategy_bot_message_domain.go` | — | no-test | produces-oracle | 🟡 partial |

**BR-11 為何 partial：** 這是一條否定性陳述（「某件事沒有發生」）。
由 Out of Scope 檢查守住：`vo.SignalVo` 仍然只有三個常數，
而 `做多`／`做空` 在整個 `internal/` 底下只出現在訊息模型與它的測試裡。
硬寫一個「信號只有三種」的斷言，只會得到一個永遠綠、永遠不會失敗的測試。

### Edge Cases（PRD §4）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| EC-01 | 同一份被三台機器人引用，改掉它的交易模式 | 三台下次醒來的訊息都換成新的寫法 | `strategy_bot_run_application.go:499` 每一輪從那一份現讀（`playRound` 既有行為，未改動） | `strategy_bot_run_application_test.go:252`／`:316`（fixture 以指標持有那一份，測試改它、下一輪就看到） | asserts-oracle | produces-oracle | ✅ conforms |
| EC-02 | 交易模式認不得，同時名稱也空白 | 照既有驗證順序報第一個撞到的；都整份拒絕，沒有部分存檔 | `trading_strategy_domain.go`：名稱在 `:52-67`、模式在 `:74` ⇒ 名稱先報 | `trading_strategy_domain_test.go`（名稱那幾條為切片前既有，仍綠） | asserts-oracle | produces-oracle | 🟡 partial |
| EC-03 | 這一輪判出「打架」 | 不送訊息，與切片前一字不差；交易模式與它無關 | `StrategyBotVerdictDomain` 未改動；打架的輪次走 `decision.ShouldSend` 為假那條路，**根本不會建訊息** | 切片前既有的打架測試全數未改動仍綠 | asserts-oracle | produces-oracle | ✅ conforms |
| EC-04 | 多空反手機器人讀不到參考價 | 那一行與現貨一字不差 | 參考價兩行不在分叉裡 | `strategy_bot_message_domain_test.go:247` | asserts-oracle | produces-oracle | ✅ conforms |
| EC-05 | 資料庫裡的交易模式是個認不得的值 | 不可能發生（存的時候擋掉）；真的發生時照回測既有的拒絕處理，不另生一套 | 回測路徑：`backtest_domain.go:91` 整次拒絕。訊息路徑：`strategy_bot_message_domain.go:51-54` 讀作「沒有模式」⇒ 引述信號自己的話、不指名任何模式 | `trading_strategy_backtest_assistant_query_test.go:424`（拒絕且 outcome 為空）、`strategy_bot_message_domain_test.go:235` | asserts-oracle | produces-oracle | ✅ conforms |
| EC-06 | 一份設成現貨，但機器人盯的是可做空的市場 | 系統不管市場做不做得到 | 市場能力在任何路徑上都沒有被讀 | — | no-test | produces-oracle | 🟡 partial |

**EC-02 為何 partial：** 「順序」由程式碼的行序保證（名稱 `:52`、模式 `:74`），
而既有的名稱測試仍綠即證明名稱那幾條先撞到。沒有「同時兩個都錯」的專屬測試——
那個測試斷言的是實作順序，而順序不是規格承諾的東西，規格只承諾「都整份拒絕」。

**EC-06 為何 partial：** 也是否定性陳述（「系統不去問市場」）。
由 Out of Scope 檢查守住：交易模式的每一個讀取點都沒有碰 `TradingSymbol` 或市場能力。

### Non-Functional（PRD §6）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| NFR-01 | 切片前建立的交易策略重演結果完全相同 | 同樣的成績單、交易明細、資金曲線 | DB 預設值 → `trading_mode_domain.go:40` | `trading_strategy_backtest_assistant_query_test.go:404`（15,000／開倉 2）、`trading_strategy_repository_test.go:314` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-02 | 引用現貨交易策略的機器人訊息逐字相同 | 逐字 | 「都不成立」那條路 | 切片前既有斷言**一字未改**仍綠（見 US-05 說明） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-03 | 機器人的一輪不因此多讀任何東西 | 交易策略本來就每一輪都讀 | `strategy_bot_run_application.go:499` 讀的是 `playRound` 已經取回的那一份，**沒有新增任何查詢** | — | no-test | produces-oracle | 🟡 partial |
| NFR-04 | 不改變任何既有可見性規則 | 看不到的交易策略仍然回覆「找不到」 | 可見性相關程式碼完全未改動 | `trading_strategy_backtest_controller_test.go`（「別人的不在那裡」為切片前既有，仍綠） | asserts-oracle | produces-oracle | ✅ conforms |

---

## Orphans

| # | Behavior | Site | Explained by | Judgement |
| :--- | :--- | :--- | :--- | :--- |
| 1 | 助手的交易策略清單多帶了 `tradingMode` | `trading_strategy_list_assistant_query.go:51` | 無 PRD 條款；`improve-codebase` 階段發現的缺口 | ⚠️ 良性且必要。少了它，使用者說「我的帳戶不能放空」時，助手得逐份 `get` 才找得出該改哪幾份——把查詢次數花在一個清單一個字就能講完的事上。由 `trading_strategy_assistant_queries_test.go:275` 釘住 |
| 2 | `TradingModeDomain.InWords()` 只有訊息模型在用 | `trading_mode_domain.go:89` | 由 AC-26 要求 | ✅ 不是 orphan。唯一呼叫端就是那一條條款的落點 |
| 3 | `NewTradingModeDomain` 的錯誤不再帶任何哨兵 | `trading_mode_domain.go:60` | 由 `ARCH.md` §3 記載 | ⚠️ 良性。兩個提問者各自包自己的哨兵；回測那一側的措辭由 `backtest_domain_test.go:322`（斷言未改動）釘住逐字不變 |
| 4 | 移除了 `verdictInWords` 這個私有方法（改為在 `Text()` 內就地決定動詞） | `strategy_bot_message_domain.go:81-89` | `.claude/rules/architecture.md`：單一 public 呼叫端的 private 一律 inline | ✅ 正確的移除。它消除的重複是零，而就地決定與旁邊那個 `headlineMark` 是同一個既有寫法 |
| 5 | `TradingModeDomain.Value()` 這個對外讀取器 | `trading_mode_domain.go:64` | 切片前是 orphan（只有測試在用） | ✅ 不再是 orphan。`trading_strategy_domain.go:116` 現在用它把模式寫回 entity |

**Out of Scope 檢查**：PRD §1 列的七項全數**沒有對應程式碼**。逐一確認：

- **要多少錢開合約／止盈止損**：`TradingStrategy`、`StrategyBot`、`StrategyBotRunRecord`
  三個 entity 都**沒有**資金、槓桿或百分比欄位；`StrategyBotRoundDto` 只多了 `TradingMode` 一個欄位。
- **加寬機器人的輪次記錄**：`strategy_bot_run_record.go` **完全未改動**。
- **第三種交易模式**：`vo.TradingModeVo` 仍然只有兩個常數。
- **把交易模式存進策略腳本**：`entities.StrategyScript` 完全未改動。
- **改變回測的任何一條算法**：`BacktestAccountDomain`、`BacktestSimulationDomain`、
  `BacktestPositionDomain`、`BacktestEquityCurveDomain`、`PositionSizingDomain` 全數未改動。
- **替既有的交易策略猜它其實是現貨**：DB 預設值是 `longShort`，沒有任何依名稱或內容猜測的程式碼。

無越界。

---

## Summary

| Status | Count |
| :--- | :--- |
| ✅ conforms | 34 |
| 🔴 violation | 0 |
| 🟠 mis-asserted | 0 |
| 🟡 partial | 6 |
| ❌ gap | 0 |
| ❔ unclear | 0 |
| ⚠️ orphan | 2（皆良性，其中一項必要）＋2 正確的移除／不再是 orphan |

**Clauses:** 40 · **Conformance:** 85%（34/40 完全一致）

六條 partial 全部是同一類東西：**否定性陳述**（BR-11 信號沒有變多、EC-06 沒去問市場、
NFR-03 沒多讀東西）或**由「同一條程式路徑」保證而無法獨立斷言的事**
（AC-08 建立與修改共用建構子、AC-09 關卡擋整份寫入、EC-02 驗證的行序）。
每一條的程式行為都正確，且都由 Out of Scope 檢查或既有測試未改動仍綠交叉守住。
硬寫測試只會多出六個永遠綠、永遠不會失敗的斷言。

### 值得記下來的一件事

**現貨那一半的「逐字不變」不是靠一條相容性測試守住的，是靠把既有測試的 fixture 改成現貨、
然後一個斷言都不動。**

原本的做法會是：讓 fixture 留在預設值（多空反手），再另外寫幾條「現貨要長這樣」的新斷言。
那樣的話，新斷言是照著**改完後的程式碼**寫出來的——它證明的是「程式現在這樣跑」，
而不是「它跟以前一樣」。

把 fixture 換成現貨，等於讓**切片之前就寫下的每一個字**去審這一半。
`strategy_bot_message_domain_test.go` 那六條、`strategy_bot_run_application_test.go` 那四條，
一個字都沒動就全綠——這是「逐字不變」唯一拿得出來的證據。
