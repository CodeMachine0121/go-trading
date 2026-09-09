# Contract Traceability Matrix — 台灣期貨夜盤接入

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/models/{vo,domains}`, `internal/domain/service`, `internal/infrastructure/marketdata`, `internal/config`, `cmd/server`
Oracle: Acceptance Criteria (50 scenarios) + Core Business Rules (12) + Non-Functional (3) = 65 clauses

> **Re-run:** first pass found 2 mis-asserted and 3 partial clauses plus 2 orphans. All
> five were closed with tests, and both orphans became contract clauses (AC-8.8, BR-12).

> **Ceiling:** static conformance audit. Each clause's expected outcome was written from
> the PRD alone, then the test assertion and the code path were judged against it
> separately. No scenario was invented and executed; the single mapped test was run as
> corroboration only.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1.1 | 記著屬於台灣期貨的交易標的向台灣期貨的來源拿 | 抓台指期時，被叫到的是台灣期貨那個來源 | `dependencies.go:307`, `market_routed_market_data_proxy.go:41` | `dependencies_test.go:TestEveryRecognisedMarketHasItsSourcesWiredUp` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-1.2 | 台股維持原樣 | 抓 2330 時被叫到的是台股來源，行為與改動前相同 | `market_routed_market_data_proxy.go:41` | `market_routed_proxies_test.go:15` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-1.3 | 加密貨幣維持原樣 | 抓 BTCUSDT 時被叫到的是加密貨幣來源 | 同上 | `market_domain_test.go:122`, `market_routed_proxies_test.go:15` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-1.4 | 沒記市場的舊資料仍然是加密貨幣 | 舊資料照加密貨幣的規則走 | `market_catalog_domain.go:69` | `market_domain_test.go:61` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-1.5 | 指名一個系統不認得的市場 | 拒絕，並說出認得哪三個市場 | `watchlist_entry_domain.go:40` | `market_domain_test.go:85`,`:1022` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.1 | 日盤進行中照常抓 | 10:07 那一輪抓台指期，並存入 10:06 那一根 | `market_domain.go:71`, `k_candle_ingestion_service.go:266` | `k_candle_ingestion_service_test.go:TestADayBoardRoundStoresTheCandleItCollected` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.2 | 夜盤進行中照常抓 | 22:30 那一輪抓台指期，並存入 22:29 那一根 | 同上 | `k_candle_ingestion_service_test.go:TestAnEveningBoardRoundStoresTheCandleItCollected` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.3 | 兩段之間的休息跳過 | 台指期跳過、不留失敗紀錄；加密貨幣照常抓 | `market_domain.go:71` | `k_candle_ingestion_service_test.go:1196`,`:780` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.4 | 夜盤收了到日盤開之前的清晨跳過 | 台指期跳過、不留失敗紀錄 | 同上 | `market_domain_test.go:667`,`:796`; `k_candle_ingestion_service_test.go:735` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.5 | 日盤剛收的那一輪還收得到當段最後一根 | 存入 13:44；之後那幾輪跳過 | `market_domain.go:425` | `market_domain_test.go:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.6 | 夜盤剛收的那一輪還收得到當段最後一根 | 存入 04:59；之後那幾輪跳過 | 同上 | `market_domain_test.go:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.7 | 週末整天跳過 | 台指期跳過；加密貨幣照常抓 | `market_domain.go:462` | `market_domain_test.go:667`,`:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2.8 | 交易時段內取不到才算失敗 | 留下一筆失敗紀錄，下一輪自然重試 | `k_candle_ingestion_service.go:266` | `fubon_market_data_proxy_test.go:243`; `k_candle_ingestion_service_test.go:836` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3.1 | 夜盤開始那半段屬於哪個交易日 | 週三 15:00 那段 → 週四 | `market_domain.go:493` | `market_domain_test.go:707` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3.2 | 過了午夜仍是同一個交易日 | 週四 02:00 那根 → 週四，與週三 22:00 同一段 | 同上 | `market_domain_test.go:707` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3.3 | 週五傍晚那段算下週一 | → 下週一 | `market_domain.go:510` | `market_domain_test.go:707` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3.4 | 日盤算它自己那一天 | 週四 10:00 → 週四 | `market_domain.go:493` | `market_domain_test.go:707` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3.5 | 夜盤已收、日盤未開那段清晨的交易日 | 週四 08:00 → 週四 | `market_domain.go:239` | `market_domain_test.go:707`,`:325` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4.1 | 來源正常回覆卻一根都沒有 | 推定今天日盤休市，當天剩餘輪次不再抓日盤 | `k_candle_ingestion_service.go:395`, `market_closure_ledger.go:38` | `k_candle_ingestion_service_test.go:1146` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4.2 | 日盤推定休市，夜盤照樣問一次 | 夜盤時段仍向來源要 K 線 | `market_closure_ledger.go:65` | `k_candle_ingestion_service_test.go:1146` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4.3 | 隔天重新判定 | 隔天同一段照樣問，不沿用昨天的推定 | 同上 | `k_candle_ingestion_service_test.go:1173` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4.4 | 來源報錯不算休市 | 留失敗紀錄、不推定休市，下一輪重試 | `k_candle_ingestion_service.go:354` | `k_candle_ingestion_service_test.go:836` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4.5 | 只針對一檔的回補不推定休市 | 不推定任何一段休市 | `market_closure_ledger.go:55` | `k_candle_ingestion_service_test.go:1025`,`:1054` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5.1 | 平常日子跟最近到期那一口 | 拿最先到期那一口的 K 線 | `fubon_listed_contracts.go:48`, `fubon_wire.go:37` | `fubon_market_data_proxy_test.go:129` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5.2 | 換月之後跟新的那一口 | 抓新合約，K 線接在同一條線上 | `fubon_market_data_proxy.go:72` | `fubon_market_data_proxy_test.go:147` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5.3 | 還沒換月時跟舊的那一口 | 抓原本那一口 | `fubon_listed_contracts.go:48` | `fubon_market_data_proxy_test.go:129`,`:371` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5.4 | 換月前後查得到一條連得起來的線 | 由早到晚一條序列；跳空照原樣、不做價格還原 | `fubon_market_data_proxy.go:100` | `fubon_market_data_proxy_test.go:147`,`:167` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5.5 | 加進觀察清單前先確認它存在 | 先向來源確認，確認後加入 | `fubon_symbol_lookup_proxy.go:60` | `fubon_market_data_proxy_test.go:327` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5.6 | 來源不可用就加不進去 | 加不進去並說明稍後再試，清單維持原狀 | `fubon_symbol_lookup_proxy.go:60` | `fubon_market_data_proxy_test.go:358` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5.7 | 來源答不出最先到期那一口就算這一輪失敗 | 留失敗紀錄，不推定休市 | `fubon_market_data_proxy.go:78` | `fubon_market_data_proxy_test.go:228`; `k_candle_ingestion_service_test.go:836` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6.1 | 台指期的 K 線缺那三個數字 | 開高低收與成交量都有值；另三個沒有值 | `fugle_wire.go:36` | `fubon_market_data_proxy_test.go:206` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6.2 | 成交量真的是零與沒有這一項不同 | 成交量是零，成交額仍然沒有值 | `fugle_wire.go:36` | `fubon_market_data_proxy_test.go:TestAFuturesVolumeOfZeroIsAFigureRatherThanAnAbsentOne` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6.3 | 成交量照原樣存 | 不換算成任何其他單位 | 同上 | `fubon_market_data_proxy_test.go:206` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6.4 | 完全沒有成交的那一分鐘沒有那一根 | 沒有那一根，不補洞也不沿用前一根 | `k_candle_ingestion_service.go:266` | `k_candle_ingestion_service_test.go:196` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6.5 | 加密貨幣的三個數字照常都有 | 三個數字都有值 | `binance_*`（未改動） | `binance_market_data_proxy_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7.1 | 一個完整交易日，一小時一格 | 20 格 | `market_domain.go:125` | `market_domain_test.go:871` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7.2 | 同一天四小時一格，兩段共用那一格只算一次 | 6 格，不是 7 | `market_domain.go:139` | `market_domain_test.go:871`,`:1035` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7.3 | 兩段之間的休息不產生格 | 0 格 | `market_domain.go:425` | `market_domain_test.go:871` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7.4 | 週末完全沒有交易 | 0 格 | 同上 | `market_domain_test.go:871` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7.5 | 查一段完全沒有交易的時間不算錯誤 | 回一個空的序列，不是拒絕 | `market_domain.go:163` | `market_domain_test.go:935`; `k_candle_series_*` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7.6 | 台股的格數一格都不變 | 與改動前相同的格數 | `market_domain.go:125` | `market_domain_test.go:406` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7.7 | 加密貨幣的格數一格都不變 | 與改動前相同的格數 | `market_domain.go:133` | `market_domain_test.go:501` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.1 | 夜盤進行中啟動，跨過午夜照補 | 補 23:31 到 01:59 交易時段內的每一根 | `market_domain.go:71` | `market_domain_test.go:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.2 | 夜盤已收、日盤未開時啟動 | 補到 04:59 為止，之後不算缺口 | 同上 | `market_domain_test.go:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.3 | 週末啟動，補到週五夜盤的尾 | 補到週六 04:59 為止 | 同上 | `market_domain_test.go:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.4 | 兩段之間的休息不算缺口 | 補到 13:44 為止 | 同上 | `market_domain_test.go:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.5 | 回補範圍內完全沒有交易時段 | 沒有東西要補，直接進入定時抓取 | `market_domain.go:78` | `k_candle_ingestion_service_test.go:935`; `market_domain_test.go:796` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.6 | 從來沒有任何 K 線 | 補回補範圍內交易時段內的每一根 | `k_candle_ingestion_service.go:266` | `k_candle_ingestion_service_test.go:917` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.7 | 加密貨幣的回補維持現狀 | 補那兩小時的每一根 | `market_domain.go:66` | `market_domain_test.go:242` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 一天可以有兩段交易時間，其中一段可以跨過午夜 | 兩段都被認得，跨午夜那段連續 | `market_rules_vo.go` `TradingSessionVo.Stretches` | `market_domain_test.go:667` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 落在哪一段之外就是非交易時段 | 13:45–15:00 與 05:00–08:45 一律跳過、不算失敗 | `market_domain.go:44` | `market_domain_test.go:667`; `k_candle_ingestion_service_test.go:1196` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 夜盤的營業日歸屬是次一個營業日 | 週五傍晚那段算下週一 | `market_domain.go:493` | `market_domain_test.go:707` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 休市推定的單位是一段交易時間 | 一段的沉默只是那一段的 | `market_closure_ledger.go:28` | `k_candle_ingestion_service_test.go:1146` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 「最近到期那一口」每一輪向來源問，不算月曆也不算結算時刻 | 來源不再列出，就是換月那一刻；不快取 | `fubon_listed_contracts.go:48` | `fubon_market_data_proxy_test.go:147` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 換月前後接成同一條線，跳空照原樣呈現 | 同一條線，不做價格還原、不標記 | `fubon_market_data_proxy.go:100` | `fubon_market_data_proxy_test.go:147` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 圖上有幾根：兩段共用的格子只算一次 | 一小時 20 格、四小時 6 格 | `market_domain.go:139` | `market_domain_test.go:871` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 缺少的成交數字沒有值，不是零 | 三個數字沒有值 | `fugle_wire.go:36` | `fubon_market_data_proxy_test.go:206` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 兩段交易時間的起訖可調整 | 換了設定就換了時段 | `application_config.go:374` | `ingestion_config_test.go:172`,`:196` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 抓取輪次與前兩個市場共用同一輪 | 台灣期貨不另開一輪 | `dependencies.go` `buildBackgroundJobs` | `k_candle_ingestion_service_test.go:644` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 台灣期貨的即時跟盤沿用「有跟盤上限的市場走固定名單」 | 期貨照名單跟，不另立一套 | `live_follow_roster_domain.go`, `dependencies.go:325` | `live_follow_roster_domain_test.go:TestTheFuturesMarketHandsOutItsPlacesFromARosterToo` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 即時跟盤時，有兩段交易時間的市場要向來源同時要兩段 | 同一個代號送出兩份訂閱，其中一份是盤後那一段 | `fugle_live_market_data_proxy.go:157` | `fugle_live_market_data_proxy_test.go:531` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8.8 | 停機超過今天，更早的補不回來 | 補回今天兩段握著的每一根；更早的補不回來且不算失敗 | `fubon_market_data_proxy.go:113`（只有日內位址） | `fubon_market_data_proxy_test.go:189` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 一輪的時間不因此明顯變長：各標的同時進行 | 每個交易標的各自併發、彼此獨立 | `k_candle_ingestion_service.go:236` | `k_candle_ingestion_service_test.go:644` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 「有幾根」不得逐格走訪 | 任意長的一段仍答得出來 | `market_domain.go:139` | `market_domain_test.go:619` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 跳過不得留下失敗紀錄 | 跳過那一輪沒有失敗紀錄 | `k_candle_ingestion_service.go:266` | `k_candle_ingestion_service_test.go:1196` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| （無） | 兩個 orphan 已收進契約：訂兩段成為 BR-12、回補上限成為 AC-8.8 | reconciled |

## Summary

- Conforms: 65/65 clauses ✅ (100%)
- Violations: （無）
- Mis-asserted: （無）—— 第一輪的 AC-2.1、AC-2.2 已補上端到端存入的斷言
- Partial: （無）—— 第一輪的 AC-1.1、AC-6.2、BR-11 已補上測試
- Gaps: （無）
- Unclear: （無）
- Orphans: 0 —— 兩個都已收進契約

### 第一輪找到、已修掉的東西

| 發現 | 為什麼是洞 | 怎麼補的 |
| :--- | :--- | :--- |
| AC-2.1／AC-2.2 只斷言到窗口與跳過 | 「存入那一根」整條路（輪次 → 服務 → 來源 → 存檔）對期貨沒有任何測試走過。夜盤是這個切片的全部理由，卻只被問到、沒被存到 | 各補一個端到端測試，斷言存進去的就是 10:06 與 22:29 那一根 |
| AC-1.1 沒有測試證明期貨被接上 | 三張路由表的內容只在組裝根，而組裝根沒有測試。少接一條線的症狀是「這個市場安靜地什麼都沒有」 | 補一個組裝根測試：每一個認得的市場，三個來源都要接得上。已用「拆掉其中一筆」驗證它會紅 |
| AC-6.2 期貨沒有零成交量的案例 | 零是合法的成交量，「這個市場沒有這一項」是另一件事；兩者寫成一樣，指標會算出一串看起來合理的錯數字 | 補一個零成交量的案例，同時斷言成交額仍然沒有值 |
| BR-11 期貨的跟盤名單沒有測試 | 名單機制是通用的，但沒有任何案例證明期貨走得進去 | 補一個期貨名單案例，時間點落在只有這個市場才有的夜盤 |
| 組裝根測試第一版是空斷言 | 三張路由表的「沒接上」訊息其實是同一句，測試卻對其中兩張比對了不存在的字串——拆掉線也不會紅 | 改成比對三張共用的那一句，並重新驗證三張表各拆一次都會紅 |
