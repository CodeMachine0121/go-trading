# Contract Traceability Matrix — 現貨交易日誌

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/`（domain／service／application／controller／persistence）
Oracle: Acceptance Criteria（42 個情境）＋ 核心業務規則（8 條）

路徑前綴：`D`＝`internal/domain/models/domains/`、`S`＝`internal/domain/service/`、`A`＝`internal/application/`、`T`＝各層 `tests/`。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | US-01 第一筆叫開倉 | 第一筆以開倉稱呼、價格為開倉價 | D/contract_trade_record_domain.go 開倉用詞；UL-MAP 合約開平倉紀錄（第一筆 entry＝開倉） | T/contract_trade_record_domain_test.go `NeverSpeaksOfFills`、`TestNewOpeningContractTradeRecordDomain` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | US-01 之後同方向叫加倉 | 同方向第二筆稱為加倉 | 同上（第二筆起的 entry＝加倉，由用詞地圖決定；各畫面依此顯示） | T/contract_trade_journal_application_test.go 開倉後加一筆 entry | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | US-01 部分反向叫減倉 | 部分反向稱為減倉、價格為平倉價 | D/trade_ledger_domain.go:162 用詞 平倉／持倉 | T/contract_trade_record_domain_test.go `NamesPositionsAsTheVenueDoes` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | US-01 讓持倉歸零的叫平倉 | 持倉歸零、交易已平倉 | D/contract_trade_record_domain.go settle | T/contract_trade_record_domain_test.go `ClosesWhenFlat` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | US-01 合約交易不再出現成交價 | 說明與拒絕訊息都不出現「成交」「成交價」 | D/contract_trade_record_domain.go、D/contract_trade_errors.go | T/contract_trade_record_domain_test.go `NeverSpeaksOfFills`（九種拒絕逐一檢查） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | US-02 新增一筆台股交易 | 持有中、市場台股、新台幣 | S/spot_trade_journal_service.go:51；D/spot_trade_market_domain.go:31 | T/spot_trade_journal_application_test.go 台股建立 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | US-02 新增一筆加密貨幣現貨交易 | 持有中、市場加密貨幣、USDT | 同上 | 同上 加密建立 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | US-02 同一標的已有持有中 | 拒絕，說明已有持有中的 #5，請在那一筆加買進 | S/spot_trade_journal_service.go:51；D/spot_trade_errors.go | 同上 已持有指向 #5 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | US-02 現貨沒有槓桿 | 拒絕，說明現貨只有先買後賣、沒有槓桿 | D/spot_trade_record_domain.go:34 | T/spot_trade_record_domain_test.go、controller 400 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | US-02 現貨不能先賣 | 拒絕，說明第一筆必須是買進 | D/spot_trade_record_domain.go:34 | T/spot_trade_record_domain_test.go selling first | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | US-02 台股數量必須是整數股 | 拒絕，說明台股以股計、必須是整數 | D/spot_trade_market_domain.go:40 | 同上 part of a Taiwan share | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | US-02 不認得的現貨標的 | 拒絕，說明找不到這個現貨標的 | S/spot_trade_journal_service.go:51 | T/spot_trade_journal_application_test.go unknown symbol | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | US-02 別人的現貨交易一律找不到 | 回覆「找不到這筆交易」 | S/spot_trade_journal_service.go:338 | 同上 somebody else's trade | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | US-03 部分賣出 | 仍持有中，剩 600 股 | D/spot_trade_record_domain.go:99 | T/spot_trade_record_domain_test.go selling part | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | US-03 全部賣出即平倉 | 已平倉 | D/spot_trade_record_domain.go:150 | 同上 selling everything | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | US-03 賣出超過持有 | 拒絕，說明賣出數量超過目前持有 600 | D/trade_ledger_domain.go:162 | 同上 selling more | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | US-03 手續費留白即零 | 手續費為 0 | S/spot_trade_journal_service.go:323 | T/spot_trade_journal_application_test.go 台股建立（fee 為 0） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | US-03 台股賣出的證交稅併入手續費 | 手續費以填的金額為準 | 同上 | 同上 全部賣掉帶手續費 2100 → 淨損益 67900 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | US-03 手續費不得為負 | 拒絕，說明手續費不得為負 | D/trade_ledger_domain.go:162 | T/trade_ledger_domain_test.go a negative fee | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | US-03 已平倉不得再加買賣 | 拒絕，說明已經平倉 | D/spot_trade_record_domain.go:99 | T/spot_trade_record_domain_test.go LocksAClosedTrade | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | US-04 平倉後的損益、報酬率與 R | 淨損益 +67,900、報酬率 +6.47%、R +1.36 | D/spot_trade_outcome_domain.go:20 | T/spot_trade_outcome_domain_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | US-04 沒有計畫止損 | 報酬率照常、R 說未設止損 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | US-04 止損放錯邊 | 拒絕，說明止損必須低於買進價 | D/spot_trade_record_domain.go:205 | T/spot_trade_record_domain_test.go、application | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | US-04 平倉後計畫鎖定 | 拒絕，說明已鎖定、可加附註 | D/spot_trade_record_domain.go:181 | 同上 changing the plan | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | US-04 最大不利與最大有利 | 最大不利價 1,020、最大有利價 1,150 與各自浮動損益 | D/spot_trade_outcome_domain.go:20；S detailOf | T/spot_trade_outcome_domain_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | US-04 沒有行情就算不出 | 顯示沒有行情資料、無法計算 | 同上 | 同上 no candles | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | US-04 持有中的浮動損益，沒有資金費用與強平價 | 以最新價估浮動損益；結果沒有資金費用與強平價 | D/spot_trade_outcome_domain.go:61；dto 沒有那兩欄 | T/spot_trade_outcome_domain_test.go；controller 回應不含 funding | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | US-05 依市場分組 | 台股與加密貨幣兩組，金額不跨幣別加總 | D/spot_trade_statistics_domain.go:44 | T/spot_trade_statistics_domain_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | US-05 平均 R 只用有止損的交易 | 平均 R 以 3 筆計並標示 | D/spot_trade_statistics_domain.go:60 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | US-05 失誤成本只算在現貨統計 | 現貨統計出現「追價進場」；合約統計不受影響 | 同上；合約統計只讀合約交易 | 同上 mistakes | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | US-05 期間內沒有平倉的現貨交易 | 兩組都說沒有、勝率不適用 | 同上 | 同上 nothing closed | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | US-06 現貨交易可以貼既有標籤 | 現貨交易帶著「追價進場」 | S/spot_trade_journal_service.go:157；D/trade_tag_selection_domain.go | T/spot_trade_journal_application_test.go review with mistake tag | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | US-06 刪除標籤時兩本一起算 | 拒絕，說明還有 1 筆交易貼著 | S/trade_journal_setting_service.go:157 | T/trade_journal_setting_application_test.go spot only | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | US-07 合約機器人連結導到合約日誌 | 打開合約日誌新增頁 | D/trade_journal_link_domain.go:34 | T/trade_journal_link_domain_test.go、contract bot run | asserts-oracle | produces-oracle | ✅ conforms |
| AC-35 | US-07 現貨機器人買入的連結預填新增 | 預填 2330、1,050、100 股、止損、止盈、策略；標示請改成實際成交 | D/spot_trade_prefill_domain.go:28 | T/spot_trade_prefill_domain_test.go；bot run 現貨連結 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | US-07 現貨機器人買入但沒有建議部位 | 預填標的與買進價，數量留空 | 同上 | 同上 without a suggestion | asserts-oracle | produces-oracle | ✅ conforms |
| AC-37 | US-07 現貨機器人出場，已有持有中 | 對 #5 加賣出，賣出價＝參考價、數量 600 股 | 同上 | 同上 exit while holding | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | US-07 現貨機器人出場，沒有持有中 | 說明沒有持有中並提供新增 | 同上（noOpenHolding） | 同上、controller | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | US-07 買入時已有持有中 | 對 #5 加買進 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-40 | US-07 別人的或已被刷掉的連結 | 找不到／已不在紀錄中 | S/trade_journal_link_service.go:63 | T/trade_journal_link_service_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| AC-41 | US-07 現貨機器人其他訊息一字不變 | 只在最後多一條連結 | D/strategy_bot_message_domain.go（有網址才加） | T/strategy_bot_run_application_test.go `LinksASpotRoundToTheSpotJournal`（訊息以連結結尾、其餘照舊） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-42 | US-08 現貨對照 | 同期間重演、並排筆數與勝率、不計成本 | D/spot_trade_live_comparison_domain.go；A/spot_trade_live_comparison_application.go:35 | T/spot_trade_prefill_domain_test.go（成本為零）、application comparison | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 現貨交易紀錄與合約完全獨立，同代號各自記 | 同代號的現貨與合約可同時持有 | 兩張表、兩個部分索引 | T/spot_trade_record_repository_test.go `KeepsOneOpenTradePerSymbol`（合約同代號可建立） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 賣出不早於第一筆買進、時間不在未來 | 拒絕並說明 | D/trade_ledger_domain.go:162 | T/trade_ledger_domain_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 持有中可修正或刪除買賣（至少留一筆買進）、平倉後鎖定 | 修正後均價重算；刪最後一筆買進被拒 | D/spot_trade_record_domain.go | T/spot_trade_record_domain_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 報酬率＝淨損益 ÷ 買進成本 | 67,900 ÷ 1,050,000 | D/spot_trade_outcome_domain.go:20 | T/spot_trade_outcome_domain_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 預填數量：台股往下取整數股、加密往下取 8 位小數 | 105,999 ÷ 1,050 → 100；1,000 ÷ 97,900 → 0.01021450 | D/spot_trade_market_domain.go:49 | T/spot_trade_record_domain_test.go `TestSpotTradeMarketDomain` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 報酬率分布六格 | 各報酬率落在對應格 | D/spot_trade_statistics_domain.go | T/spot_trade_statistics_domain_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 連結打錯本日誌時說明屬於另一本，建立交易時不抄來源 | 開連結 404 並說明；建立交易沒有來源 | S/spot_trade_journal_service.go:272、:293；合約同 | T/spot_trade_journal_application_test.go、T/contract_trade_journal_application_test.go | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 現貨對照每組另附平均進場滑點，整份策略也有 | 每列與整份各有平均滑點與筆數 | D/spot_trade_live_comparison_domain.go | T/spot_trade_prefill_domain_test.go（0.5 與 0.3） | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| S/spot_trade_journal_service.go ListTrades | 依狀態、標的、市場、期間篩選與筆數上限（沿用合約日誌的列出規則） | undocumented in this PRD；延續合約日誌既有行為，建議 PRD 補一條 |
| D/trading_strategy_names_domain.go | 名字讀不到時不把任何交易標成策略已刪除 | undocumented；沿用合約日誌既有規則 |

## Summary

- Conforms: 50/50 clauses ✅ (100%)
- Violations: —
- Mis-asserted: —（第一輪 AC-5 為 shallow、AC-41 無應用層測試，已補測試後重判）
- Partial: —
- Gaps: —
- Unclear: —
- Orphans: 2（皆為沿用合約日誌的既有行為）

說明：這是對驗收情境的靜態符合度稽核，依規格推導的預期結果判斷測試斷言與程式路徑，不執行新造的情境。每筆交易稱為開倉／加倉／減倉／平倉的「標籤」由畫面依紀錄種類與順序呈現，交易服務負責的是紀錄本身與所有說明、拒絕訊息的用詞。
