# Contract Traceability Matrix — 合約回測（contract-backtest）

Contract: PRD.md（`.sdd/2026-09-23-contract-backtest/PRD.md` v1.0）
Design map: ARCH.md（§7 Traceability）
Implementation: branch `feat/contract-backtest`（`git diff main...HEAD`，65 files）
Oracle: Acceptance Criteria（89 clauses：65 AC ＋ 22 BR ＋ 2 NFR；NFR Analytics = N/A 不計）

**Path abbreviations**
- Impl：`D/` = `internal/domain/models/domains/`；`pos` = `D/contract_backtest_position_domain.go`；`terms` = `D/contract_position_terms_domain.go`；`acct` = `D/contract_backtest_account_domain.go`；`sim` = `D/contract_backtest_simulation_domain.go`；`rules` = `D/contract_trading_rules_domain.go`；`mode` = `D/contract_trading_mode_domain.go`；`cbd` = `D/contract_backtest_domain.go`；`ctsbd` = `D/contract_trading_strategy_backtest_domain.go`；`slip` = `D/backtest_slippage_domain.go`；`src` = `D/trading_strategy_replay_sources_domain.go`；`tsd` = `D/trading_strategy_domain.go`；`mdk` = `D/market_data_kind_domain.go`；`svc` = `internal/domain/service/contract_backtest_service.go`；`bapp` = `internal/application/backtest_application.go`；`tsbapp` = `internal/application/trading_strategy_backtest_application.go`；`botapp` = `internal/application/strategy_bot_application.go`
- Tests：`CBD` = `D/tests/contract_backtest_domain_test.go`；`CTS` = `D/tests/contract_trading_strategy_domain_test.go`；`CBA` = `internal/application/tests/contract_backtest_application_test.go`；`CTA` = `internal/application/tests/contract_trading_strategy_application_test.go`；`CBC` = `internal/controller/tests/contract_backtest_controller_test.go`；`CBS` = `internal/domain/service/tests/contract_backtest_service_test.go`

**Spec-internal inconsistencies noted while writing the oracles (before reading code)**
1. US-03「交易成本照名目收」寫「保證金 10,000、進場成本率 0.05%」，但在預設「初始資金 10,000、全押」下，§4 的可押上限公式得出的保證金是 10,000 ÷ 1.0025 ≈ 9,975.06，不是 10,000。要讓場景成立，初始資金需為 10,025，或改用固定金額 10,000（兩份測試各用了一種寫法）。
2. §4 寫滑點「超過 100」即拒，場景卻寫「滑點 0.1%」與「滑點 −0.1」，沒說清單位（百分比還是比例）。程式採百分比（`portionOf` ÷100），與場景一致。
3. §4 寫「槓桿小於一 → 整份拒絕」，US-01 又寫「槓桿留白即一倍」。明確填 0 算留白還是小於一，兩處說法衝突；ARCH 選了「零 → 1」（見 BR-15）。

## Clauses

The `Spec-expected` column holds the business-observable oracle from Phase 2; the
concrete artifact it bridges to (via UL-MAP/ARCH) is what the audit columns check.

### US-01 保證金與槓桿

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 押下去的是保證金，承擔的是名目 | 5×，收盤 100 買入 → 多倉：保證金 10,000、名目 50,000、數量 500 | terms:66-80 | CBD:203 | asserts-oracle（保證金 10000、數量 500、槓桿 5） | produces-oracle | ✅ conforms |
| AC-2 | 賺賠照數量乘價差 | 110 平多，損益 +5,000；同一格開空 | pos:185-212、acct:92-118 | CBD:203（`PositionOpenCount`==2）＋ CBD:286（多空反手時賣出開空） | asserts-oracle（合起來看） | produces-oracle | ✅ conforms |
| AC-3 | 槓桿留白即一倍 | 名目＝保證金 | cbd:51-54 | CBD:222（10,000 在 100 → 數量 100；槓桿 "1"） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 槓桿小於一整份拒絕 | 0.5 → 整份拒絕，說「槓桿倍數不得小於一」 | cbd:55-58 | CBD:231（欄位 leverage，「不得小於 1 倍」） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 槓桿超過標的上限整份拒絕 | 上限 125，填 150 → 整份拒絕，說出 125 | cbd:59-63、rules:101-112 | CBD:241 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 名目所在那一級不允許這麼高的槓桿時開倉被擋下 | 20×，名目 200,000 落在 10× 那一級 → 不開倉、重演繼續、被擋下 1 次 | rules:134-152、acct:107-109 | CBD:253 | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 交易模式

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-7 | 多空反手空手時聽到賣出 | 開空倉 | mode:60-79 | CBD:286 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 只做多空手時聽到賣出 | 什麼都不發生，仍空手 | mode:70-73、acct:101 | CBD:293 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 只做多持多倉時聽到賣出 | 平多；**錢回到可用資金**；空手 | acct:97-102、pos:220-229 | CBD:300 | **shallow**：只看開倉次數與平倉方向，沒驗錢回到可用資金（例如最終權益 11,000） | produces-oracle | 🟠 mis-asserted |
| AC-10 | 只做空空手時聽到賣出 | 開空倉 | mode:70-76 | CBD:307＋CBD:321（這一列確認開的是空倉） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 只做空空手時聽到買入 | 什麼都不發生 | mode:64-67 | CBD:314 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 只做空持空倉時聽到買入 | 平空，之後空手 | acct:92-102 | CBD:321 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 交易模式留白即多空反手 | 留白＋賣出 → 開空 | mode:32-35 | CBD:328、CBD:352（回報 longShort） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 合約重演沒有現貨這一種 | 「現貨」→ 整份拒絕，列出三種可選 | mode:37-50、cbd:45-49 | CBD:359 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 反手時付不起新的那一注 | 平多、不開空、停在空手 | acct:97-112、terms:66-70 | CBD:369（開倉 1 次、平倉 1 筆、最終權益 5000） | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 交易規格與滑點

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-16 | 數量照步進往下取整 | 1,000 在 810 → 數量 1.234、保證金 999.54、剩 0.46 | rules:118-129、terms:79-80 | CBD:385 | asserts-oracle（1.234、999.54、權益 1000） | produces-oracle | ✅ conforms |
| AC-17 | 名目不足最小名目時開倉被擋下 | 固定金額 3 < 5 → 不開倉、被擋下 1 次 | rules:141-144 | CBD:400 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 止損價照價格跳動單位取最接近的一檔 | 100×(1−2.03%)=97.97 → 跳動單位 0.1 → 98.0 | terms:82-84、rules:171-177 | CBD:412（最低 98.0 觸發止損，出場於 98） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 沒有交易規格的合約標的 | 整份拒絕，說「還沒有交易規格」 | rules:53-58、svc:143-165 | CBD:427、CBD:434、CBA:180 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 滑點讓買進成交得貴一點 | 100 → 100.1 | slip:31-33、terms:61 | CBD:449 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 滑點讓賣出成交得便宜一點 | 開空 100 → 99.9 | slip:36-38、terms:62-64 | CBD:450 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 滑點留白不計 | 成交於 100 | slip:14-16 | CBD:451（decimal 零值＝留白） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 滑點不得為負 | −0.1 → 整份拒絕「滑點不得為負」 | slip:19-21、cbd:65-69 | CBD:471、CBC:204 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 交易成本照名目收 | 5×、保證金 10,000、0.05% → 進場成本 25 | terms:99 | CBD:481、CBD:497 | asserts-oracle（兩份都得 25；見上方不一致 #1） | produces-oracle | ✅ conforms |

### US-04 強制平倉看標記價格

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-25 | 多倉的強平價 | 10×、進場 100 → (100−10)/0.995 ≈ 90.4523，在進場價下方 | pos:69-79 | CBD:532（標記 90.4 → 強平）＋CBD:560（90.46 → 撐住）→ 夾出 90.4–90.46 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 最新價碰到但標記價格沒碰到就不強平 | 最新價低點 90、標記低點 91 → 不強平 | pos:130-132 | CBD:524 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 標記價格碰到強平價 | 標記低點 90.4 → 強平、收回 0、原因＝強平出場 | pos:143-144、203-207、223-225 | CBD:532 | asserts-oracle（損益 −10000、權益 0、原因 liquidation） | produces-oracle | ✅ conforms |
| AC-28 | 空倉的強平價在上方 | (100+10)/1.005 ≈ 109.45 > 100 | pos:74-76 | CBD:567（109.4 撐住，109.5 強平） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 沒有分級時用最小那一級 | 用規格的維持保證金率；成績單說「最小那一級」 | rules:67-73、181-184 | CBD:591、CBD:532（0.5% 算出的 90.45）、CBC:154 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 有分級時用名目所在那一級 | 用開倉當下名目所在那一級的**費率與速算額**；成績單說出分級與確認時間 | rules:157-167、pos:71、terms:96 | CBD:599 | **shallow**：每一級的速算額都是 0（CBD:58），所以從沒驗過速算額；只驗了費率與確認時間 | produces-oracle | 🟠 mis-asserted |
| AC-31 | 強平最多歸零 | 價格跳空遠超強平價 → 收回 0，可用資金不為負 | pos:109-111、223-225 | CBD:583 | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 止損與強平誰先到

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-32 | 止損比強平近 | 止損 95 比強平 90.45 近；最新價低點 94、標記低點 90 → 止損出場於 95 | pos:127-141 | CBD:626 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | 強平比止損近 | 止損 85、強平 90.45，標記低點 90 → 強平出場 | pos:143-145 | CBD:633 | asserts-oracle（照場景原文；另見 BR-2） | produces-oracle | ✅ conforms |
| AC-34 | 同一格碰到止損與止盈 | 最高 106、最低 94 → 止損出場於 95 | pos:137-154 | CBD:640 | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 資金費率

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-35 | 多倉在正費率時付錢 | 500×100×0.01% = 5 付出；保證金 −5；強平價往進場價靠近 | pos:84-94、acct:41-56 | CBD:690、CBD:707（權益 9995）、CBD:773 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 空倉在正費率時收錢 | 收到 5 | pos:88-90 | CBD:691（−5） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-37 | 多倉在負費率時收錢 | 收到 5 | pos:87 | CBD:692（−5） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 結算之後才開的倉不收付 | 在結算那一格收盤才開的倉 → 不收付那一次 | sim:78-84（先收付資金費用再開倉） | CBD:716 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | 一格內有多次結算 | 一天刻度三次結算 → 三次都收付（15） | sim:68-78 | CBD:726 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-40 | 結算紀錄沒有標記價格 | 用那一格的標記收盤價（500×120×0.01%=6） | acct:49-52 | CBD:741 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-41 | 收盤那一刻的結算屬於下一格 | 09:00 結算屬於 09:00 那一格 | sim:65-75（`[start,end)`） | CBD:751（不屬於前一格）＋CBD:690（下一格開盤那一刻的結算在下一格收付） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-42 | 累計資金費用是淨額 | 付 30、收 12 → 18 | acct:170、192-195 | CBD:762 | asserts-oracle | produces-oracle | ✅ conforms |

### US-07 合約成績單

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-43 | 做多做空分開統計 | 做多 3 筆、勝率 2/3；做空 2 筆、勝率 0 | acct:166-216 | CBD:794 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-44 | 沒有空單時做空勝率不適用 | 做空 0 筆，勝率＝不適用（null，不是 0） | acct:200-208 | CBD:814 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-45 | 強平出場被記下 | 強平出場筆數 1；原因＝強平出場 | acct:177-178 | CBD:532 | asserts-oracle | produces-oracle | ✅ conforms |

### US-08 其他規則

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-46 | 指名一支 K 線種類的策略腳本做合約重演 | 整次拒絕，說「吃的是 K 線」 | bapp:87-113、mdk `RequireReplayableAs` | CBA:170、CBC:194（400） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-47 | 期間湊不出兩格 | 整次拒絕，理由與現貨相同 | cbd:128-130（共用 `notEnoughKCandlesForBacktest`） | CBD:825、CBS:214 | asserts-oracle | produces-oracle | ✅ conforms |

### US-09 交易策略的行情種類與交易模式

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-48 | 建立一份合約交易策略 | 合約來源＋合約種類 → 建立成功 | tsd:76-107 | CTS:35、CTA:22 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-49 | 沒說行情種類即 K 線 | 建立成功，種類＝K 線 | mdk:39-43、entities/trading_strategy.go ToDto | CTS:42 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-50 | 合約交易策略混進 K 線的來源 | 整份拒絕，說出來源 A 吃的是另一種行情 | trading_strategy_signal_sources_domain.go `RequireMarketDataKind`、tsd:105 | CTS:55 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-51 | K 線交易策略混進合約的來源 | 同上 | 同上 | CTS:64、CTA:51 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-52 | 行情種類建立後不得更換 | 修改被拒絕，說「行情種類建立後不得更換」 | trading_strategy_service.go:122、mdk `RetainingForTradingStrategy` | CTS:155、CTA:98 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-53 | 合約交易策略記著交易模式 | 只做空 → 存為 shortOnly | tsd:92-97 | CTS:101、CTA:22 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-54 | 合約交易策略沒填交易模式 | → longShort | mode:32-35 | CTS:102 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-55 | K 線交易策略不收交易模式 | 整份拒絕，措辭與今天相同 | tsd:85-90（`NewSpotOnlyReplayDomain`） | CTS:124 | asserts-oracle | produces-oracle | ✅ conforms |

### US-10 重演一份合約交易策略

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-56 | 重演一份合約交易策略 | 只做多、兩個來源都 1h、5×、BTCUSDT → **用 1h 合約行情格**、照只做多規則回成績單＋資金曲線＋交易明細 | tsbapp:76-104、ctsbd:24-60、svc:99-138 | CBA:231（shortOnly）、CBC:310 | **shallow**：驗了交易模式有照策略走、方向與損益也對，但沒驗刻度（`Interval`＝來源的 1h）與資金曲線；兩支測試都只有一個來源 | produces-oracle | 🟠 mis-asserted |
| AC-57 | 重演時送來交易模式 | 整次拒絕「交易模式由交易策略自己決定」 | ctsbd:42-45 | CTS:284、CBC:221 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-58 | 拿 K 線交易策略做合約重演 | 整次拒絕「吃的是 K 線」 | ctsbd:30-38 | CTS:292、CBA:255 | asserts-oracle | produces-oracle（註：svc 先讀交易規則，所以標的沒有規格時會先說「沒有規格」） | ✅ conforms |
| AC-59 | 拿合約交易策略做現貨重演 | 整次拒絕「吃的是合約行情」 | D/trading_strategy_backtest_domain.go:36 | CTS:183、CBA:269 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-60 | 單一來源的合約交易策略等於那一支腳本 | 結果與直接重演腳本完全相同 | src `Combine`:140-、ctsbd:84-96 | CTS:226（整份 DTO `assert.Equal`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-61 | 打架的格子被計數 | 當作持平；打架棒數 1 | src:152-164 | CTS:254 | asserts-oracle | produces-oracle | ✅ conforms |

### US-11 現貨那一邊擋下合約

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-62 | 現貨重演指名一支合約策略腳本 | 整次拒絕「吃的是合約行情」 | bapp:44-62、87-113 | CBA:192、CTS:173 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-63 | 這一刀之前存下的混合交易策略 | 現貨重演 → 說出來源 A 吃的是另一種行情 | src:66-75、tsbapp:111-（帶上來源的 MarketDataKind） | CTS:193 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-64 | 機器人引用合約交易策略 | 整台拒絕「策略機器人目前只跑 K 線」 | botapp:163-、mdk `RequireFollowableByStrategyBot` | CTA:119、CTS:166 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-65 | 現貨重演照舊 | 結果與這一刀之前一字不差 | 現貨模型沒動；D/trading_strategy_backtest_domain.go 改為委派給 src | 既有現貨測試套件（只改了建構子參數，斷言沒動） | asserts-oracle（回歸測試） | produces-oracle | ✅ conforms |

### Business Rules（§4）

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | ① 帶著倉位進入這一格時收付結算（含起點、不含收盤）；資金費用進出保證金並重算強平價 | 收付在出場判定之前；強平價隨之移動 | sim:64-81、pos:84-94 | CBD:773（同一格先收付資金費用，後觸發強平） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | ② 不利側：止損（最新價）vs 強平（標記價格），離進場價近的先；止損成交在止損價（含滑點）、強平收回零 | 兩個都碰到時，近的那個先 | pos:127-145 | CBD:626（兩個都碰到、止損較近） | **shallow**：「強平較近而且兩個都碰到」這個分支沒測過（CBD:633 的最新價低點 90 根本碰不到 85 的止損） | produces-oracle | 🟠 mis-asserted |
| BR-3 | ③ 止盈（最新價），成交在止盈價（含滑點） | 不利側之後才判止盈 | pos:147-154 | CBD:640、857、867 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | ④ 照交易模式套信號，收盤價成交（含滑點） | 平倉／開倉／反手 | sim:82-84、acct:79-119 | CBD:277 表格、CBD:449-451 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | ⑤ 以收盤價記下資金曲線一點 | 權益 = 可用資金 + 收盤價下的倉位價值 | sim:85、acct:133-139 | CBD:385（EquityCurve[0]=1000）、CBD:707 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 開倉的那一格不判出場價位 | 開倉那一格的高低點即使越過止損／強平，也不出場 | sim:81-84（出場判定排在開倉之前） | — | no-test（每一份測試的開倉格都是平的：高＝低＝收） | produces-oracle | 🟡 partial |
| BR-7 | 名目＝保證金×槓桿；數量＝名目÷成交價，照步進往下取整；實際保證金＝數量×成交價÷槓桿 | 見 AC-1/AC-16 | terms:72-80、rules:118-129 | CBD:203、385、898 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 可押上限＝可用資金÷(1＋槓桿×進場成本率) | 10,025 ÷ 1.0025 = 10,000 | terms:66-67、backtest_transaction_costs_domain.go:170-177 | CBD:481、CBD:937 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 維持保證金＝數量×標記價格×維持保證金率−速算額（開倉當下名目所在那一級） | 速算額 c 會讓強平價移動 | pos:69-79（cushion = M + c）、terms:96 | CBD:599 | **shallow**：c 從沒設過非零值 | produces-oracle（代數驗算過：多倉 (qE−M−c)/q(1−r)、空倉 (qE+M+c)/q(1+r)） | 🟠 mis-asserted |
| BR-10 | 強平價：保證金＋數量×(價格−進場價)×方向＝維持保證金 | 多倉 90.4523、空倉 109.4527 | pos:69-79 | CBD:532、560、567 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 資金費用＝數量×結算標記價格×費率；多倉付正費率、空倉收正費率 | ±5 | pos:84-94 | CBD:690-692 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 進場成本＝名目×進場成本率；出場成本＝數量×出場價×出場成本率 | 兩端都收 | terms:99、pos:209-210 | CBD:481、497（只測進場） | **shallow**：沒有一份測試設過 `ExitCostPercentage` | produces-oracle | 🟠 mis-asserted |
| BR-13 | 滑點：買進 ×(1＋s)、賣出 ×(1−s)（止損／止盈／信號出場也要套） | 進出場成交價都往不利方向偏 | slip:31-38、terms:61-64、pos:171-177 | CBD:449-451（只測進場） | **shallow**：出場成交的滑點（`exitFillFor`）沒測過 | produces-oracle | 🟠 mis-asserted |
| BR-14 | 一筆損益＝數量×價差×方向−進場成本−出場成本＋資金費用淨額（收為正）；強平＝−(開倉保證金＋進場成本) | 見公式 | pos:203-212 | CBD:203（只有價差）、CBD:544（強平 −10050） | **shallow**：一般出場那一筆，沒測過「兩端成本＋資金費用」一起算的損益 | produces-oracle | 🟠 mis-asserted |
| BR-15 | 驗證：槓桿小於一、大於標的最高槓桿 → 整份拒絕 | 0.5 拒、150>125 拒；**明確填 0（小於一）也要拒** | cbd:51-63 | CBD:231、241 | asserts-oracle（0.5／150 兩種情況） | **diverges**：`leverage.IsZero()` → 1（cbd:52-54），明確填 0 也被當留白、默默改成一倍，而不是拒絕（見上方不一致 #3；ARCH §3 明寫「零→1」） | 🔴 violation |
| BR-16 | 交易模式認不得 → 拒 | 列出三種 | mode:37-50 | CBD:359 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-17 | 滑點為負或超過 100 → 拒 | −0.1 拒、101 拒 | slip:19-25 | CBD:471、CBD:930 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-18 | 標的沒有交易規格 → 拒 | 說出沒有交易規格 | rules:53-58 | CBD:427、434、CBA:180、CBS:202 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-19 | 現貨重演既有的每一條驗證 | 同一句話、同一個欄位 | cbd:35-38（`NewBacktestDomain`） | CBD:949 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-20 | 付資金費用使保證金低於維持保證金 → 當格以強平處理 | 同一格強平 | sim:78-81、pos:125 | CBD:773 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-21 | 沒有分級也沒有規格 → 整份拒絕 | 拒 | rules:53-58 | CBD:427 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-22 | 算式逐格失敗 → 整次拒絕 | 回傳錯誤、沒有結果 | svc:83-85、128-132 | CBS:161/169 | asserts-oracle | produces-oracle | ✅ conforms |

### Non-Functional（§6）

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-1 | 單次重演的格數上限沿用現貨重演的回測根數上限 | 超過上限 → 拒絕，措辭與現貨相同 | D/backtest_domain.go:180（經 cbd:35）、cmd/server/dependencies.go（`KCandleQueryMaxResults`） | — | no-test（每一份合約測試都用 100000 當上限） | produces-oracle | 🟡 partial |
| NFR-2 | 只有登入者能重演；三道關卡與「找不到」措辭 | 沒登入 → 401；別人的腳本／交易策略 → not found | dependencies.go:435、556（`requiresSignIn`） | CBA:316、328（not found） | no-test：兩條新路由沒有「沒登入 → 401」的測試（routes_test 只列出路由） | produces-oracle | 🟡 partial |
| NFR-3 | Analytics | N/A | — | — | — | — | N/A（不計） |

### Out of Scope — negative checklist

| Item | Present in diff? |
|------|------------------|
| 全倉保證金 | No（逐倉：pos 的保證金是每一注自己的） |
| 四棵條件樹（開多／平多／開空／平空） | No（仍是買入／賣出兩棵樹） |
| 信號來源混用兩種行情 | No（被拒絕，AC-50/51/63） |
| 合約策略機器人、合約建議部位與訊息 | No（機器人拒絕合約策略，AC-64） |
| 下一格開盤成交、每筆最低手續費、掛單／吃單分別、分級歷史 | No |
| 內建行情對話助手認得合約重演 | No（`assistantqueries` 只改了測試的建構子參數） |
| 回測結果留存 | No（svc 什麼都不存） |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| cbd:40-43 | 請求帶了 `maintenanceMarginRate` 就拒絕（「維持保證金率由分級決定」）；測試在 CBD:267 | undocumented |
| cbd:72-77 | 百分比倉位加上「照名目收的進場成本」後永遠付不起 → 整份拒絕（`NeverOpensAnything`）；測試在 CBD:937 | undocumented（現貨規則延伸出來的合約版） |
| rules:137-139 | 低於最小下單量時開倉被擋下（PRD §1 提到，但沒有 AC／BR）；測試在 CBD:912 | undocumented |
| rules:121-122 | 規格沒寫數量步進時保留精確數量（16 位小數）；測試在 CBD:898 | undocumented |
| botapp:53 | 修改機器人（不只建立）時，引用合約交易策略也會被拒絕 | undocumented（合理延伸） |
| svc:193-201 | 期間開始前最後一次結算會讀進來，排給腳本看，但從不收付；測試在 CBS:235 | undocumented |

No orphan implements an Out-of-Scope item.

## Summary

- Conforms: 77/89 clauses ✅ (86.5%)
- Violations: BR-15（明確填 0 的槓桿被默默讀成一倍，不是以「小於一」拒絕：`cbd:52-54`）
- Mis-asserted: AC-9、AC-30、AC-56、BR-2、BR-9、BR-12、BR-13、BR-14（測試是綠的，但驗的比 oracle 弱）
- Partial: BR-6、NFR-1、NFR-2（沒有測試驗 oracle）
- Gaps: none
- Unclear: none
- Orphans: 6（全部 undocumented；0 項違反 out-of-scope）

---

## Resolution (after the audit)

| Item | Resolution |
| :--- | :--- |
| BR-15（零槓桿） | 規格改為「槓桿留白或零即一倍」（與 BRIEF、UL-MAP、ARCH 一致）；新增「明確填零即一倍」測試 |
| US-03 成本情境的數字矛盾 | 情境改為「固定金額押 10,000」，與測試一致 |
| 滑點單位 | 規格明定單位是百分點（0.1 即 0.1%） |
| AC-9 | 補上「平倉後錢回到可用資金」的最後剩多少斷言（11,000） |
| AC-30 / BR-9 | 新增速算額不為零的分級測試（強平價 ≈ 91.33 而非 91.84） |
| AC-56 | 補上刻度（1h）與資金曲線點數斷言 |
| BR-2 | 新增「止損與較近的強平同格都碰到 → 強平」測試 |
| BR-12 | 新增出場成本率測試（出場成本 11、淨損益 989） |
| BR-13 | 新增信號出場（多、空）與止損出場的滑點測試 |
| BR-14 | 新增含兩端成本與資金費用的一般出場淨損益測試（4,942.5） |
| BR-6 | 新增「開倉那一格的低點不觸發止損」測試 |
| NFR-1 | 新增超過單次可讀根數即拒絕的測試 |
| NFR-2 | 新增兩個合約入口未登入回拒絕的測試 |
| 六個 orphan | 規格 §4 補上：送來維持保證金率即拒絕、永遠付不起的百分比、最小下單量／無數量步進、機器人建立與修改都不得引用合約交易策略、觀察區間前最後一次結算不收付 |
| AC-58 的拒絕順序 | 保留：交易規則先讀（槓桿上限要靠它驗），同時缺規格時先說缺規格——兩句都正確，前者是使用者要先補的 |
