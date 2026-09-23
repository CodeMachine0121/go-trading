# Contract Traceability Matrix — 短線回測強化（short-term-backtest）

Contract: PRD.md（`.sdd/2026-09-24-short-term-backtest/PRD.md` v1.0）
Design map: ARCH.md（§7 Traceability）
Implementation: branch `feat/short-term-backtest`（`git diff main...HEAD`，59 files）
Oracle: Acceptance Criteria（43 clauses：27 AC ＋ 14 BR ＋ 2 NFR；NFR Analytics = N/A 不計）

**Path abbreviations**
- Impl：`D/` = `internal/domain/models/domains/`；`fill` = `D/backtest_fill_timing_domain.go`；`seg` = `D/backtest_segments_domain.go`；`stat` = `D/backtest_trade_statistics_domain.go`；`sim` = `D/backtest_simulation_domain.go`；`csim` = `D/contract_backtest_simulation_domain.go`；`bd` = `D/backtest_domain.go`；`cbd` = `D/contract_backtest_domain.go`；`err` = `D/backtest_errors.go`；`src` = `D/trading_strategy_replay_sources_domain.go`；`ctv` = `internal/domain/models/vo/closed_trade_vo.go`；`cctv` = `internal/domain/models/vo/contract_closed_trade_vo.go`；`svc` = `internal/domain/service/backtest_service.go`；`csvc` = `internal/domain/service/contract_backtest_service.go`；`cfg` = `internal/config/application_config.go`；`deps` = `cmd/server/dependencies.go`；`bctl` = `internal/controller/backtest_controller.go`；`tsctl` = `internal/controller/trading_strategy_backtest_controller.go`
- Tests：`ST` = `D/tests/short_term_backtest_domain_test.go`；`BDT` = `D/tests/backtest_domain_test.go`；`TA` = `internal/domain/service/tests/backtest_time_allowance_test.go`；`CFG` = `internal/config/tests/application_config_test.go`；`CBC` = `internal/controller/tests/contract_backtest_controller_test.go`

**Spec-internal inconsistencies noted while writing the oracles (before reading code)**
1. **「一字不差」做不到字面意義**：§1 與 AC-27 要求「什麼都沒說時與今天一字不差」，但 US-03 要求**每一張**成績單都多五格，所以回應不可能與今天逐字相同。只能讀成「一張成績單、數字都相同」。（實作另外永遠回 `fillTiming` 與 `validationStartTime: null`，見 Orphans。）
2. **分段界線：「走完」與「開始時間」**：§4 寫調參段＝「驗證起點**之前走完**的格」、驗證段＝「驗證起點**以後走完**的格」。驗證起點落在一格中間時，那一格在驗證起點**之後**才走完，照字面應歸驗證段；ARCH §8 卻定成「開始時間 ≥ 驗證起點才歸驗證段」，那一格就歸調參段。PRD 與 ARCH 在未對齊刻度的驗證起點上說法相反（見 BR-11）。
3. **調參段湊不出任何一格**：PRD 只寫驗證段湊不出格要拒絕（AC-26），沒說調參段湊不出格怎麼辦（驗證起點緊貼在期間起點之後、第一格還沒走完時）。ARCH 寫兩段任一為空都拒絕。
4. **驗證起點等於期間終點**：AC-25 只寫「晚於期間終點」拒絕；「等於終點」該用哪一句拒絕（「必須落在期間之內」還是「驗證段湊不出任何一格」）沒寫。
5. **成本佔毛利的單位**：AC-17 寫「25%」，沒說回百分比 25 還是比例 0.25。實作回比例 0.25。
6. **格數上限數的是什麼**：AC-02「這一段要用六萬格」沒說清是期間內的刻度區間數、還是另含算式需要的回看歷史。實作數的是期間內的刻度區間數。

## Clauses

The `Spec-expected` column holds the business-observable oracle from Phase 2; the
concrete artifact it bridges to (via UL-MAP/ARCH) is what the audit columns check.

### US-01 重演有自己的格數上限與允許時間

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 在重演上限之內 | 上限 50,000，要用 40,000 格 → 照常交出成績單 | bd:193、deps:414/430、cfg:380 | CFG:11（預設 50000）＋ BDT:66 表格裡「上限之內」的列（上限用的是 1000） | **shallow**：沒有任何測試在 50,000 上限下重演 40,000 格，也沒驗 deps 真的把 `BacktestMaxCandleCount` 接進兩個重演服務 | produces-oracle | 🟠 mis-asserted |
| AC-2 | 超過重演上限 | 60,000 > 50,000 → 整次拒絕，說出「60000」與「最多 50000」 | bd:193-197 | BDT:117（`a stretch needing more buckets…`） | **shallow**：只驗 `ErrBacktestValidation`，沒驗句子裡有要用的格數和上限 | produces-oracle（「要用到 %d 根…最多 %d 根」） | 🟠 mis-asserted |
| AC-3 | 重演不再受單次查詢上限限制 | 查詢上限 1,000、重演上限 50,000、要 2,000 格 → 照常交出 | deps:414、deps:430（改讀 `BacktestMaxCandleCount`，不再讀 `KCandleQueryMaxResults`） | — | no-test：所有服務測試仍把 1000 傳成重演上限；沒有測試驗接線 | produces-oracle | 🟡 partial |
| AC-4 | 超過整次允許時間 | 到允許時間還沒跑完 → 整次中止，說出允許時間；沒有成績單 | svc:76-91、svc:130-146、svc:163-168、csvc:84-96、csvc:131-146、err:117-123、bctl:127、tsctl:138 | TA:47（現貨，腳本＋交易策略）、TA:102（合約兩種）、CBC:410（422） | asserts-oracle（`ErrBacktestTimeAllowanceSpent`、句子裡有 "20ms"、422；結果被丟掉）| produces-oracle（注意：預設時長印成 `1m30s`，不是「90 秒」——同一個時長，寫法不同） | ✅ conforms |

### US-02 可以選下一格開盤成交

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-5 | 收盤成交與今天相同 | 以 100 開倉 | sim:86-88、fill:28-29 | ST:81 | asserts-oracle（進場 100、`fillTiming`="close"） | produces-oracle | ✅ conforms |
| AC-6 | 下一格開盤成交 | 以 101 開倉，進場時間＝第二格 | sim:70-78 | ST:93 | asserts-oracle（101、進場時間＝start+1h） | produces-oracle | ✅ conforms |
| AC-7 | 最後一格的信號不成交 | 最後一格買入 → 沒有開倉 | sim:71-74、csim:98-102 | ST:104 | asserts-oracle（開倉 0 次；收盤成交的做法會開 1 次，所以有鑑別力） | produces-oracle | ✅ conforms |
| AC-8 | 開盤進場的那一格就判出場價位 | 第二格開盤 100 進場、最低 97、止損 2% → 同一格 98 止損出場 | sim:73-75 | ST:111 | asserts-oracle（`stopLoss`、98） | produces-oracle | ✅ conforms |
| AC-9 | 下一格開盤平倉 | 第三格賣出、第四格開盤 105 → 以 105 平倉 | sim:71-73（Apply 同時處理平倉） | ST:93（出場價 103） | **shallow**：出場那一格開盤＝收盤＝103，所以「下一格開盤平倉」和「下一格收盤平倉」測起來一樣；合約滑點那個案例（ST:181）也是開盤＝收盤＝104，而且沒驗出場價 | produces-oracle | 🟠 mis-asserted |
| AC-10 | 開盤那一刻的資金費率結算不收付 | 1h，01:00 開盤成交、01:00 有一次結算 → 累計資金費用 0 | csim:90-97 | ST:148 | asserts-oracle（開倉 1 次、資金費用 0；ST:173 對照「帶著倉位進來的就要付 5」） | produces-oracle | ✅ conforms |
| AC-11 | 合約的滑點照舊套在開盤成交價上 | 滑點 0.1%，開盤 100 → 成交於 100.1 | csim:99-101、`D/contract_position_terms_domain.go:61-64` | ST:181 | asserts-oracle（100.1，收盤是 104） | produces-oracle | ✅ conforms |
| AC-12 | 認不得的成交時點 | 「盤中」→ 整次拒絕，說出只有收盤成交與下一格開盤成交兩種 | fill:26-41、bd:166-169、bctl:98-103 | ST:123、CBC:396 | asserts-oracle（欄位 `fillTiming`、句子裡有 close 和 nextOpen、400） | produces-oracle | ✅ conforms |

### US-03 成績單多五格

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-13 | 一般情形 | +300、−100、+200、−100 → 獲利因子 500/200＝2.5、期望值 300/4＝75、最大連虧 1 | stat:38-65 | ST:205 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 一筆都沒虧 | +100、+50 → 獲利因子不適用、期望值 75 | stat:62-65 | ST:217 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 一筆都沒平倉 | 四格都不適用；最大連虧 0 | stat:25-28 | ST:226 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 打平打斷連續虧損 | −10、−20、0、−5 → 2 | stat:47-54 | ST:236 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 成本佔毛利 | 扣成本前 100＋100、成本 50 → 50/200＝25% | stat:67-70 | ST:244 | asserts-oracle（0.25；單位見不一致 #5） | produces-oracle | ✅ conforms |
| AC-18 | 價差本身沒賺時成本佔毛利不適用 | 扣成本前 −40 → 不適用 | stat:67 | ST:256 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 平均持倉時間 | 1h、3h → 2h | stat:42、stat:59-60 | ST:265 | asserts-oracle（7200 秒） | produces-oracle | ✅ conforms |
| AC-20 | 還開著的那一注不算進五格 | 一筆已平倉 +100 ＋ 一注還開著（浮虧） → 期望值 100、最大連虧 0 | `D/backtest_account_domain.go:245-252`、`D/contract_backtest_account_domain.go:167-200` | ST:295 | asserts-oracle（經由真的重演：100 買、101 賣＝+100；101 買、收在 50 還開著） | produces-oracle | ✅ conforms |

### US-04 樣本外驗證

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-21 | 切出調參段與驗證段 | 交出整段、調參段、驗證段三張 | bd:321-372、cbd:163-219 | ST:336、ST:427、CBC:362 | asserts-oracle（6／3／3 格、驗證段從驗證起點開始；合約 2／2；HTTP 兩段都在） | produces-oracle | ✅ conforms |
| AC-22 | 驗證段從空手開始 | 驗證段從初始資金、空手開始；調參段留下的那一注不在驗證段的交易明細裡 | bd:364-368（每段各建一個新的逐格走） | ST:349 | asserts-oracle（驗證段只有 120 進場的那一筆；調參段那一注仍開著、明細為空；初始資金 10000） | produces-oracle | ✅ conforms |
| AC-23 | 驗證段的算式看得到驗證起點以前的歷史 | 走到驗證段第一格時，算式看得到之前的 20 格 | svc:80-85（算式對全部格子只跑一次）、bd:364-368（只切信號） | — | no-test：沒有任何測試在有驗證起點時跑算式、再驗第一格驗證段看得到的歷史 | produces-oracle | 🟡 partial |
| AC-24 | 驗證起點等於期間起點 | 整次拒絕：「驗證起點必須落在期間之內」 | seg:26-34 | ST:389 | asserts-oracle（欄位 `validationStartTime`＋那一句） | produces-oracle | ✅ conforms |
| AC-25 | 驗證起點晚於期間終點 | 同上 | seg:31 | ST:390 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 驗證段湊不出任何一格 | 整次拒絕：「驗證段湊不出任何一格」 | seg:71-74、bd:288、cbd:134 | ST:406、ST:478 | asserts-oracle（現貨與合約都有；算式跑之前就拒絕） | produces-oracle | ✅ conforms |
| AC-27 | 沒給驗證起點 | 只交出整段一張，數字與今天相同 | seg:26-28、bd:353-355、cbd:201-203 | ST:374 ＋ 其他既有重演測試照舊通過（只多傳一個 `nil`／零值成交時點） | asserts-oracle（沒有 inSample／validation；數字沒變） | produces-oracle（照不一致 #1 的讀法；另有兩個永遠會回的新欄位，見 Orphans） | ✅ conforms |

### §4 Business rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | 格數上限與允許時間是系統設定（五萬、九十秒）；單次查詢上限不再限制重演 | 預設 50000／90s，可改；兩個重演服務讀的是重演自己的上限 | cfg:342-349、cfg:380-382、deps:414-415、deps:430-431 | CFG:11、CFG（`TestLoadReadsTheReplaySettings`） | **shallow**：預設值與讀設定都驗了；「不再用查詢上限」這半句（接線）沒驗 | produces-oracle | 🟠 mis-asserted |
| BR-2 | 允許時間從開始跑算式起算，涵蓋所有信號來源；超過整次中止；每一格算式各自的允許時間照舊 | 一個計時器涵蓋所有來源；讀資料庫不算；單格允許時間不變 | svc:130-146、csvc:131-146（一個 context 包住所有來源的迴圈）；`internal/infrastructure/script/indicator_script_runner.go:222-247` 沒改 | TA:72、TA:118 | **shallow**：交易策略案例只有**一個**來源，而且那一個自己就超時；換成「每個來源各自計時」的實作也會通過，「所有來源合計」沒被驗到 | produces-oracle | 🟠 mis-asserted |
| BR-3 | 第 N 格信號在 N+1 格開盤成交；每一格依序 ①開盤那一刻的結算 → ②開盤成交 → ③其餘結算 → ④出場價位 → ⑤收盤記資金曲線 | 現貨：②④⑤；合約：①②③④⑤ | sim:70-78；csim:90-107 | ST:93、ST:111（現貨 ②④）；ST:148、ST:157、ST:173（合約 ①②③） | **shallow**：合約的 ④（剛在開盤開的倉，同一格就判出場價位／強平）沒有測試；兩種都沒驗 ⑤ 記在收盤價上 | produces-oracle（照著算過：開盤那一刻的結算＝`!After(bucketStart)` 那一批，在 Apply 之前付；其餘的在 Apply 之後、ApplyExitLevels 之前付；資金曲線 `EquityAt(Close)`） | 🟠 mis-asserted |
| BR-4 | 開盤才開的倉不收付開盤那一刻的結算，但照常收付同一格稍晚的結算 | 1d，00:00 那次不付、08:00 那次付：500×100×0.0001＝5 | csim:91-103 | ST:148、ST:157 | asserts-oracle（0 與 5） | produces-oracle | ✅ conforms |
| BR-5 | 最後一格的信號不成交 | 現貨與合約都一樣 | sim:71-74、csim:98-102 | ST:104（只有現貨） | asserts-oracle（合約是同一個 `index-1` 寫法，沒有另外測） | produces-oracle | ✅ conforms |
| BR-6 | 收盤成交時順序與今天相同 | 出場價位 → 收盤成交 → 資金曲線（合約前面先付結算） | sim:86-88、csim:110-115 | 既有的 `backtest_simulation_domain_test`、`backtest_exit_simulation_test`、`backtest_transaction_cost_simulation_test`、`contract_backtest_domain_test`（零值成交時點＝收盤） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 獲利因子＝正淨損益合計 ÷ \|負淨損益合計\|；沒有虧損或沒有交易 → 不適用 | 如左 | stat:44-48、stat:62-65 | ST:205、ST:217、ST:226 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 每筆期望值＝淨損益合計 ÷ 筆數；平均持倉時間＝出場減進場的平均；沒有交易 → 不適用 | 如左 | stat:39-42、stat:57-60 | ST:205、ST:265、ST:226 | asserts-oracle | produces-oracle（持倉時間是整秒、無條件捨去：1s 與 2s 平均回 1，不是 1.5——只有秒以下才看得出差別） | ✅ conforms |
| BR-9 | 最大連續虧損＝淨損益 < 0 最多連續幾筆；打平與賺錢打斷；沒有交易 → 0 | 如左 | stat:47-54 | ST:205、ST:236、ST:226 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 成本佔毛利＝交易成本合計 ÷ 扣成本前合計；扣成本前＝淨損益＋兩端成本（合約另加回資金費用，資金費用不算成本）；扣成本前合計 ≤ 0 → 不適用 | 如左 | stat:67-70、ctv:72-82、cctv:54-65 | ST:244、ST:256、ST:276、ST:285 | asserts-oracle（合約：75＋20＋5＝100、成本 20；剛好等於 0 的邊界沒測，`IsPositive` 本身就排除了） | produces-oracle | ✅ conforms |
| BR-11 | 調參段＝驗證起點之前走完的格；驗證段＝驗證起點以後走完的格 | 刻度對齊時：開始時間 ≥ 驗證起點的格歸驗證段。驗證起點落在一格中間時，那一格照 PRD 字面（「以後走完」）應歸驗證段 | seg:55-76（依**開始時間**切：開始時間 ≥ 驗證起點才歸驗證段） | ST:336、ST:427（都是對齊刻度的驗證起點） | asserts-oracle（只測了對齊的情形） | **unclear**：驗證起點落在一格中間時，實作把那一格歸調參段；PRD 字面與 ARCH §8 相反（不一致 #2） | ❔ unclear |
| BR-12 | 兩段各自從初始資金、空手重演；信號由同一次逐格執行得到（算式在驗證段看得到之前的全部歷史） | 如左 | bd:321-372、cbd:163-219、svc:80-85 | ST:349 | **shallow**：驗了空手與初始資金；「同一次執行、看得到之前的歷史」沒有測試（同 AC-23） | produces-oracle | 🟠 mis-asserted |
| BR-13 | 整段成績單照舊從頭走到尾 | 有切段時整段仍涵蓋所有格 | bd:352、cbd:200 | ST:336（整段 6 格） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-14 | 分段成績單與整段同一種形狀，各有自己的資金曲線、交易明細與打架格數 | 現貨對現貨、合約對合約；打架格數各段自己數 | bd:329-350、cbd:176-197、src:141-168 | ST:362（1／2／3）、ST:349、ST:427（合約那段是 `longOnly` 的合約形狀） | asserts-oracle | produces-oracle | ✅ conforms |

### §6 Non-functional

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-1 | 整次允許時間預設九十秒 | 預設 90s（低於 100s） | cfg:381-382 | CFG:11 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 沿用既有登入與歸屬規則 | 四個重演入口仍要登入；交易策略仍看歸屬 | 路由與 middleware 沒改；`internal/application/trading_strategy_backtest_application.go` 的歸屬檢查沒改 | CBC:347 ＋ 既有現貨入口的登入／歸屬測試 | asserts-oracle | produces-oracle | ✅ conforms |

### Out of Scope — implemented?

| Out-of-scope item | Implemented? |
|-------------------|--------------|
| 參數掃描、多段滾動驗證、顯著性檢定 | No（`seg` 只收一個驗證起點） |
| 限價掛單、掛單吃單分別、每筆最低手續費 | No（成交時點只有 close／nextOpen） |
| 不同刻度混用、四棵條件樹、合約機器人 | No（這個分支沒動 `src` 的刻度檢查、條件樹、機器人） |
| 讓算式只看得到宣告的回看格數 | No（腳本執行器沒動） |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| seg:67-70 | 調參段湊不出任何一格 → 拒絕，說「調參段湊不出任何一格走完的刻度區間」；測試在 ST:448 | undocumented（只有 ARCH 寫到；PRD 沒這一條，見不一致 #3） |
| seg:31 | 驗證起點**等於**期間終點也用「必須落在期間之內」拒絕（PRD 只寫「晚於」） | undocumented（不一致 #4） |
| fill:27-35 | 成交時點會先去掉前後空白、不分大小寫比對（`NEXTOPEN` 也接受） | undocumented |
| `internal/domain/models/dto/backtest_result_dto.go`、`contract_backtest_result_dto.go`（`FillTiming`、`ValidationStartTime` 沒有 `omitempty`） | 每個回應都會回 `fillTiming` 和 `validationStartTime`（沒切段時是 `null`），就算什麼都沒給 | undocumented；跟「一字不差」的字面讀法衝突（不一致 #1） |
| bd:359-362、cbd:207-210 | 重演時切不開就默默只交出整段（`SelectInput*` 已先擋過，正常情況到不了） | undocumented（防禦性寫法） |
| deps:549 | 內建助手的交易策略重演用的是同一個 `backtestService`，所以現在也吃 50,000 的上限和 90 秒的允許時間 | undocumented（ARCH 說助手「照舊」，指的是沒有新欄位） |

No orphan implements an Out-of-Scope item.

**Documentation drift noticed（不算 clause）**：`.sdd/UL-MAP.md` 新加的「成交時點／獲利因子／每筆期望值／平均持倉時間／最大連續虧損筆數／成本佔毛利／驗證起點／調參段／驗證段」這幾列，程式名稱欄仍寫 *(尚未實作)*，但這個分支已經實作了。

## Summary

- Conforms: 33/43 clauses ✅ (76.7%)
- Violations: none
- Mis-asserted: AC-1, AC-2, AC-9, BR-1, BR-2, BR-3, BR-12（測試是綠的，但驗的比 oracle 弱）
- Partial: AC-3, AC-23（沒有測試驗 oracle）
- Gaps: none
- Unclear: BR-11（驗證起點落在一格中間時歸哪一段：PRD 與 ARCH 說法相反）
- Orphans: 6（全部 undocumented；0 項違反 out-of-scope）

---

## Resolution (after the audit)

| Item | Resolution |
| :--- | :--- |
| AC-1 / AC-3 / BR-1 | 新增「一次重演的上限是它自己的」測試：兩千格在五萬上限內接受、在一千上限內拒絕；設定預設值與讀取已有測試 |
| AC-2 | 拒絕訊息斷言要用幾格與最多幾格；措辭改為「超過一次重演可用的最大根數」 |
| AC-9 | 平倉那一格的開盤（103）與收盤（108）不同，斷言以開盤成交 |
| BR-2 | 新增兩個信號來源各自在限內、合計超過允許時間即中止的測試 |
| BR-3 | 新增合約下一格開盤成交時，成交那一格就觸發止損、資金曲線以收盤記的測試 |
| BR-12 / AC-23 | 新增分段重演時算式只跑一次、看得到整段歷史的測試 |
| BR-11 | 規格改為以一格的開始時間歸段（與設計一致）；驗證起點落在一格中間時，那一格歸調參段 |
| 允許時間措辭 | 改以秒說出（「90 秒內沒跑完」） |
| 六個 orphan | 規格 §4 補上：調參段湊不出一格即拒絕、驗證起點等於終點的措辭、成交時點拼法、成本佔毛利以比例呈現、內建助手同受約束；「一字不差」改為「既有數字一字不差」 |
| UL-MAP | 新詞彙補上技術名稱 |
