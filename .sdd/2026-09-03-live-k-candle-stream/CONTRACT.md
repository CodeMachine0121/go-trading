# K 線即時跟盤 — Contract Verification Matrix

**Oracle:** `PRD.md` 第 3 節的 Gherkin 驗收條件（26 條）
**Oracle 紀錄:** 實作前寫於 scratchpad `oracle-live-stream.md`（獨立性關卡的證據）
**判定方式:** 靜態一致性稽核——測試對照 oracle、程式碼對照 oracle，兩邊**各自獨立**判定；
不以「測試跑綠」作為結論。

---

## 1. Clauses

| ID | 條款 | Oracle（實作前寫下） | 實作位置 | 測試 | 測試稽核 | 程式碼稽核 | 狀態 |
|---|---|---|---|---|---|---|---|
| AC-01.1 | 第一個觀看者讓系統開始跟盤 | 跟盤份數 0→1；他立刻收到目前進行中的那一根 | `k_candle_follow_service.go:88`（查無即開一份）、`symbol_follow.go:56`（join 順帶補上目前那一根） | `TestOneFollowPerSymbolNoMatterHowManyAreWatching`、`TestAViewerArrivingMidCandleIsGivenTheShapeSoFar` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.2 | 同一標的多人在看時只跟一次 | 兩人收到**同一份**內容；份數仍為 1 | `k_candle_follow_service.go:86`（map 查有即加入） | `TestOneFollowPerSymbolNoMatterHowManyAreWatching` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.3 | 還有人在看就繼續跟 | 份數仍為 1；剩下那人照常收到更新 | `symbol_follow.go:80`（不是最後一個就不回報） | `TestTheFollowEndsOnlyWhenTheLastViewerLeaves` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.4 | 最後一個觀看者離開就停止跟盤 | 份數變 0 | `k_candle_follow_service.go:155`（回報是最後一個即移除並取消） | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.5 | 不在觀察清單上的標的一樣跟得動 | 照樣開始跟；他收得到更新 | `k_candle_follow_service.go:78`（只認 symbol，完全不讀觀察清單） | `TestASymbolOffTheWatchlistIsStillFollowed` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.6 | 換看另一個就換跟另一個 | 舊的份數→0、新的→1；只收到新的更新 | 舊 context 結束→`leave`；新的 `WatchKCandles` | `TestChangingSymbolLeavesTheOldMarketBehind` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.7 | 連線斷掉就算離開 | 等同離開：份數變 0，無額外逾時判定 | `k_candle_follow_service.go:104`（等 context 結束即 leave） | `TestTheFollowEndsOnlyWhenTheLastViewerLeaves`（以 context 取消模擬）、`TestTheRequestEndsWhenTheViewerLeaves` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.1 | 剛跟上就先給目前進行中的那一根 | 立刻收到，不必等下一次變動 | `symbol_follow.go:66` | `TestAViewerArrivingMidCandleIsGivenTheShapeSoFar` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.2 | 剛跟上時那一根還沒有任何成交 | 沒有收到任何一根，且跟上本身成功 | `symbol_follow.go:65`（`hasLatest` 為否即不送） | `TestJoiningBeforeTheMarketHasTradedHandsOverNothingAndStillSucceeds` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.3 | 送出的一律是五分鐘一根 | 即使畫面看一小時一根，送的仍是五分鐘一根 | `binance_live_market_data_proxy.go:113`（訂閱 `@kline_` + 既有的 `kCandleInterval`＝`5m`） | `TestTheFeedIsOpenedForFiveMinuteCandlesOfThatSymbol` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.1 | 十秒內成交上百筆也只送一次 | 這十秒內只送出一次 | `k_candle_follow_domain.go:78` | `TestAdmitLetsAFormingCandleThroughOncePerCeiling`／`十秒內成交上百筆只送一次` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.2 | 還沒滿十秒的變動先不送 | 不送；滿十秒才送當時的樣子 | 同上 | 同上／`距上次送出僅兩秒的變動先不送` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.3 | 一根走完一定送到，不等滿十秒 | 立刻送出最終樣子 | `k_candle_follow_domain.go:78`（`!Closed &&` 是關鍵的那一半） | 同上／`距上次送出僅兩秒，那一根走完就立刻送` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.4 | 沒有變動就沒有東西要送 | 不送出任何東西 | 沒有輸入就不會呼叫 `Admit` | `TestTheFollowHoldsBackWhatTheThrottleRefuses`（服務層確有詢問節流） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.1 | 進行中的那一根查不到 | 最新查得到的是 09:00；09:05 查不到 | `k_candle_follow_service.go:247`（未走完即 return，**不呼叫 Save**） | `TestOnlyAClosedCandleIsStored` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.2 | 指標計算不使用進行中的那一根 | 結果與沒有跟盤時**完全相同** | 靠「不存」達成：計算讀資料庫，資料庫沒有它。指標計算的每一條路徑本切片一行未改 | 既有 `indicator_calculation_service_test.go` 全數維持綠燈；`TestOnlyAClosedCandleIsStored` 釘住「不存」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.3 | 走完之後就查得到了 | 09:05 查得到 | `k_candle_follow_service.go:252`（`store`） | `TestOnlyAClosedCandleIsStored` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.1 | 走完的當下就存入 | 立刻存入，不必等下一輪 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.2 | 事後修正照樣覆蓋，不會變成兩根 | 收盤價變 120；09:05 仍只有一根 | `k_candle_repository.go:45` 的 upsert（**既有，本切片一字未改**） | 既有 `k_candle_repository_test.go` 的覆蓋測試 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.3 | 沒有人在看時由自動抓取補上 | 由下一輪存入，最慢五分鐘內 | `KCandleIngestionJob`（**既有，本切片一字未改**） | 既有 `k_candle_ingestion_service_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.4 | 跟盤送來的資料違規時不存入 | 不存入；留下紀錄說明是哪一根、違反哪一條 | `k_candle_follow_service.go:265`（走既有的 `NewKCandleDomain` 把關） | `TestACandleBreakingARuleIsSkippedAndTheFollowCarriesOn` | 「不存入」asserts-oracle；「留下紀錄」shallow（見下方註） | produces-oracle | 🟠 mis-asserted |
| AC-06.1 | 中斷時明說即時更新已經停止 | 收到一則「已停止」的更新（非靜默、非連線錯誤） | `k_candle_follow_service.go:190`（`publishStalled`） | `TestTheViewerIsToldWhenTheFeedStops` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.2 | 系統自己重新跟上，不需要人介入 | 收到**現在的樣子**；中斷期間的變動不補播 | `k_candle_follow_service.go:180`（迴圈重連）；中斷期間的輸入從未被保留 | `TestComingBackGivesTheShapeNowAndReplaysNothing` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.3 | 重試的間隔逐次拉長，且不放棄 | 每次比前次長，最長 30 秒；不停止重試 | `k_candle_follow_domain.go:114` | `TestNextRetryDelayGrowsUpToTheCeilingAndNeverGivesUp`、`TestTheFirstRetryGapIsBoundByTheCeilingToo` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.4 | 中斷期間走完的那幾根不會遺失 | 09:05 看得到 | `KCandleIngestionJob`（**既有，本切片一字未改**） | 既有攝取測試 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.5 | 即時完全不能用時其他功能照常 | 查詢／新增／修改／刪除／指標計算／自動抓取全部照常；新資料仍五分鐘內出現 | 跟盤獨立於既有每一條路徑；失敗只影響跟盤自己（`run` 迴圈內自行吸收） | `TestASourceThatRefusesIsReportedAndRetried` 釘住「來源全滅時跟盤自己撐住」；既有 13 個套件全綠釘住其餘功能未受影響 | asserts-oracle | produces-oracle | ✅ conforms |

### AC-05.4 的註

「不存入」與「跟盤本身繼續」兩半都由測試釘死；**「留下紀錄」那一半沒有測試**。
紀錄目前是一行 log（`k_candle_follow_service.go:267`，帶交易標的、起始時間與違反的規則），
而斷言 log 輸出既脆弱又會把測試綁在訊息字面上。

與攝取切片的差別值得說明：那裡的跳過紀錄回給呼叫端（`SkippedKCandleDto`），因為那是一輪
有結束、有報告的作業；跟盤沒有呼叫端可以回報，一行 log 是誠實的機制。
**這一條記為已知限制而非缺陷**——若日後跟盤要對外提供狀況查詢，紀錄就會有結構，屆時補測試。

---

## 2. Business Rules / NFR

| ID | 條款 | 覆蓋情形 |
|---|---|---|
| BR-1 | 跟盤的單位是交易標的 | 由 AC-01.2 覆蓋 ✅ |
| BR-2 | 有人看才跟，最後一個離開就停；連線斷掉即視為離開 | AC-01.1／01.4／01.7 ✅ |
| BR-3 | 跟的是觀看者要的那一個，與觀察清單無關 | AC-01.5 ✅ |
| BR-4 | 進行中 K 線只給觀看者看 | AC-04.1／04.2 ✅ |
| BR-5 | 一根走完的當下就存入，並通過既有的每一項 K 線規則 | AC-05.1／05.4 ✅ |
| BR-6 | 重複即覆蓋 | AC-05.2 ✅（既有規則） |
| BR-7 | 每五分鐘一輪的自動抓取不因跟盤而停 | AC-05.3／06.4 ✅（既有 job 未動） |
| BR-8 | 更新間隔上限只節流「目前的樣子」 | AC-03.3 ✅ |
| BR-9 | 即時是加值不是取代 | AC-06.5 ✅ |
| NFR-1 | 至多每十秒送出一次；一根走完不受此限 | AC-03.1／03.3 ✅；預設值由 `application_config_test.go` 釘住 |
| NFR-2 | 同時跟盤的標的數量不設上限 | 程式碼無任何上限判斷（`follows` map 無容量限制）✅ |
| NFR-3 | 無身分驗證；跟盤只讀行情、只寫已收完的 K 線 | 路由清單測試 `TestAutomaticIngestionOpensNoWayIn` 釘住這條路徑不觸及觀察清單 ✅ |

---

## 3. Orphans

| 行為 | 對應條款 | 判定 |
|---|---|---|
| `GET /k-candles/live` 未指定 `symbol` 時回 400 | 無對應條款 | ⚠️ 未載明於 PRD，但屬既有慣例（查詢一律要求交易標的）。**不是超出範圍**——它是同一條規則的延伸 |
| 停止之後的觀看者被回絕（`ErrKCandleFollowStopped`） | 無對應條款 | ⚠️ 未載明。關機是系統行為而非業務情境，PRD 未描述；程式碼的選擇（明白回絕而不是掛在沒人餵的通道上）是唯一合理解 |
| 跟不上的觀看者會被丟掉一則更新 | 無對應條款 | ⚠️ 未載明。這是「一個人不能拖垮所有人」的取捨，只丟得掉正在成形的那一根（走完的在送出前就已存入），因此不違反任何條款 |

三項都**不落在 Out of Scope 清單內**，不構成範圍蔓延。建議日後補進 PRD。

---

## 4. Summary

```
✅ 25 conforms · 🔴 0 violations · 🟠 1 mis-asserted · 🟡 0 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 3 orphans
Conformance: 96.2%（25/26）
```

**稽核過程中修掉的三處**（都是測試沒真的釘住 oracle，程式碼本來就對）：

| # | 問題 | 處置 |
|---|---|---|
| 1 | AC-01.2 只驗了「份數仍為 1」，沒驗「兩人收到同一份」 | 補上兩個觀看者各自收到同一根的斷言 |
| 2 | AC-02.3「一律五分鐘一根」完全沒有測試——訂閱錯長度會安靜地送來別的粗細 | 補上斷言訂閱路徑為 `/btcusdt@kline_5m` |
| 3 | AC-06.2「恢復後給現在的樣子、不補播」沒有測試 | 補上斷言重連後收到的是新值，且沒有補播 |

三處都以突變測試確認會紅（只送給第一個觀看者／訂閱一分鐘 K 線／恢復後仍當成跟不動，
全部被對應測試擋下）。

**Ceiling:** 這是靜態一致性稽核——它逐條比對測試斷言與程式碼路徑對規格的預期結果，
不執行自己發明的情境。AC-04.2、AC-05.2／05.3、AC-06.4／06.5 講的是
「既有行為不得因本切片而改變」，判定依據是「本切片未觸及那些路徑」加上「既有測試全數維持綠燈」。
