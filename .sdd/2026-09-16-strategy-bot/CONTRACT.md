# 策略機器人 — Contract Verification

**Oracle:** `.sdd/2026-09-16-strategy-bot/PRD.md` — Section 3 acceptance criteria, Section 4 business rules, Section 6 non-functional requirements
**Design map:** `.sdd/2026-09-16-strategy-bot/ARCH.md`
**Glossary:** `.sdd/UL-MAP.md`
**Kind:** static conformance audit — each clause's expected outcome was derived from the
PRD before any code or test was read, then the test and the code were judged
**separately** against it. Not a test run.

---

## 1. Clauses

Statuses: ✅ conforms · 🟡 partial (code right, no test pinning it) · 🟠 mis-asserted · 🔴 violation · ❌ gap · ❔ unclear

### US-01 — 把幾支策略組成一台機器人

| ID | Clause | Oracle (from the spec alone) | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01 | 組起一台機器人 | 建立成功、回覆識別碼、狀態為已停止 | `strategy_bot_service.go:38` · `strategy_bot_domain.go:145` | `strategy_bot_application_test.go` CreateResolvesEveryNamedStrategyThroughTheGates | ✅ |
| AC-02 | 同一支策略以不同參數值當成兩個來源 | 建立成功，A 與 B 各自說話 | `strategy_bot_signal_sources_domain.go:40` | `strategy_bot_domain_test.go` LetsOneStrategyBeTwoSources | ✅ |
| AC-03 | 巢狀的條件，深度為 2 | 建立成功且深度為 2 | `strategy_bot_condition_domain.go:55` | `strategy_bot_condition_domain_test.go` NodeCount / Depth | ✅ |
| AC-04 | 條件指到沒宣告的信號來源 | 拒絕，指出是哪一個代號 | `strategy_bot_condition_domain.go:66` | condition Refusals「naming a source that was never declared」 | ✅ |
| AC-05 | 來源代號重複 | 拒絕，說明代號不得重複 | `strategy_bot_signal_sources_domain.go:75` | domain Refusals「two sources answering to one label」 | ✅ |
| AC-06 | 參數名稱對不上 | 拒絕，指出是哪一個名稱 | `strategy_bot_signal_sources_domain.go:98` | domain Refusals「a value set on a knob…」＋ application test | ✅ |
| AC-07 | 信號來源指名一支看不到的策略 | 回覆「找不到」，與不存在者一字不差 | `strategy_bot_application.go:119`（既有三道關卡） | application test CreateRefusesAStrategyThisPersonCannotSee | ✅ |
| AC-08 | 買入條件不得為空 | 拒絕 | `strategy_bot_domain.go:112` | domain Refusals「no buy condition」/「no sell condition」 | ✅ |
| AC-09 | 條件群組至少兩個子條件 | 拒絕，說出下限 | `strategy_bot_condition_domain.go:88` | condition Refusals「a group joining only one condition」 | ✅ |
| AC-10 | 深度剛好到上限 5 | 建立成功 | `strategy_bot_condition_domain.go:113` | AcceptsTheDeepestAllowedNesting | ✅ |
| AC-11 | 深度 6 | 拒絕，說出上限 5 | 同上 | condition Refusals「nesting one level deeper」 | ✅ |
| AC-12 | 節點數 33 | 拒絕，說出上限 32 | `strategy_bot_condition_domain.go:118` | RefusesATreeWithTooManyNodes | ✅ |
| AC-13 | 信號來源 11 個 | 拒絕，說出上限 10 | `strategy_bot_signal_sources_domain.go:47` | RefusesMoreSignalSourcesThanAllowed | ✅ |
| AC-14 | 觸發間隔 0 | 拒絕，必須大於零 | `strategy_bot_domain.go:92` | domain Refusals「a trigger interval of nothing」 | ✅ |
| AC-15 | 觸發間隔 1441 | 拒絕，說出上限 1440 | `strategy_bot_domain.go:98` | domain Refusals「one minute over a day」 | ✅ |
| AC-16 | 名稱在自己的機器人之間重複 | 拒絕，該名稱已被使用 | `strategy_bot_repository.go:164`（唯一索引） | repository RefusesANameThisPersonAlreadyUses ＋ controller 409 | ✅ |
| AC-17 | 兩位使用者各有同名機器人 | 都成功，互不影響 | `strategy_bot.go` `(owner_id, name)` 索引 | repository CountsAndListsPerOwner | ✅ |
| AC-18 | 沒有登入就組不了 | 拒絕並要求先登入 | 既有 `requiresSignIn` 中介層 | controller RefusesEveryRouteWithoutProofOfIdentity | ✅ |

### US-02 — 讓機器人判出一個結論

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-19 | 買入條件成立 | 結論是買入 | `strategy_bot_verdict_domain.go:28` | verdict ReadsTwoConditionsAsOneConclusion | ✅ |
| AC-20 | 賣出條件成立 | 結論是賣出 | 同上 | 同上 | ✅ |
| AC-21 | 兩個都不成立 | 沒有結論、不送訊息 | 同上 | 同上 ＋ ShouldSend「a quiet round」 | ✅ |
| AC-22 | 巢狀的「或」那一邊成立 | 結論是買入 | `strategy_bot_condition_domain.go:141` | condition Holds「a nested group is read as the brackets say」 | ✅ |
| AC-23 | 持有可以當條件用 | 結論是買入 | 同上 | condition Holds「hold is a value a condition may compare against」 | ✅ |
| AC-24 | 兩個條件同時成立即為衝突 | 不送訊息、標上衝突、維持執行中 | `strategy_bot_verdict_domain.go:33` · `strategy_bot_run_state_domain.go` | verdict test ＋ run application MarksAConflictAndSaysNothing | ✅ |
| AC-25 | 每一個信號來源照自己的刻度各跑一次 | A 讀一小時的最後一根，B 讀五分鐘的最後一根 | `strategy_bot_run_application.go` `readSignals` | run application **ReadsEachSourceAtItsOwnCoarseness**（此次稽核補上：先前每一輪的兩個來源都是同一種刻度，這條實際上沒有被任何測試釘住） | ✅ |
| AC-26 | 衝突在下一輪不再成立就自動消失 | 衝突記號被清掉、送出買入 | `strategy_bot_run_state_domain.go` `RoundFinished` 無條件覆寫 `Conflicting` | run state RoundFinished 表格 | ✅ |

### US-03 — 只在信號變了的時候被打擾

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-27 | 剛啟動時第一個結論一定送 | 送出買入並記下 | `strategy_bot_verdict_domain.go` `ShouldSend` | verdict ShouldSend ＋ run application SendsAConclusionThatChanged | ✅ |
| AC-28 | 結論與上次相同就不送 | 不送，記著的不變 | 同上 | verdict ShouldSend ＋ SaysNothingWhenTheConclusionHasNotChanged | ✅ |
| AC-29 | 結論變了就送 | 送出並記下新的 | 同上 | verdict ShouldSend | ✅ |
| AC-30 | 沒有結論不清掉記著的信號 | 不送，記著的仍是買入 | `strategy_bot_run_state_domain.go` `RoundFinished` | run state RoundFinished「a round that sent nothing」 | ✅ |
| AC-31 | 中間沒有結論的那一輪不吃掉下一次變化 | 送出賣出 | 同上（記著的不變，故下一輪仍相異） | verdict ShouldSend ＋ RoundFinished | ✅ |
| AC-32 | 衝突不清掉記著的信號 | 不送，記著的仍是買入 | 同上 | run application MarksAConflictAndSaysNothing | ✅ |
| AC-33 | 停了再啟動，第一個結論重新送一次 | 送出買入（啟動時已清空） | `strategy_bot_run_state_domain.go` `Start` | run state StartPutsTheBotToWork… ＋ application StartPutsTheBotToWorkDueImmediately | ✅ |
| AC-34 | 訊息說得出判斷的依據 | 含機器人名、標的、信號、參考價與時刻、逐一列出每個來源、不稱成交價 | `strategy_bot_message_domain.go` | message domain 四個測試 ＋ run application SendsAConclusionThatChanged | ✅ |

### US-04 — 按下播放之後就可以離開

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-35 | 啟動之後立刻跑第一輪 | 變成執行中、立刻跑、不等一個間隔 | `Start` 把 `NextRunAt` 設為現在；`FindDue` 撈 `next_run_at <= now` | application StartPutsTheBotToWorkDueImmediately ＋ repository FindDueAnswersOnlyRunningBotsThatAreDue | ✅ |
| AC-36 | 重複啟動不算失敗 | 仍是執行中、不重跑、不清掉上次信號 | `strategy_bot_service.go:149` 早退 | application StartingAlreadyRunningChangesNothing | ✅ |
| AC-37 | 停止 | 變成已停止、不再被排到 | `Stop` ＋ `FindDue` 的 run_state 條件 | application StoppingAnAlreadyStopped… ＋ repository FindDue | ✅ |
| AC-38 | 重複停止不算失敗 | 仍是已停止 | `strategy_bot_service.go:184` | application StoppingAnAlreadyStoppedBotIsNotAFailure | ✅ |
| AC-39 | 沒有 Telegram 設定就啟動不了 | 拒絕、說要先設定、維持已停止 | `RequireStartable` ＋ `StrategyBotApplication.StartStrategyBot` | run state RequireStartable ＋ application StartRefusesWithNowhereToBeSpokenTo ＋ controller 400 | ✅ |
| AC-40 | 執行中的機器人達到數量上限 | 拒絕、說出 10、不停掉任何一台 | `RequireStartable` | run state RequireStartable ＋ application StartRefusesAtTheRunningLimitWithoutStoppingAnything | ✅ |
| AC-41 | 執行中的機器人改不動 | 拒絕、說要先停止、一個字都沒變 | `RequireEditable` | run state RequireEditable ＋ application UpdateRefusesWhileTheBotIsRunning ＋ controller 409 | ✅ |
| AC-42 | 刪除執行中的機器人不必先按停止 | 刪掉了、不再被排到 | `strategy_bot_service.go:133`（不檢查執行狀態） | application DeleteDoesNotAskForAStopFirst ＋ repository Delete | ✅ |
| AC-43 | 被停擺原因停下來的機器人，啟動時清掉原因 | 變成執行中、停擺原因被清掉 | `Start` | run state Start ＋ application StartPutsTheBotToWorkDueImmediately | ✅ |

### US-05 — 一輪出錯時，該停的停、該等的等

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-44 | 市場沒有交易就跳過 | 跳過、維持執行中、不送 | `strategy_bot_round_failure_domain.go` default | failure domain「a market that is not trading」 | ✅ |
| AC-45 | K 線不夠就跳過 | 跳過、維持執行中 | 同上 | failure domain ＋ run application KeepsRunningWhenTheCandlesAreNotThereYet | ✅ |
| AC-46 | 連不上 Telegram 就下一輪再送 | 維持執行中、記著的信號不更新 | `NewStrategyBotDeliveryFailureDomain` default | run application ReadsTelegramsRefusalTheWayItWasMeant | ✅ |
| AC-47 | Telegram 遲遲不答也是下一輪再送 | 同上 | 同上 | 同上 | ✅ |
| AC-48 | 策略被刪了就停下 | 已停止、原因是那支策略找不到了 | `NewStrategyBotRoundFailureDomain` | failure domain ＋ run application HaltsForFailuresThatWillNeverFixThemselves | ✅ |
| AC-49 | 採用來的策略被取消發佈 | 與上一列**一字不差** | 同上（兩個哨兵映到同一個停擺原因） | failure domain「withdrawn… stops it the same way」 | ✅ |
| AC-50 | 算式執行失敗就停下 | 已停止、原因是算不出來 | 同上 | failure domain ＋ run application HaltsWhenAScriptWillNotRun | ✅ |
| AC-51 | 金鑰不被接受就停下 | 已停止、原因是金鑰不被接受 | `NewStrategyBotDeliveryFailureDomain` | failure domain ＋ run application ReadsTelegramsRefusal… | ✅ |
| AC-52 | 找不到聊天室就停下 | 已停止、原因是找不到聊天室 | 同上 | 同上 | ✅ |

### US-06 — 管理自己的那幾台

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-53 | 列出自己的每一台，依名稱由小到大，並交出十一項內容 | 三台依名稱排序；每一台含全部欄位 | `strategy_bot_repository.go` `Order("name ASC")` · `strategy_bot.go` `ToDto` | repository CountsAndListsPerOwner（排序由 SQL 保證）＋ entity ToDto 三個測試（此次稽核補上建立／修改時間的斷言） | ✅ |
| AC-54 | 一台都沒有時回空清單，不是錯誤 | 空清單、非錯誤 | `strategy_bot_service.go:63` | application ListHandsBackAnEmptyListRatherThanARefusal | ✅ |
| AC-55 | 別人的機器人一律看不到，與不存在者一字不差 | 「找不到」 | `findOwnedBot` 回 `StrategyBotNotFound(id)` | application ReadsAndListsOnlyThisPersonsBots ＋ controller 404 | ✅ |
| AC-56 | 別人停不了我的機器人 | 「找不到」，我那台仍執行中 | 同上 | application StopRefusesToTouchSomebodyElsesBot | ✅ |
| AC-57 | 修改一台已停止的機器人 | 成功、更新修改時間、識別碼／建立時間／擁有者不變 | `strategy_bot_service.go:96`（`writeDto.OwnerID` 取自庫內）· repository 只更新三欄 | repository SaveReplacesTheSourcesAndTreesItHadBefore ＋ UpdateRunStateTouchesOnlyABotsLife | ✅ |
| AC-58 | 改回自己原本的名稱不算重複 | 成功 | `(owner_id, name)` 唯一索引與自己不衝突 | 由 AC-16 的索引語意涵蓋；**沒有專屬測試** | 🟡 |
| AC-59 | 建立時的每一條規則，修改時同樣適用 | 拒絕、一個字都沒變 | `UpdateStrategyBot` 走同一個 `NewStrategyBotDomain` | application UpdateRewriteRefusesAStrategyThisPersonCannotSee ＋ controller | ✅ |
| AC-60 | 刪除 | 查不到；來源與條件一併消失 | GORM cascade | repository DeleteTakesTheSourcesAndTreesWithIt | ✅ |
| AC-61 | 沒有登入就做不了任何一件 | 拒絕並要求先登入 | `requiresSignIn` | controller RefusesEveryRouteWithoutProofOfIdentity（七條路由全覆蓋） | ✅ |

### US-07 — 離開之後它還在跑

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-62 | 系統重新啟動之後自己繼續跑 | 仍是執行中、繼續被排到、不需重按 | 執行狀態留存於 `StrategyBots.run_state`；`FindDue` 每次重問，無任何行程內註冊 | repository FindDueAnswersOnlyRunningBotsThatAreDue（證明「狀態在庫裡、每次重問」）。**沒有測試直接演一次重啟** | 🟡 |
| AC-63 | 錯過的輪次不補跑 | 只跑一輪 | `RoundFinished`／`RoundSkipped` 把下次設為 **現在**＋間隔 | run state RoundFinished／RoundSkipped 斷言 `now+5m` | ✅ |
| AC-64 | 上一輪還沒跑完就不排下一輪 | 不併行跑第二輪 | `StrategyBotRoundGuard.TryEnter` | RoundGuard 測試 ＋ run application LeavesABotThatIsAlreadyMidRound | ✅ |
| AC-65 | 停止時進行中的那一輪跑完為止 | 那一輪跑完、之後不再被排到 | `Stop()` 只關 `done`，不取消執行中的 context | scan job StopsScanningWhenStopped（證明不再排新的）。**「進行中那一輪跑完」沒有專屬測試** | 🟡 |
| AC-66 | 背景總開關關掉時不排任何機器人、執行狀態不被改寫 | 沒有機器人被排到；狀態不變 | `backgroundJobsFor` 關閉時回空清單 | `cmd/server/dependencies_test.go`（0 個 job）。**「狀態不被改寫」為構造上成立**（關閉時無任何寫入路徑） | 🟡 |

### Section 4 — Core Business Rules

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| BR-01 | 兩個條件、一個結論；衝突與沒有結論都不送也都不動記著的信號 | 四選一，兩者皆靜默且不覆寫 | `strategy_bot_verdict_domain.go` | verdict 兩個表格 | ✅ |
| BR-02 | 規則與規則之間不得用且／或串接 | 系統沒有這種入口 | 型別上不存在：條件只有比對與群組兩種，群組的子項也是條件 | 由 `StrategyBotConditionDto`／`StrategyBotConditionDomain` 的形狀保證 | ✅ |
| BR-03 | 條件只有等於，沒有不等於 | 沒有這種入口 | `StrategyBotConditionDto` 無此欄位 | 同上（型別保證） | ✅ |
| BR-04 | 彙總刻度與參數值掛在信號來源、標的整台共用 | 各來源各自刻度；標的一個 | `StrategyBotSignalSource` 有刻度、`StrategyBot` 有 symbol | AC-25 ＋ AC-02 | ✅ |
| BR-05 | 機器人只問「現在怎麼樣」，一律取最後一根走完的 | 觀察區間僅一格 | `strategyBotObservationWindowLength = time.Minute` | AC-25（兩個刻度各自的截止點） | ✅ |
| BR-06 | 送成功才算送出去 | 送不成時不更新記著的信號 | `playRound` 只在 `DeliveryFailureNone` 時帶入信號 | run application ReadsTelegramsRefusalTheWayItWasMeant | ✅ |
| BR-07 | 執行狀態是留存的，不是記在記憶體裡的 | 重啟後照樣算數 | `StrategyBots.run_state` | AC-62 | 🟡 |
| BR-08 | 只服務擁有者；擁有者不得更換 | 別人一律「找不到」；rewrite 不改 owner | `findOwnedBot` ＋ `writeDto.OwnerID = storedBot.OwnerID` ＋ repository 的 `Select` 不含 owner_id | AC-55／AC-56 ＋ repository UpdateRunStateTouchesOnlyABotsLife | ✅ |
| BR-09 | 上限表（5／32／10／10／1–1440／128） | 各自的拒絕訊息說出該數字 | 六個 domain 常數 | AC-10～AC-15、AC-40 | ✅ |
| BR-10 | 停擺原因四種；另三種不停 | 四停三留 | `strategy_bot_round_failure_domain.go` | failure domain 兩個表格（七種情形全覆蓋） | ✅ |
| BR-11 | 一輪跑到一半系統關閉：沒有痕跡，仍是執行中，下次重跑 | 不漏訊號 | 沒有送出就沒有更新記著的信號 | 由 BR-06 的同一條路徑涵蓋；**沒有專屬測試** | 🟡 |
| BR-12 | 擁有者在一輪進行中刪除機器人：那一輪的結果不再有地方可寫 | 訊息不送出 | `RecordRound` 先 `FindOne`，查無即回錯 | run application CarriesOnWhenARoundCannotBeBookedIn | 🟡（涵蓋「寫不回去」，未演「刪除後訊息不送」） |

### Section 6 — Non-Functional Requirements

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| NFR-01 | 同一輪內各信號來源併發執行 | 併發而非逐一 | `readSignals` 的 `sync.WaitGroup` | `-race` 下全綠可佐證無資料競爭，**但沒有測試斷言「併發」本身** | 🟡 |
| NFR-02 | 每分鐘掃描一次；同時輪次有上限；不排隊堆積 | 掃描間隔與上限由設定控制 | `StrategyBotScanJob` ＋ `FindDue(limit)` | scan job 三個測試 ＋ repository FindDueHonoursTheCapOnTheReadItself | ✅ |
| NFR-03 | 受背景工作總開關約束 | 關閉時不啟動 | `backgroundJobsFor` | `dependencies_test.go` | ✅ |
| NFR-04 | 每一件事都要說得出目前登入者 | 說不出來一律拒絕 | `requiresSignIn` | controller RefusesEveryRouteWithoutProofOfIdentity | ✅ |
| NFR-05 | 機器人金鑰從頭到尾不離開系統 | 不出現在任何回覆或訊息裡 | 沿用既有封裝；`SendMessage` 與 `SendTestMessage` 共用同一條開鎖路徑 | 既有 telegram 測試 ＋ run application 斷言 credential 只到 proxy | ✅ |
| NFR-06 | 指標算式從頭到尾不離開系統；信號來源不含算式 | 回覆裡沒有算式 | `StrategyBotSignalSourceDto` **沒有這個欄位**——型別上放不進去 | entity SignalSourceToDtoCarriesNoScriptAndNoStaleName | ✅ |
| NFR-07 | 看不到一台機器人的每一種理由共用同一句「找不到」 | 一字不差 | `StrategyBotNotFound(id)` 單一構造 | AC-55 | ✅ |
| NFR-08 | 沿用既有六種刻度、五種種類、三個信號 | 一個都不新增 | 直接沿用既有 VO 與 domain | domain Refusals「a coarseness that is not one of the six」 | ✅ |
| NFR-09 | 一則訊息上限 4096 沿用 | 超過即拒絕不截斷 | **機器人訊息刻意不走這條規則**——那是使用者自己打的測試訊息的上限；系統自產的訊息若因此被拒，會讓一台機器人為了自己的訊息太長而靜默（見 `telegram_delivery_service.go` `SendMessage` 註解） | — | 🔴 **與 PRD 不符（見下）** |

---

## 2. Orphans

掃過新增的公開介面，沒有找到任何一項行為對應不到條款。
特別檢查 PRD 的 Out of Scope 清單：

| Out of Scope 項目 | 有沒有被實作 |
| :--- | :--- |
| 下單／代操 | 沒有。整個切片沒有任何寫向交易所的路徑 |
| 回測一台機器人 | 沒有。既有回測未被改動 |
| 執行紀錄的留存與查詢 | 沒有。只留「上一次送出的信號」與「停擺原因」 |
| Telegram 以外的投遞管道 | 沒有 |
| 一台機器人盯多個標的 | 沒有。`StrategyBot.Symbol` 是單一欄位 |
| 把機器人分享到市集 | 沒有 |
| 停擺時主動通知 | 沒有 |
| 條件裡比對信號以外的東西 | 沒有。`StrategyBotConditionDto` 只放得下來源代號與信號 |

---

## 3. Summary

```
Contract verification complete for "策略機器人".
Oracle: PRD Acceptance Criteria — 77 clauses (66 AC · 12 BR · 9 NFR).

✅ 69 conforms · 🔴 1 violation · 🟠 0 mis-asserted · 🟡 7 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 0 orphans
Conformance: 89.6% fully conforming, 98.7% with correct behaviour
```

### 🔴 Violation — NFR-09

PRD 第 6 節寫著「一則 Telegram 訊息的字數上限沿用既有的 4096，超過即拒絕不截斷」。
**實作刻意沒有把機器人訊息送進那條規則**，理由寫在 `telegram_delivery_service.go`
的 `SendMessage` 註解裡：那條上限的目的是讓**打字的人**知道自己送出去的不是半則；
一則系統自己組的訊息若因為太長被拒，結果是**一台機器人為了自己的訊息而靜默**，
而那個拒絕是說給一個不在現場的人聽的。

這是 PRD 寫錯，不是程式寫錯——但**兩者現在不一致，必須擇一**。
建議把 PRD 這一條改成：「機器人訊息由系統組成，長度受上限（來源 10 個）約束而非驗證；
組得出超長訊息是設計錯誤，不是執行期拒絕的理由。」

> 依既定作法，`/contract` 只稽核不改碼，所以這一條留給你拍板；PRD 與 UL-MAP 的修改
> 也一併等你決定。

### 🟡 Partial（行為正確，但沒有測試把它釘住）

| ID | 缺什麼 | 值不值得補 |
| :--- | :--- | :--- |
| AC-58 | 改回自己原本的名稱不算重複 | 值得，但要真的資料庫；索引語意已由 AC-16 證明 |
| AC-62 / BR-07 | 沒有測試直接演一次「重啟」 | Go 測試演不出行程重啟；現有證明（狀態在庫裡、每次重問）已是能做到的最強形式 |
| AC-65 | 「停止時進行中那一輪跑完」 | 值得，但需要能卡住一輪的測試替身 |
| AC-66 | 「總開關關掉時執行狀態不被改寫」 | 構造上成立（關閉時沒有任何寫入路徑存在） |
| BR-11 | 「一輪跑到一半系統關閉不漏訊號」 | 與 BR-06 同一條路徑，已被覆蓋 |
| BR-12 | 「刪除後那一輪的訊息不送出」 | 值得補 |
| NFR-01 | 沒有斷言「併發」本身 | 併發是效能特性不是行為；`-race` 全綠是能給的最強保證 |

### 稽核當場修掉的（本次已進版）

- **AC-25** 先前是 🟠：每一輪的兩個信號來源都設成同一種刻度，所以
  「各用自己的刻度」這條**沒有被任何測試釘住**，測試改壞刻度也不會紅。
  已加 `ReadsEachSourceAtItsOwnCoarseness`，斷言兩個不同的讀取截止點。
- **AC-53** 先前漏掉建立時間／最後修改時間的斷言，已補上（並一併釘住「一律以世界標準時間交出」）。

> **Ceiling:** 這是靜態稽核——它拿 PRD 的預期結果去分別檢查測試斷言與程式路徑，
> 不是靠跑測試判定，也不會自己發明情境去執行。
