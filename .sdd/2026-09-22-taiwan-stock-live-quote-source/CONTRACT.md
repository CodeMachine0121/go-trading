# 台股即時行情改由證交所提供 — Contract Verification

**Contract source:** `.sdd/2026-09-22-taiwan-stock-live-quote-source/PRD.md`（Acceptance Criteria 為 oracle）
**Design map:** `ARCH.md` · **Glossary:** `.sdd/UL-MAP.md`
**Kind:** static conformance audit — 以 spec 推導的期望值分別審查「測試有沒有斷言它」與「程式有沒有產出它」，**不以測試綠燈為判準**，也不執行本審查自行發明的情境。

---

## Clauses

| ID | Clause（節錄） | Oracle（先於讀碼，僅由 spec 推導） | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-1 | 八檔台股全部都在跟盤名單上 | 觀察清單八檔台股、交易時段內 → 八檔全部在名單上且都拿得到即時更新 | `live_follow_roster_domain.go:63,86` | `live_follow_roster_domain_test.go:119`、`k_candle_follow_service_test.go:1757` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 只有一檔時照樣跟 | 一檔 → 該檔在名單上，不走別的路 | `live_follow_roster_domain.go:86` | `live_follow_roster_domain_test.go:134` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 一檔台股都沒有時不向證交所問任何東西 | 名單為空 → 不開通道、不發出任何詢問 | `live_follow_roster_domain.go:131`（空名單→空 channels） | `live_follow_roster_domain_test.go:134` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 名單超過一次問得了的檔數時分成數次問 | 超過上限 → 分成數輪，**每一檔都跟得到**，觀看者看不出被分批 | `live_follow_roster_domain.go:142-145` | `live_follow_roster_domain_test.go:159` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 收盤時段一檔都不跟 | 收盤 → 名單為空、一檔不跟 | `live_follow_roster_domain.go:67` | `live_follow_roster_domain_test.go:181` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 不在觀察清單上的台股仍被告知沒有即時更新 | 看一檔不在清單上的台股 → 看得到圖，且**被告知沒有即時更新** | `live_follow_roster_domain.go:167`、`k_candle_follow_service.go:121` | `live_follow_roster_domain_test.go:193`、`k_candle_follow_service_test.go:1773` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 證交所報的量原樣記下 | 累計一千張 → 記下一千張（不換算） | `twse_realtime_wire.go:20,120` | `twse_forming_k_candle_test.go:183` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 零就是零 | 累計零張 → 零，且是確定的零而非「沒有值」 | `twse_realtime_wire.go:120`（`Volume` 為必填 `decimal.Decimal`） | `twse_forming_k_candle_test.go:183`（起點 `0` 參與差分） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 歷史與即時的成交量是同一個數字 | 同一分鐘兩邊**同一單位、同一數字** | 即時 `twse_realtime_wire.go`（原樣帶過）；歷史 `fugle_wire.go`（原樣帶過，未改動） | `taiwan_stock_volume_unit_test.go:28` | asserts-oracle — 同一分鐘同時走兩個來源並排比對 | produces-oracle | ✅ conforms |
| AC-10 | 累計增加多少就是這一分鐘的量 | 起點 500 張、現在 530 張 → 這一根 30 張 | `twse_forming_k_candle.go:192` | `twse_forming_k_candle_test.go:167` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 累計沒有增加就不送 | 累計未動 → 不送出進行中那一根，且不算失敗 | `twse_forming_k_candle.go:194` | `twse_forming_k_candle_test.go:196` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 一根走完時存入的是這一分鐘的量 | 該分鐘 500→560 張 → 走完那根 60 張 | `twse_forming_k_candle.go:179,192` | `twse_forming_k_candle_test.go:214` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 累計倒退時重新起算而不產出負數 | 起點 500、現在 20 → 重新起算，**不產出負的成交量** | `twse_forming_k_candle.go:163-171` | `twse_forming_k_candle_test.go:240` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 整天沒有成交就整天不送 | 整個交易時段累計為零 → 從頭到尾不送出那一檔進行中的那一根 | `twse_realtime_wire.go:96`、`twse_forming_k_candle.go:194` | `twse_realtime_live_market_data_proxy_test.go:66,184` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 歷史仍向富果取 | 自動抓取開始 → 向**富果**取回走完的 K 線 | `cmd/server/dependencies.go:670`（取數 map 未動） | — | **no-test**（組裝根的接線，本切片無測試斷言台股歷史仍指向富果） | produces-oracle | 🟡 partial |
| AC-16 | 即時向證交所取 | 交易時段內送出即時更新 → 內容來自**證交所** | `cmd/server/dependencies.go:694` | — | **no-test**（同上） | produces-oracle | 🟡 partial |
| AC-17 | 證交所問不到時歷史照常補齊 | 告知即時已停止，且**自動抓取照常**從富果補齊 | 兩張 map 無共用狀態；`twse_realtime_live_market_data_proxy.go:101`（feed 收線） | `twse_realtime_live_market_data_proxy_test.go:235`（只涵蓋「feed 收線」半邊） | **shallow** — 未斷言「歷史照常」那一半 | produces-oracle | 🟡 partial |
| AC-18 | 富果取不到時即時照常 | 歷史不可用 → 即時照常來自證交所 | 同上（結構獨立） | — | no-test | produces-oracle | 🟡 partial |
| AC-19 | 成交最慢約八秒反映到畫面 | 剛成交 → 最多八秒內反映在進行中那一根 | `twse_realtime_live_market_data_proxy.go:108`（詢問間隔）＋設定預設 `3` 秒 | `ingestion_config_test.go:106`（只斷言間隔預設值） | **shallow** — 端到端的八秒含外部來源自身的五秒，repo 內無法斷言 | produces-oracle（可設定的那一半） | 🟡 partial |
| AC-20 | 延遲不影響走完那一根的內容 | 該根以**正確的最終樣子**存入——延遲只影響何時看到 | `twse_forming_k_candle.go:76`（以成交時間而非到達時間歸位） | `twse_forming_k_candle_test.go:286` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 策略判斷不受延遲影響 | 判斷依據是走完後的最終樣子，結果不因延遲而不同 | 既有規則，本切片未改動 | — | no-test（本切片未新增；既有行為由策略側測試涵蓋） | produces-oracle | 🟡 partial |
| BR-1 | 跟盤範圍＝觀察清單上的全部台股，不再有上限、不依登錄先後挑選 | 全部台股進名單；非交易時段為空 | `live_follow_roster_domain.go:63-88` | `live_follow_roster_domain_test.go:119,181` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 分批詢問對觀看者沒有差別 | 超過上限分批，**每一檔都跟得到** | `live_follow_roster_domain.go:142` | `live_follow_roster_domain_test.go:159` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 成交量**不做任何換算**，兩個台股來源同為「張」 | 程式裡不存在成交量的比例換算 | `twse_realtime_wire.go`（原樣帶過） | `taiwan_stock_volume_unit_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 每分鐘成交量＝本次累計−分鐘起點累計；沒增加不送、倒退重新起算 | 三種情形各自有明確結果，皆不產生負數 | `twse_forming_k_candle.go:163,192,194` | `twse_forming_k_candle_test.go:167,196,240` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 新鮮度：來源約五秒更新、系統每三秒問一次，最慢約八秒 | 同 AC-19 | `twse_realtime_live_market_data_proxy.go:108` | `ingestion_config_test.go:106` | shallow（同 AC-19） | produces-oracle | 🟡 partial |
| BR-6 | 不變的事：名單改變重建通道、中斷告知並重新跟上、重試逐次拉長至多三十秒不放棄、一根走完即存入、更新間隔節流 | 這些規則一條都不改 | `k_candle_follow_service.go`（僅 121 行那一個分支改動） | `k_candle_follow_service_test.go`（既有回歸測試全數維持綠燈） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 即時最慢約八秒；向證交所不超過每分鐘二十次；分批不得讓任一檔的更新間隔超過「詢問間隔×批數」 | 三項皆成立 | 間隔／節奏：`dependencies.go:655`、`application_config.go`；分批：每條通道各自一個 goroutine 與 ticker，故實際間隔為**詢問間隔本身**，嚴於要求 | `ingestion_config_test.go:106`（只涵蓋前兩項的設定值） | shallow — 第三項無測試 | produces-oracle | 🟡 partial |
| NFR-2 | 不引入任何新的機密設定 | 證交所路徑不需要金鑰 | `twse_realtime_live_market_data_proxy.go:147-208`（無任何金鑰欄位／標頭） | `ingestion_config_test.go:106` | shallow — 斷言網址非空，未斷言「無金鑰」 | produces-oracle | 🟡 partial |
| NFR-3 | 加密貨幣與永續合約行為一律不變 | 兩者的跟盤方式與抓取完全不受影響 | `application_config.go`（crypto 規則為零值）、合約路徑未觸及 | `live_follow_roster_domain_test.go:77`、`ingestion_config_test.go:153` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 即時來源問不到時留下紀錄，內容需足以分辨「對方擋下」與「對方沒答話」 | 兩種失敗留下**可區分**的紀錄 | `twse_realtime_live_market_data_proxy.go:160`（擋下：`market source answered %d`）、`:157`（沒答話：`reach market source`）、`:200`（body 內拒絕：`market source refused`）；由 `:120` 寫入紀錄 | `twse_realtime_live_market_data_proxy_test.go:109,163` | asserts-oracle — 斷言三種原因彼此不同，且各自說得出來源實際說了什麼 | produces-oracle | ✅ conforms |

---

## Orphans

| # | 行為 | 位置 | 判定 |
| :--- | :--- | :--- | :--- |
| O-1 | 明確要求未延遲報價（`delay=0`） | `twse_realtime_live_market_data_proxy.go:155` | 合約未提及。屬防禦性實作：來源預設值曾變動，安靜變成延遲報價看起來會像市場很安靜。**建議補進 PRD 的核心業務規則**，非違規 |
| O-2 | 收線／取消時的三種結束路徑（等待下一輪、送出受阻、位址不可達） | `twse_realtime_live_market_data_proxy.go:101,147`；測試 `:254,275,319,351` | 由 `ILiveMarketDataProxy` 介面契約要求（「關閉 channel 是唯一的結束告知」），非本 PRD 條款。**不屬 Out of Scope**，記錄即可 |
| O-3 | `HasFollowCeiling()` 目前無任何市場回 `true` | `market_domain.go:244` | 保留決策已寫入 `ARCH.md` §6 並附回頭處理的訊號。非違規 |

**Out of Scope 反向檢查**：六項（改歷史來源、改加密貨幣跟盤、升級富果方案、把延遲壓到一秒內、用證交所補歷史、取用五檔買賣價量）逐一比對程式碼，**無任何一項被實作**。無範圍外溢。

---

## Summary

| 判定 | 數量 |
| :--- | :--- |
| ✅ conforms | 22 |
| 🔴 violation | **0** |
| 🟠 mis-asserted | **0** |
| 🟡 partial | 9 |
| ❌ gap | **0** |
| ❔ unclear | 0 |
| ⚠️ orphan | 3（皆非違規） |
| **合計** | **31** |

**Conformance: 71.0%（22 / 31）**

沒有任何一條的**程式行為**是錯的——31 條全部 `produces-oracle`。

**本次審查發現的兩條 `mis-asserted` 已修正**（兩個新測試都以變異確認過會紅）：

- `NFR-4` 原本只斷言「有錯誤」。把「被擋下／body 內拒絕／連不上」三句話改成同一句，測試照樣綠——現在會紅。
- `AC-9` 原本沒有任何測試同時拿兩個來源比對，而那正是本切片最容易靜默出錯的地方。新測試讓同一分鐘各走一次富果與證交所並比對成交量。

  **它第一版寫錯了**：fixture 把「富果以股、證交所以張」這個未經查證的前提寫死在兩側，於是它驗證的是那個前提而不是系統。盤中實測推翻前提後已重寫——兩側現在餵**同一個數字**，任何一側被縮放都會紅（實測：重新加回 ×1000 會殺掉 9 個測試）。

**剩下九條 partial**，程式行為皆正確，缺的是測試證明力，且各有原因：

- **AC-15／AC-16**（歷史走富果、即時走證交所）是組裝根的接線。要斷言它需要一個涵蓋 `dependencies.go` 的測試，本專案目前沒有這一層測試，補之前應先決定要不要開這個層級。
- **AC-17／AC-18**（一邊不可用不影響另一邊）成立於結構獨立（兩張互不相干的 map、無共用狀態），可由閱讀確認，但沒有單一測試同時驅動兩條路。
- **AC-19／AC-21／BR-5／NFR-1／NFR-2** 含外部來源自身的更新週期與「沒有新機密設定」這類否定性質，repo 內斷言不了或斷言成本遠高於價值。

以上九條建議**留作已知狀態**而非補測試：它們不是疏漏，是測試邊界的誠實位置。


---

## Amendment — 2026-09-23

盤中對實際來源查證後，本矩陣有兩處判定必須推翻：

| ID | 原判定 | 更正後 |
| :--- | :--- | :--- |
| AC-7 | ✅ conforms（張換算成股） | **前提本身是錯的**。兩個台股來源同為「張」，換算反而製造出一千倍落差。條款與程式皆已改為「原樣記下」，現為 ✅ conforms |
| AC-9 | ✅ conforms | 當時的測試把錯誤前提寫死在 fixture 兩側，**驗的是前提不是系統**。重寫後兩側餵同一個數字，現為 ✅ conforms |

**這次審查為什麼沒抓到：** oracle 是從 PRD 推導的，而 PRD 這一條本身就寫錯了。契約驗證能證明「程式符合規格」，**不能證明規格符合現實**——後者只有真實來源答得出來。教訓寫進 ARCH §8 的更正欄。

---

## 更正 — 2026-09-23（盤中實測後）

本切片原本要「歷史問富果、即時問證交所」。**即時那一半走不通，已改為輪詢富果自己的盤中 K 線。**

盤中對證交所公開報價實測到三件事，任何一件都足以否決它：

- 快照落後 **10–60 秒**且不固定；
- **連續兩次請求之間快照會倒退**（累計量從 16897 掉回 16704），叢集各節點不一致；
- 成交價欄位在**數百張成交期間完全不更新**（`trade.t` 凍結兩分半，`v` 卻一直漲）。

用這種資料組出來的分 K，錯得沒有任何下游偵測得到——實測時收盤價就對不上（112.9 vs 112.95）、成交量差 25%。

**改法**：富果的盤中端點本來就直接給**已經成形的那一分鐘**，所以不自己折 K 線、不做累計差分、不換算單位。連帶消失的問題：成交量單位之爭、「加入當下那一分鐘殘缺」、掛牌板別推導、session cookie。

**留下的代價**：一次請求只問得了一檔，所以檔數會反映在輪詢間隔上（約 10 檔／10 秒）。仍比原本方案的 5 檔訂閱上限好，而且資料是對的。

**盤中驗證**：跟盤 2 分 10 秒，9 個走完的分鐘與來源自己的說法**逐一完全相同**（成交量與收盤價）；100 筆進行中更新（證交所版只有 8 筆）。驗證程式留在 `taiwan_stock_live_source_check_test.go`（`-tags livecheck`，只在盤中手動跑）。

**教訓**：本切片對來源的三次判斷（成交量單位、哪個欄位有價格、公開報價夠不夠新）全部錯過一次，而三次測試都是綠的——因為每一份假資料都是照我自己的假設寫的。**只有真實來源能證明規格符合現實，而且只有在它營業的時候。**
