# Oracle — 多分身運作下的背景工作分工與訊息只送一次

每一列的「預期」**只從 PRD 推導**，在任何實作存在之前寫下。
寫測試時斷言的期望值一律抄這裡，**不准從跑出來的結果回填**。

固定數字（PRD §4）：值班租期 30s、續期 10s、保守提早 5s；寄送檢查 2s、寄送逾時 2m；
重試 2s 起加倍封頂 5m；送達期限＝輪次訊息 max(觸發間隔, 5m)、動靜 1h；已結訊息保留 7 天。
`T` = 該動作發生的時刻。

---

## A · `JobLeadershipTermDomain`

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| A1 | 租期 30s、提早 5s，取得於 `T` | `ExpiresAt` | `T+30s` | 值班租期 |
| A2 | 同上 | `HeldUntil` | `T+25s` | 保守提早 |
| A3 | 提早量 ≥ 租期（例如 30s / 30s） | 建構 | 提早量被 clamp 到租期的一半（15s），`HeldUntil` = `T+15s` | 防禦：不允許永遠不值班 |

## B · `JobLeadershipService` / `JobLeadershipApplication`

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| B1 | repo `Acquire` 回 true，現在 `T` | `RenewLeadership` 後於 `T+24s` 問 `IsLeader` | `true`；`Acquire` 收到 holder=本分身名、expiresAt=`T+30s` | 保守提早量之內仍在值班 |
| B2 | 同上 | 於 `T+26s` 問 `IsLeader` | `false` | 接近租期尾聲時保守地認定不在值班 |
| B3 | 同上 | 於**正好** `T+25s` 問 | `false`（到了那一刻就放手） | 邊界 |
| B4 | repo `Acquire` 回 false | `RenewLeadership` 後立刻問 | `false` | 值班分身持續續期時別台無法接手 |
| B5 | 曾持有；這次 `Acquire` 回 error | `RenewLeadership` 後立刻問 | `false`，且 Renew 回傳 error | 續期失敗即視同沒續到 |
| B6 | 未持有 → 這次取得 | `RenewLeadership` 回傳的變化 | `Gained=true, Lost=false` | 留下成為值班紀錄 |
| B7 | 持有中 → 這次 `Acquire` false | 回傳的變化 | `Gained=false, Lost=true` | 留下失去值班紀錄 |
| B8 | 持有中 → 這次仍持有 | 回傳的變化 | 兩者皆 false | 續期不是變化 |
| B9 | 持有中 | `ReleaseLeadership` | repo `Release(name, 本分身名)` 被呼叫；之後 `IsLeader` = `false` | 正常關機時立刻交還 |
| B10 | 從未持有 | `ReleaseLeadership` | **不**呼叫 repo `Release` | 不是值班時什麼都不做 |

## C · `JobLeadershipLeaseRepository`（真 Postgres）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| C1 | 沒有租約列 | A `Acquire(now=T, exp=T+30s)` | `true` | 沒人值班時成為值班 |
| C2 | A 持有到 `T+30s` | B `Acquire(now=T+10s)` | `false` | 別台無法接手 |
| C3 | A 持有到 `T+30s` | A `Acquire(now=T+10s, exp=T+40s)` | `true`；之後 B 於 `T+35s` `Acquire` → `false` | 續期 |
| C4 | A 持有到 `T+30s` | B `Acquire(now=T+31s)` | `true` | 到期由別台接手 |
| C5 | A 持有到 `T+30s` | A `Release` 後 B `Acquire(now=T+1s)` | `true` | 正常關機立刻交還 |
| C6 | A 持有 | B `Release` 後 C `Acquire(now=T+1s)` | `false`（B 交還不了別人的） | 防禦 |
| C7 | 沒有租約列 | A 與 B 併發 `Acquire(now=T)` 各 20 次 | 恰好一方得到 `true` | 恰好一台成為值班 |

## D · 值班 job

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| D1 | 不在值班 | 資金費率 / 持倉統計 / 交易規格 / 保證金分級 job 的一輪 | 對應 application 的 round **不被呼叫** | 只有值班分身抓行情 |
| D2 | 在值班 | 同上 | round 被呼叫一次 | 照常 |
| D3 | 啟動時在值班 | K 線 job 第一輪 | 跑回補（`RunBackfill`），不跑定時輪 | 只有一台時行為相同 |
| D4 | 啟動時不在值班 → 之後成為值班 | K 線 job 成為值班後的第一個 tick | 跑**回補**（補交接缺口），下一個 tick 跑定時輪 | 接手時先回補 |
| D5 | 在值班 → 失去 → 再成為值班 | K 線 job | 再次成為值班的第一個 tick 跑回補 | 同上 |
| D6 | 不在值班 | 跟盤 roster job 一輪 | 呼叫 `RefreshRelayedFollows`（轉播快照），不向來源要 | 失去值班時放下來源名額、改為轉播 |
| D7 | 在值班 | 跟盤 roster job 一輪 | 呼叫 `RefreshFixedFollows` | 照常 |
| D8 | 合約 K 線 job | D3–D5 同樣成立 | 同上 | 值班工作清單 |

## E · `KCandleFollowService.RefreshRelayedFollows`（code review 後由放下改為轉播）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| E1 | 正在向來源跟盤 2330 | `RefreshRelayedFollows` | 來源那條線結束；改由快照轉播，`FollowedSymbolCount()` 仍為 1 | 失去值班放下名額 |
| E2 | 值班分身寫下 2330 的快照 | 轉播的觀看者 | 收到同一根 K 線；收盤的那根不會被下一根蓋掉；舊快照不當成即時 | 每台分身都看得到即時更新 |

## F · 輪次認領（`StrategyBotRepository`，真 Postgres）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| F1 | X、Y 執行中且到期、未被認領 | A `ClaimDue(limit 10)` | 回 X、Y；兩列 `round_claimed_by = A` | 認領 |
| F2 | X 被 A 認領到 `T+2m` | B 於 `T+1m` `ClaimDue` | 不回 X | 已被認領的跳過 |
| F3 | X 被 A 認領到 `T+2m` | B 於 `T+3m` `ClaimDue` | 回 X，`round_claimed_by = B` | 認領到期由別台接手 |
| F4 | X、Y 到期 | A、B 併發 `ClaimDue` | 兩方回傳的集合**不重疊**、聯集 = {X, Y} | 各自認領不同的 |
| F5 | X 已停止（但到期時刻已過） | `ClaimDue` | 不回 X | 只有執行中 |
| F6 | X 被 A 認領中 | B `ClaimOne(X)` | `false` | 手動跑一輪拒絕 |
| F7 | X 沒被認領（甚至已停止） | B `ClaimOne(X)` | `true` | 手動跑一輪照舊能跑停止中的 |
| F8 | X 被 A 認領 | B `ReleaseRoundClaim(X, B)` | X 仍被 A 認領 | 只有認領者能釋放 |
| F9 | X 被 A 認領 | A `ReleaseRoundClaim(X, A)` | X 的認領欄位清空，下一次 `ClaimDue` 會回它（若到期） | 跑完解除 |
| F10 | X 上次訊號「買入」 | `ForgetSentSignal(X, 賣出)` | 仍是「買入」 | 只在仍是那一則的信號時清 |
| F11 | X 上次訊號「買入」 | `ForgetSentSignal(X, 買入)` | 變成空 | 作廢後忘記 |

## G · `StrategyBotRunApplication`（mock repository，真 service / domain）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| G1 | `ClaimDue` 回 X | `RunDueRounds` | 對 X 跑一輪；`ClaimDue` 收到 claimant = 本分身名、claimedUntil = now + 輪次時限上限 + 15s | 認領 |
| G2 | `ClaimOne` 回 false | `RunRoundNow(X)` | 回 `ErrStrategyBotAlreadyRunningARound`；不跑 | 正在跑時拒絕 |
| G3 | `ClaimOne` 回 true | `RunRoundNow(X)` | 跑一輪並回傳 X 的最新狀態 | 沒人在跑時手動跑 |
| G4 | X 上次「賣出」、這輪結論「買入」 | 跑一輪 | **不呼叫**投遞 proxy；transaction 內：UpdateRunState(last=買入)、Append、Enqueue 一則 kind=round、signal=買入、roundDueAt=X 的 NextRunAt、text 含 `🔖 Run 52`（Append 回 52）、expiresAt = now + max(觸發間隔, 5m)；ReleaseRoundClaim | 一起成立、印 Run N |
| G5 | X 上次「買入」、這輪「買入」 | 跑一輪 | Append 被呼叫；**Enqueue 不被呼叫** | 結論沒變 |
| G6 | 這輪「衝突」或「沒有結論」 | 跑一輪 | Enqueue 不被呼叫；last 不變 | 衝突或沒有結論 |
| G7 | Append 回 error | 跑一輪 | `Atomically` 回 error（transaction 會 rollback）；Enqueue 不被呼叫 | 記下途中出錯 |
| G8 | 鎖讀到的 NextRunAt ≠ 這輪的 dueAt | 跑一輪 | 不 UpdateRunState、不 Append、不 Enqueue；**仍 ReleaseRoundClaim** | 同一輪被記第二次 |
| G9 | 這輪因交易策略找不到而停擺 | 跑一輪 | UpdateRunState(stopped, tradingStrategyUnavailable)；Enqueue 一則 kind=lifecycle、text = 既有停擺訊息、roundDueAt=nil | 停擺通知一起成立 |
| G10 | 觸發間隔 1 分鐘 | G4 的 enqueue | expiresAt = now + 5m | 送達期限至少 5 分鐘 |
| G11 | 觸發間隔 15 分鐘 | G4 的 enqueue | expiresAt = now + 15m | 送達期限＝觸發間隔 |

## H · `StrategyBotApplication` 生命週期

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| H1 | X 已停止，有投遞設定 | 啟動 X | Enqueue 一則 kind=lifecycle、text = 既有「已啟動」訊息、expiresAt = now + 1h；**不呼叫**投遞 proxy | 啟動通知成為待送訊息 |
| H2 | X 執行中 | 停止 X | Enqueue 「已停止」訊息 | 同上 |
| H3 | Enqueue 回 error | 停止 X | 停止仍成功回傳 | 通知失敗不影響已發生的動作 |

## I · `PendingMessageDomain`

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| I1 | expiresAt = `T+5m` | `IsExpiredAt(T+5m)` | `true`（到了那一刻即作廢） | 送達期限 |
| I2 | 同上 | `IsExpiredAt(T+4m59s)` | `false` | 邊界 |
| I3 | attempt 0，結果成功 | `AfterAttempt` | sent | 寄出 |
| I4 | attempt 0（第一次失敗），連不上 | `AfterAttempt(now=T)` | retry，nextAttemptAt = `T+2s`，attempt = 1 | 2s 起 |
| I5 | attempt 2（第三次失敗），等太久 | `AfterAttempt(T)` | retry，`T+8s` | 加倍 |
| I6 | attempt 20 | 連不上 | retry，`T+5m` | 封頂 5 分鐘 |
| I7 | attempt 0，連不上且 RetryAfter 7s | `AfterAttempt(T)` | retry，`T+7s` | 照 Telegram 說的等 |
| I8 | attempt 5（退避 64s），RetryAfter 7s | `AfterAttempt(T)` | retry，`T+64s`（取較長） | 照較長者 |
| I9 | 金鑰不被接受 | `AfterAttempt` | refused，halt reason = `credentialRejected` | 永久失敗 |
| I10 | 找不到聊天室 | `AfterAttempt` | refused，`destinationNotFound` | 永久失敗 |
| I11 | 寄送前得到「沒有投遞設定」 | `AfterMissingDeliverySetting` | refused，`deliveryNotConfigured` | 永久失敗 |
| I12 | kind = round | `ForgetsSignalWhenAbandoned` | `true` | 只有輪次訊息 |
| I13 | kind = lifecycle | 同上 | `false` | 動靜作廢不影響信號 |
| I14 | status = ready，nextAttemptAt = `T` | `IsDispatchableAt(T)` | `true` | 可寄 |
| I15 | status = ready，nextAttemptAt = `T+1s` | `IsDispatchableAt(T)` | `false` | 還在等 |
| I16 | status = sending，claimedUntil = `T+1s` | `IsDispatchableAt(T)` | `false` | 寄送逾時前別台不碰 |
| I17 | status = sending，claimedUntil = `T` | `IsDispatchableAt(T)` | `true` | 寄送中逾時後重寄 |
| I18 | 非法 status 字串 | 建構 | 視為 ready | 安全預設 |

## J · `PendingMessageQueueDomain`

| # | Given（依 id） | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| J1 | 收件人 1：#1(可寄)、#2(可寄)；收件人 2：#3(可寄) | `HeadsDispatchableAt(T)` | `[#1, #3]` | 依序、不同人互不等 |
| J2 | 收件人 1：#1(等待中，nextAttempt 在未來)、#2(可寄) | 同上 | `[]` | 前一則沒結清，後一則等 |
| J3 | 空 | 同上 | `[]` | 防禦 |

## K · `PendingMessageService.DispatchPendingMessages`（mock repository / proxy）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| K1 | 一則可寄的 Run 52 訊息，Claim true，proxy 成功 | 寄送 | proxy 收到該 text；`MarkSent(id, 本分身)`；回傳 1 | 寄出 |
| K2 | Claim 回 false | 寄送 | proxy 不被呼叫 | 一則同時只被一台拿去寄 |
| K3 | proxy 回連不上（attempt 0） | 寄送 | `Reschedule(id, 本分身, attempt=1, nextAttemptAt=T+2s)`；不 Abandon | 稍後再寄 |
| K4 | proxy 回連不上 + RetryAfter 7s | 寄送 | `Reschedule(..., T+7s)` | 照 Telegram 說的等 |
| K5 | proxy 回金鑰不被接受；鎖讀 bot 為執行中 | 寄送 | `Abandon(id, credentialRejected)`；bot UpdateRunState(stopped, credentialRejected)；**不** Enqueue 任何通知 | 作廢並停下 |
| K6 | 同 K5，但 bot 已停止 | 寄送 | Abandon；**不** UpdateRunState | 已停止時只作廢 |
| K7 | 投遞設定已移除（deliver 回 not configured） | 寄送 | Abandon(deliveryNotConfigured)；bot 停下 | 投遞設定已被移除 |
| K8 | 輪次訊息已過期（signal=買入） | 寄送 | proxy 不被呼叫；先 `ForgetSentSignal(bot, 買入)` 再 `Abandon(expired)` | 超過送達期限 |
| K9 | 動靜訊息已過期 | 寄送 | 不呼叫 `ForgetSentSignal`；Abandon(expired) | 動靜作廢不影響信號 |
| K10 | 任何一次寄送檢查 | 寄送 | `DeleteSettledBefore(T−7d)` 被呼叫 | 保留 7 天 |
| K11 | 讀取訊息回 error | 寄送 | 回 error；proxy 不被呼叫 | 寄送檢查失敗下一次再試 |
| K12 | deliver 回其他 error（例如開鎖失敗） | 寄送 | `Reschedule`（視同會自己好），不 Abandon | 防禦：只有確知的永久失敗才停 bot |

## L · `PendingMessageRepository`（真 Postgres）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| L1 | bot X 第 dueAt 輪已 Enqueue 一則 | 同 (X, dueAt) 再 Enqueue | 不出錯；X 仍只有一則 | 同一輪不會有第二則 |
| L2 | 兩則生命週期訊息（roundDueAt nil） | 都 Enqueue | 兩則都在 | NULL 不互撞 |
| L3 | 一則 ready、nextAttemptAt ≤ now | A `Claim` 再 B `Claim` | A true、B false | 一則同時只被一台拿去寄 |
| L4 | A claim 到 `T+2m` | B 於 `T+3m` `Claim` | true | 寄送中逾時重寄 |
| L5 | A claim | B `MarkSent(id, B)` | 不生效，仍為寄送中 | 只有拿著的人能結清 |
| L6 | A claim | A `MarkSent` | 狀態 sent，`FindDispatchCandidates` 不再回它 | 寄出只寄一次 |
| L7 | 一則 sent 於 `T−8d`、一則 abandoned 於 `T−6d` | `DeleteSettledBefore(T−7d)` | 只刪前者 | 保留 7 天 |
| L8 | bot X 有一則待送訊息 | 刪除 X | 該則消失 | 機器人被刪掉後訊息作廢 |
| L9 | 在 `Atomically` 內 Enqueue 後 work 回 error | — | 沒有任何訊息留下 | 記下途中出錯什麼都沒改變 |

## M · `TelegramMessageDeliveryProxy`

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| M1 | Telegram 回 429 `{"ok":false,"parameters":{"retry_after":7}}` | `Deliver` | FailureReason = unreachable，RetryAfter = 7s | Telegram 要求放慢 |
| M2 | 回 200 ok | `Deliver` | FailureReason none，RetryAfter 0 | 照舊 |
| M3 | 回 429 但沒有 retry_after | `Deliver` | unreachable，RetryAfter 0 | 防禦 |

## N · 測試訊息

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| N1 | proxy 回 429 + retry_after | `SendTestMessage` | 結果 `Delivered=false, FailureReason=unreachable`；不 Enqueue | 測試訊息當場送 |

## O · 訊息文字

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| O1 | round.RunNumber = 52 | `Text()` | 最後一行為 `🔖 Run 52` | 印輪次編號 |
| O2 | round.RunNumber = 0 | `Text()` | 沒有 `Run` 那一行 | 尚未編號時不印 |
