# 策略機器人的部位規劃 — Contract Verification Matrix

**Contract source:** `.sdd/2026-09-18-strategy-bot-position-plan/PRD.md`（Acceptance Criteria 為 oracle）
**Design map:** `.sdd/2026-09-18-strategy-bot-position-plan/ARCH.md`
**Glossary:** `.sdd/UL-MAP.md`
**Verified:** 2026-09-18
**Ceiling:** 靜態一致性稽核。逐條把**測試斷言**與**程式路徑**各自對照規格推出的 oracle，
不以「跑完全套變綠」當判準，也不自行發明並執行新的情境。

---

## Clauses

### US-01 — 一台機器人記得一組部位規劃

| ID | Clause | Oracle（由規格推出） | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01 | 填完整一組 | 五樣都跟著存起來 | `strategy_bot_domain.go:109` → `entities.StrategyBot` 六欄 → `strategy_bot_repository.go:59` | `strategy_bot_domain_test.go:181`、`strategy_bot_repository_test.go:395`、`strategy_bot_controller_test.go:527` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 整組都不填 | 存得起來，且沒有部位規劃 | `position_plan_domain.go:52`（資金非正 → 零值，**不回錯誤**） | `strategy_bot_domain_test.go:198`、`strategy_bot_repository_test.go:444`、`strategy_bot_controller_test.go:590` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 只填部位資金 | 押多少讀作全押，與回測預設值一字不差 | `NewPositionSizingDomain`（**未改動**的空字串分支） | `position_plan_domain_test.go:90`（`staking everything needs no figure` 那一列，斷言保證金 50000） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 押多少的百分比超過一百 | 整台拒絕，措辭與回測一字不差 | `position_sizing_domain.go:78`（**句子未改動**）→ `strategy_bot_domain.go:111` 包成 `ErrStrategyBotValidation` | `position_plan_domain_test.go:282`、`strategy_bot_domain_test.go:205`（並斷言訊息**不含** `backtest`）、`strategy_bot_controller_test.go:571` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 押多少的固定金額是零 | 同上 | `position_sizing_domain.go:82` | `position_plan_domain_test.go:282`（`a fixed amount of nothing`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 押多少填了認不得的值 | 整台拒絕並說出有哪三種 | `position_sizing_domain.go:67` | `position_plan_domain_test.go:282`、`strategy_bot_domain_test.go:205` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 槓桿小於一倍 | 整台拒絕 | `position_plan_domain.go:69` | `position_plan_domain_test.go:282`、`strategy_bot_domain_test.go:205` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 停損距離是負的 | 整台拒絕 | `position_plan_domain.go:98` | `position_plan_domain_test.go:282`（停損與停利各一列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 停損距離超過一百個百分點 | 整台拒絕，理由是止損價會變成負數 | `position_plan_domain.go:102` | `position_plan_domain_test.go:282` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 這一刀之前就存在的機器人 | 沒有部位規劃 | `entities/strategy_bot.go:64` 起六欄的 `default:0` | `strategy_bot_repository_test.go:444`（**跑在真實 PostgreSQL 上**：寫一列不帶那六欄，讀回來資金為零） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 正在跑的時候改不動 | 與改任何其他欄位一字不差 | 機器人既有的修改關卡（**未改動**）——它擋的是整台寫入 | 切片前既有的那一條（`postman` 亦有「執行中改不動」）仍綠 | asserts-oracle | produces-oracle | 🟡 partial |

**AC-11 為何 partial：** 關卡未改動、擋的是整台寫入，所以部位規劃自動被涵蓋。
沒有「只改部位規劃也擋得住」的專屬測試——它會與既有那一條走完全相同的程式路徑。

### US-02 — 訊息自己算完要押多少、停在哪裡

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-12 | 做多那一輪的四個數字 | 保證金 5000、名目 15000、止損 62255.085、止盈 67389.525 | `position_plan_domain.go:133` 起 | `position_plan_domain_test.go:49`（**四個數字逐字斷言**）、`strategy_bot_run_application_test.go:1201`（同四個數字出現在訊息裡） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 做空那一輪，止損在上面 | 止損 66105.915、止盈 60971.475 | `position_plan_domain.go:165`／`:174`（`movedBy` 依方向反向） | `position_plan_domain_test.go:71`（逐字斷言＋**再斷言止損大於參考價、止盈小於**）、`strategy_bot_message_domain_test.go:314` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 會虧／會賺算的是名目 | 虧 450、賺 750 | `position_plan_domain.go:167`／`:176`（`portionOf(notional, …)`） | `position_plan_domain_test.go:49`／`:71` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 不上槓桿 | 保證金 50000、無槓桿與名目兩行、止損 98 | `position_plan_domain.go:62`（非正即一倍）＋`strategy_bot_message_domain.go:179` | `position_plan_domain_test.go:90`（`no leverage` 與 `one times leverage` 兩列）、`strategy_bot_message_domain_test.go:338`（斷言**沒有**名目那一行） | asserts-oracle | produces-oracle | 🟡 partial |
| AC-16 | 固定金額就是那個數字 | 保證金 8000 | `PositionSizingDomain.StakeFor`（**未改動**） | `position_plan_domain_test.go:90`（`a fixed amount is that amount`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 部位資金不夠押下那個固定金額 | 說出部位資金不足，且不印一個算出來的保證金 | `position_plan_domain.go:145`（早退，只帶押不下的那個數字）＋`strategy_bot_message_domain.go:170` | `position_plan_domain_test.go:202`（名目與兩個出口都為空）、`strategy_bot_message_domain_test.go:338`（`a stake the capital cannot cover`：斷言 `部位資金不足，押不下 8000` 且**不含** `保證金 8000`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 只填停損 | 有止損那一行、沒有止盈那一行 | `position_plan_domain.go:165`／`:174` 的 `HasStopLoss`／`HasTakeProfit` | `position_plan_domain_test.go:90`、`strategy_bot_message_domain_test.go:338` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 只填停利 | 對稱 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |

**AC-15 為何 partial：** 「保證金 50000」「沒有那兩行」「止損 98」三件事分別有斷言
（前兩件在上列兩條測試裡），但**「止損 98」那個具體數字**沒有獨立斷言——
它走的是與 AC-12 完全相同的那一行乘法，而 AC-12 用 64180.5 逐字釘住了它。

### US-03 — 只有要開倉的那一輪才有部位規劃

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-20 | 現貨的買入要開倉 | 有建議部位、止損低於參考價 | `TradingModeDomain.TargetFor`（**未改動**）→ `position_plan_domain.go:140` | `position_plan_domain_test.go:49`（持多）、`strategy_bot_run_application_test.go:1201` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 現貨的賣出是出清 | **沒有**建議部位；訊息其餘照舊 | `TargetFor` 回「空手」→ `position_plan_domain.go:140` 早退 | `position_plan_domain_test.go:221`（`standing aside` 那一列）、`strategy_bot_run_application_test.go:1248`（**整條走完**：斷言訊息寫「賣出」且不含「建議部位」，且歷史記 `HasPositionPlan` 為假） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 多空反手的賣出要開空倉 | 有建議部位、止損高於參考價 | `TargetFor` 回「持空」 | `position_plan_domain_test.go:71` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 讀不到參考價 | 沒有建議部位；照現在的做法說讀不到 | `position_plan_domain.go:141`（`!hasReference`） | `position_plan_domain_test.go:221`（`no price to measure from`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 交易模式讀不出來 | 沒有建議部位 | `strategy_bot_service.go:348`（讀不出來就原樣回那一輪）＋`TradingModeDomain` 零值 → `TargetFor` 回「不變」→ `:140` 早退 | `position_plan_domain_test.go:221`（`no change` 那一列）、`:275`（零值） | asserts-oracle | produces-oracle | 🟡 partial |

**AC-24 為何 partial：** 兩條路都有斷言（模式讀不出來時服務原樣回那一輪；
零值模式的目標是「不變」而計畫因此不建議），但**「資料庫裡真的是個認不得的值」
那個端到端情境沒有測試**——它經過存檔關卡就不可能發生，而寫出來的那一條
會與既有那兩條走相同的程式路徑。

### US-04 — 沒填的那幾台一個字都沒變

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-25 | 沒有部位規劃的機器人 | 整則訊息與這個切片之前逐字相同 | `strategy_bot_message_domain.go:159`（整段不印那條路） | `strategy_bot_message_domain_test.go:414`（明白斷言四種字樣都不出現）＋**檔內每一條既有斷言一字未動仍綠** | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 現貨機器人填了才看得到金額 | 有保證金那一行、沒有槓桿那一行 | 按填沒填分，**不按交易模式分**：`:159` 只看 `HasPositionPlan`，`:179` 只看 `Leveraged` | `strategy_bot_message_domain_test.go:338`（`no leverage leaves out the notional`） | asserts-oracle | produces-oracle | 🟡 partial |

**AC-26 為何 partial：** 「填了就看得到金額」與「沒填槓桿就沒有那一行」各自有斷言，
但**「一台現貨機器人」這個特定組合**沒有專屬測試。它與多空反手走的是同一條路——
`positionPlanLines` 完全不讀交易模式，而那正是這一條規則的內容
（「按填沒填分，不按交易模式分」），由程式的形狀保證。

### US-05 — 訊息說得出它自己的兩個限制

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-27 | 講出這個系統不下單 | 有那一行 | `strategy_bot_message_domain.go:168`（寫在**段落標題裡**，不是小字附註） | `strategy_bot_message_domain_test.go:331`、`strategy_bot_run_application_test.go:1201` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 講出回測沒把止損止盈算進去 | 有那一行 | `strategy_bot_message_domain.go:205` | `strategy_bot_message_domain_test.go:331`、`strategy_bot_run_application_test.go:1201` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 沒有建議部位時不講那兩句 | 兩句都不出現 | `:159` 整段不印；`:205` 另受「至少一個出口」約束 | `strategy_bot_message_domain_test.go:414`、`:338`（`a size with neither exit`：有建議但無出口時也不印那一句警告） | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 — 那一輪的歷史記得住那幾個數字

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-30 | 有建議部位的一輪 | 記著那三個數字 | `strategy_bot_run_record_repository.go:75` 起 → `entities.StrategyBotRunRecord` 三個 `NullDecimal` | `strategy_bot_run_record_repository_test.go:150`（真實 PostgreSQL）、`strategy_bot_run_application_test.go:1201`（**斷言歷史拿到的是訊息裡那幾個數字**） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 沒有建議部位的一輪 | 那三欄是空的，其餘與切片前讀起來一樣 | `NullDecimal` 不設 Valid；`ToDto` 的 `figureOrNothing` 回 `nil` ＋ DTO 的 `omitempty` | `strategy_bot_run_record_repository_test.go:181`（entity 三個 `Valid` 為假、DTO 三個為 `nil`）、`:206`（押不下去也記空） | asserts-oracle | produces-oracle | ✅ conforms |

### Core Business Rules（PRD §4）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| BR-01 | 部位規劃有五樣 | 五樣都存得下、讀得回 | `PositionPlanSettingsDto`（寫入／讀回／一輪共用**同一個形狀**） | `position_plan_domain_test.go:393`（五樣原樣交回） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 部位資金是整組的開關 | 沒填即沒有部位規劃，其餘四樣填了也不算 | `position_plan_domain.go:52`（**在讀其餘四樣之前**早退） | `position_plan_domain_test.go:366`（資金為零、押多少填 150 → **不回錯誤**且不建議） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 倉位大小模式沿用回測既有的三種與措辭 | 取值、預設、兩條驗證、拒絕的句子全部一字不差 | `NewPositionSizingDomain` **句子未改動**；`PositionPlanDomain` 直接呼叫它 | `position_plan_domain_test.go:282`（三條句子逐字）＋`backtest_domain_test.go`（**斷言未改動**仍綠，證明回測那一側逐字不變） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-04 | 槓桿不得小於一倍；沒填即一倍且不印那兩行 | 三件都成立 | `position_plan_domain.go:62`／`:69`／`strategy_bot_message_domain.go:179` | `position_plan_domain_test.go:90`／`:282`／`:409` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 兩個距離不得為負、不得超過一百；各自沒填就不印 | 兩條驗證兩個出口都適用 | `validatedDistance`（**一份，兩個出口共用**） | `position_plan_domain_test.go:282`（停損與停利各兩列）、`:90` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 只有目標是持多或持空的那一輪才有 | 空手與不變都沒有 | `position_plan_domain.go:140` | `position_plan_domain_test.go:221` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 方向決定止損在哪一邊 | 持多在下、持空在上；止盈對稱 | `movedBy`（**一份**，兩個出口以反向參數呼叫） | `position_plan_domain_test.go:49`／`:71` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 讀不到參考價就沒有部位規劃 | 不用別的價替代 | `position_plan_domain.go:141` | `position_plan_domain_test.go:221` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-09 | 交易模式讀不出來就沒有部位規劃 | 不替任何人猜方向 | `strategy_bot_service.go:348` ＋ `TradingModeDomain` 零值 | `position_plan_domain_test.go:275` | asserts-oracle | produces-oracle | 🟡 partial |
| BR-10 | 有建議部位的訊息必須講出兩件事 | 兩句都在 | `strategy_bot_message_domain.go:168`／`:205` | `strategy_bot_message_domain_test.go:331` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 沒填的機器人訊息逐字等於切片前 | 逐字 | `:159` 整段不印 | 見 AC-25 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 執行紀錄存下那三個數字，沒有時為空 | 兩種都對 | `strategy_bot_run_record_repository.go:75` | `strategy_bot_run_record_repository_test.go:150`／`:181` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-13 | 每一個金額與價格都用精確小數 | 不出現浮點數 | entity 六欄與紀錄三欄皆 `numeric(38,18)`；`PositionPlanDto` 每一欄皆 `decimal.Decimal`；**這一刀新增的檔案裡沒有一個 `float64`** | — | no-test | produces-oracle | 🟡 partial |

**BR-13 為何 partial：** 否定性陳述（「沒有浮點數」）。
由型別本身守住——`grep float64` 在這一刀新增的四個檔案裡零命中，
而測試以字串比對小數（`"62255.085"`）而非浮點數比較，本身就是那件事的間接證據。

### Edge Cases（PRD §4）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| EC-01 | 固定金額大於部位資金 | 說出不足、不印算出來的保證金、不是錯誤、不中斷 | `position_plan_domain.go:145`（仍回 `true`：有設定就有建議，只是那個建議是「押不下去」） | `position_plan_domain_test.go:202`、`strategy_bot_message_domain_test.go:338`、`strategy_bot_run_record_repository_test.go:206` | asserts-oracle | produces-oracle | ✅ conforms |
| EC-02 | 停損距離填 100 | 止損價正好是零，**允許** | `validatedDistance` 用 `GreaterThan` 而非 `GreaterThanOrEqual` | `position_plan_domain_test.go:382`（斷言止損價為零） | asserts-oracle | produces-oracle | ✅ conforms |
| EC-03 | 停損與停利都沒填 | 只印保證金那幾行；那一輪仍然有建議部位 | `:165`／`:174` 兩個都不進 | `position_plan_domain_test.go:90`（`neither exit at all still suggests a size`）、`strategy_bot_message_domain_test.go:338` | asserts-oracle | produces-oracle | ✅ conforms |
| EC-04 | 槓桿填 1 | 與沒填一字不差 | `noLeverage` 比較用 `GreaterThan` | `position_plan_domain_test.go:90`（兩列並排，期望值完全相同） | asserts-oracle | produces-oracle | ✅ conforms |
| EC-05 | 這一輪判出「打架」 | 不送訊息，與既有一字不差 | 打架走 `ShouldSend` 為假那條路，**根本不會建訊息**（未改動） | 切片前既有的打架測試全數未改動仍綠 | asserts-oracle | produces-oracle | ✅ conforms |
| EC-06 | 三台引用同一份規則、各填不同的部位規劃 | 三台各算各的 | 設定從 `botDto.PositionPlan` 抄進那一輪，**不從交易策略抄** | `strategy_bot_run_application_test.go:1201`（設定掛在那一台上而非那一份規則上） | asserts-oracle | produces-oracle | 🟡 partial |
| EC-07 | 改了設定之後回頭看三天前那一輪 | 讀到的是**當時**算出來的數字 | 數字存在執行紀錄裡（`:75`），讀回時不重算 | `strategy_bot_run_record_repository_test.go:150` | asserts-oracle | produces-oracle | 🟡 partial |

**EC-06／EC-07 為何 partial：** 兩者的機制都有斷言（設定來自機器人；數字存下來而非重算），
但「三台並行」與「隔三天再看」這兩個**情境**沒有測試。
它們各自要一個假時鐘或三台機器人的編排，而兩條路上的每一段都已經各自被蓋住了。

### Non-Functional（PRD §6）

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| NFR-01 | 沒填的機器人訊息逐字相同、紀錄讀起來一樣 | 逐字 | `:159` ＋ DTO 的 `omitempty` | 見 AC-25、AC-31 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-02 | 回測的每一條算法一行不改 | 既有成績單一個數字不變 | `BacktestAccountDomain`、`BacktestSimulationDomain`、`BacktestPositionDomain`、`BacktestEquityCurveDomain` **完全未改動**；`PositionSizingDomain` 只動了錯誤的包裝，`StakeFor` 一行未改 | 回測既有測試全數未改斷言仍綠 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-03 | 精確小數 | 見 BR-13 | 同上 | — | no-test | produces-oracle | 🟡 partial |
| NFR-04 | 一輪不因此多讀任何東西 | 那五樣本來就跟著機器人讀回來 | `strategy_bot_run_application.go` 從 `botDto` 抄，**沒有新增任何查詢** | — | no-test | produces-oracle | 🟡 partial |
| NFR-05 | 不改變任何既有可見性規則 | 別人的機器人仍然「找不到」 | 可見性相關程式碼完全未改動 | 切片前既有測試仍綠 | asserts-oracle | produces-oracle | ✅ conforms |

---

## Orphans

| # | Behavior | Site | Explained by | Judgement |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `domains.NewSignalDomainOf` 這個新建構子 | `signal_domain.go:33` | 無 PRD 條款 | ✅ 正確的新增，且**消除了既有的重複**：`trading_strategy_backtest_domain.go` 原本為了滿足另一個建構子而組一個只有一個元素的 map，現在兩個呼叫端都直接給信號 |
| 2 | `PositionSizingDomain.Value()` 這個讀取器 | `position_sizing_domain.go:104` | `ARCH.md` §3 隱含（要存得回那兩樣） | ✅ 不是 orphan。`PositionPlanDomain.ToSettingsDto` 用它。第一版是從 `StakeFor(100)` 反推那個數字，而那會隨「模式拿它做什麼」改變——第四種模式出現的那天，存第三種會悄悄變樣 |
| 3 | `positionSizingFailure` 這個私有錯誤型別與 `PositionSizingFailureAboutFigure` | `position_sizing_errors.go` | `ARCH.md` §2、§4 | ⚠️ 良性且必要。它讓**同一句話**同時服務兩個哨兵：回測包成帶欄位名的拒絕、機器人包成自己的驗證錯誤。沒有它，存一台機器人的人會在畫面上讀到「backtest validation failed」——而那不是他在做的事。由 `strategy_bot_domain_test.go:205` 的 `NotContains("backtest")` 與回測既有測試同時釘住 |
| 4 | `validatedDistance`／`movedBy`／`portionOf` 三個 package-level 私有函式 | `position_plan_domain.go:97`／`:194`／`:206` | 無 PRD 條款 | ⚠️ 良性。`.claude/rules/architecture.md` 允許為**消除重複**而抽，而三者都是：`portionOf` 四處、`movedBy` 兩處（且它是止損與止盈不會漂移成兩套公式的唯一原因）、`validatedDistance` 兩處。三者都不讀任何 receiver 欄位，所以掛成 method 會是一個不用 receiver 的 method——更糟 |
| 5 | `entities` 的 `figureOrNothing` 與 persistence 的 `storedFigure` | `strategy_bot_run_record.go` 末、`strategy_bot_run_record_repository.go:86` | 無 PRD 條款 | ⚠️ 良性。各在自己那一層做一次方向相反的轉換，各被三處呼叫。合成一個會讓 entity 認識 persistence |
| 6 | `dto.StrategyBotRunRecordWriteDto` 這個新形狀 | `strategy_bot_run_record_write_dto.go` | `naming.md`「Service 收一組參數就封成 DTO」 | ✅ 正確。`Append` 原本四個參數，加上三個數字與一個旗標會變八個；而那三個數字**只有一起才有意義**——逐一傳過去，可以傳出「一輪沒有建議，但這裡有個止損價」這種沒有東西會拒絕的組合 |
| 7 | 合併了 `wholePercentage` 與 `oneHundredPercent` | `position_sizing_domain.go:19` | `improve-codebase` 階段發現 | ✅ 正確的移除。同一個套件裡同一個值的第二個名字 |
| 8 | 測試 fixture 改為捕捉 `Append` 的寫入 | `strategy_bot_run_application_test.go` | 無 PRD 條款 | ⚠️ 良性。fixture 既有那條 `Append` 期望是 `AnyTimes`，gomock 會先配對到它，所以測試自己再加一條永遠不會觸發——捕捉是唯一問得出「歷史拿到了什麼」的方法 |

**Out of Scope 檢查**：PRD §1 列的六項**沒有任何一項有對應程式碼**。逐一確認：

- **讓回測模擬止損止盈**：`BacktestAccountDomain` 完全未改動，
  逐棒仍然不問高低點碰到了什麼；**而訊息裡那一行警告正是這件事還沒做的證據**。
- **追蹤部位**：沒有任何「現在持有什麼」的欄位、型別或查詢；
  `PlanFor` 不收也不讀任何持倉資訊。
- **多台機器人共用一筆錢**：部位資金是 `StrategyBots` 上的一欄，
  沒有任何跨機器人的加總。
- **停損距離由策略腳本算**：`indicator_script_shape.go` 與 `IndicatorResultTypeVo`
  完全未改動；`SignalVo` 仍然只有三個取值。
- **手續費與滑點**：沒有任何相關欄位或算式。
- **把部位規劃存進交易策略**：`entities.TradingStrategy` **完全未改動**。

無越界。

---

## Summary

| Status | Count |
| :--- | :--- |
| ✅ conforms | 39 |
| 🔴 violation | 0 |
| 🟠 mis-asserted | 0 |
| 🟡 partial | 9 |
| ❌ gap | 0 |
| ❔ unclear | 0 |
| ⚠️ orphan | 4（皆良性，其中一項必要）＋4 正確的新增／移除 |

**Clauses:** 48 · **Conformance:** 81%（39/48 完全一致）

九條 partial 分三類，沒有一條是程式行為的疑慮：

- **否定性陳述**（BR-13、NFR-03 沒有浮點數、NFR-04 沒多讀東西）——
  證據是程式碼裡**沒有那幾行**，硬寫斷言只會永遠綠。
- **由同一條程式路徑保證**（AC-11 關卡擋整台寫入、AC-15 與 AC-12 同一行乘法、
  AC-26 `positionPlanLines` 根本不讀交易模式、AC-24／BR-09 兩半各有斷言）。
- **情境層的編排**（EC-06 三台並行、EC-07 隔三天再看）——
  機制各自有斷言，把它們接起來要一個假時鐘或三台機器人的編排。

### 值得記下來的兩件事

**一、規格先釘、再寫測試的順序，這一次抓到的是設計而不是算術。**

`PlanFor` 的第一版是三個分開的問題——「有設定嗎」「這一輪要開倉嗎」「讀得到價嗎」——
由呼叫端依序問。照 PRD 的 US-03 推 oracle 的時候才看清楚：
**那三個問題只能一起回答**，因為「沒有建議部位」對讀訊息的人是**一個**事實。
分開問的那一版會讓訊息長出三種「這一輪沒有東西要押」的寫法，
而三種寫法遲早會有一種忘記更新。現在它們是一個回傳值，四個理由收在裡面。

**二、這一刀最重要的一行程式是一行警告。**

`　⚠️ 回測沒有把止損止盈算進去`

它不是文案。使用者拿得到的每一張成績單，都是在**沒有這個停損**的前提下算出來的，
而這則訊息正在叫他去掛那個停損。金額算錯他會看出來；
「我的停損從來沒被回測驗證過」他**永遠不會自己發現**。
那一行是這整刀唯一補得起那個落差的東西，所以它被 PRD US-05 釘成驗收項——
而它什麼時候可以拿掉，正是下一刀（讓回測模擬止損止盈）有沒有做完的判準。
