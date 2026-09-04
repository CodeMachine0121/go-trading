# 行情對話助手 — Oracle（預期結果，先於實作寫下）

每一列的「預期結果」都只從 `PRD.md` 的 AC 場景推導，**不參考任何實作**。
測試的每一個斷言值一律取自本表；若某個值只能靠跑實作才得知，就是本表不完整，回頭補或問。

## AssistantAskDomain — 提問的把關（US-01）

| # | 輸入 | 預期結果 |
|---|---|---|
| A1 | `"BTCUSDT 最近走勢如何"` | 通過，正規化後與輸入相同 |
| A2 | `"  BTCUSDT 最近走勢  "` | 通過，正規化後 `"BTCUSDT 最近走勢"` |
| A3 | `"　全形空白包住　"`（全形空白） | 通過，正規化後 `"全形空白包住"` |
| A4 | `""` | 拒絕，`ErrAssistantAskEmpty`，訊息含「必須寫點什麼」 |
| A5 | `"   "` | 拒絕，`ErrAssistantAskEmpty` |
| A6 | `"\t\n "` | 拒絕，`ErrAssistantAskEmpty` |
| A7 | `"　"`（僅全形空白） | 拒絕，`ErrAssistantAskEmpty` |

## AssistantQueryRoundsDomain — 查詢次數上限（US-06，上限 8）

| # | 已用次數 / 上限 | 預期結果 |
|---|---|---|
| R1 | 0 / 8 | `Allows()` = true；`ReachedLimit()` = false |
| R2 | 3 / 8 | `Allows()` = true；`ReachedLimit()` = false |
| R3 | 7 / 8 | `Allows()` = true（第 8 次照做）；`ReachedLimit()` = false |
| R4 | 8 / 8 | `Allows()` = false；`ReachedLimit()` = true |
| R5 | 0 / 1 | `Allows()` = true；記一次後 `Allows()` = false、`ReachedLimit()` = true |
| R6 | `Record()` 累加 | 記三次後 `Used()` = 3 |

## AssistantCandleLimitDomain — 單次根數上限（US-07，上限 200）

| # | 助手要求根數 | 預期結果 |
|---|---|---|
| C1 | 50 | 採用 50，`Truncated()` = false |
| C2 | 200 | 採用 200，`Truncated()` = false |
| C3 | 199 | 採用 199，`Truncated()` = false |
| C4 | 201 | 採用 200，`Truncated()` = true |
| C5 | 500 | 採用 200，`Truncated()` = true |
| C6 | 未指定（0） | 採用 200，`Truncated()` = false（未指定不算截斷——他沒要更多） |
| C7 | -1 | 拒絕，`ErrAssistantQueryArgument`，訊息含「必須大於零」 |
| C8 | 上限 1、要求 5 | 採用 1，`Truncated()` = true |

> C6 的判定：PRD「沒說要幾根一律視為 200 根」與「超過上限只回最新的 200 根並告知已截斷」
> 是兩條不同規則。未指定不是「要求超過」，故不告知截斷。

## DailyUsageAllowanceDomain — 每日額度（US-08，額度 300000）

| # | 今日已用 / 額度、現在時刻（UTC） | 預期結果 |
|---|---|---|
| D1 | 0 / 300000 | `Exhausted()` = false |
| D2 | 150000 / 300000 | `Exhausted()` = false |
| D3 | 299999 / 300000 | `Exhausted()` = false |
| D4 | 300000 / 300000 | `Exhausted()` = true（剛好等於即用盡） |
| D5 | 400000 / 300000 | `Exhausted()` = true（超出也算用盡） |
| D6 | 現在 `2026-09-04T13:45:10Z` | `ResetsAt()` = `2026-09-05T00:00:00Z` |
| D7 | 現在 `2026-09-04T00:00:00Z` | `ResetsAt()` = `2026-09-05T00:00:00Z`（午夜當下屬於這一天） |
| D8 | 現在 `2026-09-04T23:59:59Z` | `ResetsAt()` = `2026-09-05T00:00:00Z` |
| D9 | 現在為非 UTC 時區的同一瞬間 | `ResetsAt()` 仍為該瞬間 UTC 曆日的隔日午夜 |
| D10 | 現在 `2026-09-04T13:45:10Z` | `StartOfDay()` = `2026-09-04T00:00:00Z` |

## ConversationDomain — 對話的行為（US-01、US-02、US-05、US-10）

近期訊息上限 20。一次問答攤平成兩則訊息：提問、回答。

| # | 給定 | 預期結果 |
|---|---|---|
| V1 | 3 次問答（= 6 則） | `RecentMessages(20)` 回 6 則，順序：ask,answer,ask,answer,ask,answer |
| V2 | 10 次問答（= 20 則） | `RecentMessages(20)` 回 20 則，全部 |
| V3 | 13 次問答（= 26 則） | `RecentMessages(20)` 回**最後** 20 則；第一則是第 4 次問答的提問 |
| V4 | 13 次問答（= 26 則） | `ToDto()` 的訊息數 = 26（舊的仍讀得到） |
| V5 | 0 次問答 | `RecentMessages(20)` 回空；`ToDto()` 訊息數 = 0 |
| V6 | 上限 1、3 次問答 | `RecentMessages(1)` 回 1 則，是最後那次的回答 |
| V7 | 任何對話 | `RecentMessages` 只含提問與回答兩種角色，**不含任何查詢紀錄** |
| V8 | 追加一次問答，現在 `2026-09-04T10:00:00Z` | 新 entity 的 `LastActiveAt` = `2026-09-04T10:00:00Z`；問答數 +1 |
| V9 | 一次問答含 2 筆查詢紀錄 | 追加後該 turn 的 `QueryCount` = 2、查詢紀錄 2 筆 |
| V10 | `ToDto()` 的時間欄位 | 一律以 UTC 交出 |
| V11 | `ToSummaryDto()` | 含識別碼、`LastActiveAt`、訊息則數 |

## AssistantConversationService.Ask — 編排（全部 US）

mock：`IAssistantProxy`、`IConversationRepository`、`IClockProxy`、`[]IAssistantQuery`

| # | 給定 | 預期結果 |
|---|---|---|
| S1 | 未指名對話、助手直接回答 | 新建對話並落地一次問答；回答與新對話識別碼交回；`Save` 被呼叫一次 |
| S2 | 指名既有對話、助手直接回答 | `AppendTurn` 被呼叫一次，帶該對話識別碼 |
| S3 | 提問為空白 | `ErrAssistantAskEmpty`；**proxy 與 repository 的寫入一次都沒被呼叫** |
| S4 | 指名不存在的對話 | `ErrConversationNotFound`；不建立新對話、不落地 |
| S5 | 今日用量 = 額度 | `ErrDailyUsageAllowanceExhausted`，訊息含重置時刻；**proxy 未被呼叫** |
| S6 | 今日用量 = 額度 − 1，這次用量 500 | 正常回答並落地（事後結算） |
| S7 | proxy 回一次查詢請求，第二輪回答 | 對應的 `IAssistantQuery.Run` 被呼叫一次；落地的 `QueryCount` = 1 |
| S8 | proxy 一輪回兩個查詢請求 | 兩個都被執行；`QueryCount` = 2 |
| S9 | 查詢名稱不在清單裡 | 該次結果為「沒有這個能力」交回助手；回答照常產出；不視為失敗 |
| S10 | `IAssistantQuery.Run` 回錯誤 | 錯誤原文交回助手當資料；回答照常產出；該筆查詢紀錄標為被拒 |
| S11 | proxy 連續回查詢請求超過 8 輪 | 第 9 輪不再放行；最後一次請求標明已達上限；落地的 `StoppedAtQueryLimit` = true |
| S12 | proxy 回錯誤（不可用） | `ErrAssistantUnavailable`；**`Save`／`AppendTurn` 一次都沒被呼叫** |
| S13 | proxy 逾時（context deadline） | `ErrAssistantUnavailable`；不落地 |
| S14 | proxy 回空白回答 | `ErrAssistantUnavailable`；不落地 |
| S15 | 每一輪 | 交給 proxy 的能力宣告 = 注入清單的每一項（名稱、說明、參數格式） |
| S16 | 三輪往返，各回用量 100 | 落地的用量 = 300（累加，不是最後一次） |
| S17 | 落地時 | `Save`／`AppendTurn` 只被呼叫一次（一次問答一次寫入） |

## AssistantConversationService 讀取（US-08、US-10）

| # | 給定 | 預期結果 |
|---|---|---|
| S18 | 三段對話，`LastActiveAt` 不同 | `ListConversations` 依 `LastActiveAt` 由新到舊 |
| S19 | 一段都沒有 | `ListConversations` 回空清單、無錯誤 |
| S20 | 一段有 6 則訊息 | `GetConversation` 回 6 則，由早到晚 |
| S21 | 不存在的識別碼 | `GetConversation` 回 `ErrConversationNotFound` |
| S22 | 今日額度已用盡 | `ListConversations`／`GetConversation` 照常回內容；**未查用量** |
| S23 | proxy 不可用 | 兩個讀取方法照常（它們不碰 proxy） |

## 助手能力（US-03、US-04、US-07）

| # | 能力 / 給定 | 預期結果 |
|---|---|---|
| Q1 | `TradingSymbolListAssistantQuery`，兩個標的 | 回文字含 BTCUSDT、ETHUSDT |
| Q2 | `KCandleSeriesAssistantQuery`，要求一小時刻度 | 以該刻度呼叫既有用例；回文字含彙總後的根數 |
| Q3 | `KCandleSeriesAssistantQuery`，要求 500 根 | 只交出最新 200 根，回文字明確含「已截斷」字樣 |
| Q4 | `KCandleSeriesAssistantQuery`，未指定根數 | 視為 200 根；回文字**不含**截斷字樣 |
| Q5 | `KCandleSeriesAssistantQuery`，要求七分鐘刻度 | 回錯誤，其訊息即既有規則的拒絕原因（可交回助手） |
| Q6 | `KCandleSeriesAssistantQuery`，區間內無資料 | 回文字表示沒有資料，**不是錯誤** |
| Q7 | `KCandleRangeAssistantQuery`，要求 500 根 | 同 Q3 |
| Q8 | `IndicatorCalculationAssistantQuery` | 以既有用例算一次；回文字含指標名稱與值 |
| Q9 | `StrategyListAssistantQuery` | 回文字含每支策略的識別碼與名稱 |
| Q10 | `StrategyGetAssistantQuery`，不存在 | 回錯誤，訊息為既有的找不到說明 |
| Q11 | `StrategyCreateAssistantQuery`，名稱重複 | 回錯誤，訊息為既有的「名稱已被使用」 |
| Q12 | `StrategyUpdateAssistantQuery` | 以既有用例改一支；回文字含改完的內容 |
| Q13 | 每一項能力 | `Name()` 不重複、`Description()` 非空、`ArgumentSchema()` 是合法 JSON |
| Q14 | 能力清單整體 | **不含任何刪除策略或寫入 K 線的能力** |
| Q15 | 每一項能力的參數為非法 JSON | 回錯誤（可交回助手），不 panic |

## Controller（錯誤對映）

| # | 給定 | 預期結果 |
|---|---|---|
| H1 | `POST /chat` 正常 | 200，body 含 `conversationId` 與 `answer` |
| H2 | body 非合法 JSON | 400 |
| H3 | `ErrAssistantAskEmpty` | 400 |
| H4 | `ErrConversationNotFound` | 404 |
| H5 | `ErrDailyUsageAllowanceExhausted` | 429 |
| H6 | `ErrAssistantUnavailable` | 503 |
| H7 | 其他不明錯誤 | 502（沿用既有慣例） |
| H8 | `GET /chat/conversations` | 200，陣列 |
| H9 | `GET /chat/conversations/:id`，id 非正整數 | 400 |
| H10 | `GET /chat/conversations/:id` 正常 | 200，含訊息陣列 |

## Repository（需 `TEST_POSTGRES_DSN`，未設則跳過）

| # | 給定 | 預期結果 |
|---|---|---|
| P1 | `Save` 一段對話 | 回填識別碼與 `LastActiveAt` |
| P2 | `AppendTurn` | 對話多一次問答；查詢紀錄一起落地 |
| P3 | `AppendTurn` 指名不存在的對話 | `ErrConversationNotFound` |
| P4 | `FindOne` | 回該對話含每一次問答與查詢紀錄，依時間由早到晚 |
| P5 | `FindOne` 不存在 | `ErrConversationNotFound` |
| P6 | `FindAll` | 依 `LastActiveAt` 由新到舊 |
| P7 | `FindAll` 空 | 回空清單、無錯誤 |
| P8 | `SumUsageBetween` 跨兩段對話 | 回區間內每一次問答用量的總和 |
| P9 | `SumUsageBetween` 區間內無問答 | 回 0、無錯誤 |
| P10 | `SumUsageBetween` 邊界 | 起點含、終點不含 |
