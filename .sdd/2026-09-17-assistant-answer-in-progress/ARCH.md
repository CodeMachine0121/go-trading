# 一次回答在寫的時候就看得見 — Architecture Design

---

## 1. Design Goal & Guiding Principle

**讓那筆「進行中」的紀錄成為唯一的真相。**

今天「一次回答正在進行」這件事只存在於一條 HTTP 連線上。這個切片把它搬進資料庫：
一次回答一被提出就有一列，狀態是進行中；寫完轉成已回答，壞掉轉成失敗。

**不在記憶體裡另外存一份名單。** 一份會與資料庫講出兩種話，而且重啟就沒了、
資料列卻還在。既有的做法（背景 job 的輪次記錄）已經是這一套：
`running` → `succeeded`/`failed`，啟動時掃殘留。這裡照抄，不發明新的。

第二條原則：**通過檢查之前什麼都不留。** 空白的提問、額度用完、
接到別人的對話上——這三種在任何一列被寫下之前就擋掉，
所以「對話裡的每一則都真的發生過」仍然成立。

---

## 2. Change Scope

| 層 | 動作 |
| :--- | :--- |
| Entity | `AssistantTurn` 多兩欄（狀態、失敗原因） |
| Domain model | 新增 `AssistantTurnStatusVo`；`ConversationDomain` 多兩個行為 |
| Domain service | `AssistantConversationService.Ask` 拆成「開一次回答」與「把它寫完」 |
| Interface | `IConversationRepository` 多三個 method |
| Infrastructure | `ConversationRepository` 實作那三個 |
| Application | `AssistantConversationApplication` 多一個「收殘留」 |
| Controller | `Ask` 改回 202 與一次回答的識別碼與狀態 |
| 組裝根 | 啟動時收殘留 |

---

## 3. New Classes / Modules

| 型別 | 檔案 | 職責 |
| :--- | :--- | :--- |
| `AssistantTurnStatusVo` | `internal/domain/models/vo/assistant_turn_status_vo.go` | 三種狀態的值物件，建構子把不認得的一律正規化成失敗 |
| `AssistantAnswerStartedDto` | `internal/domain/models/dto/assistant_answer_started_dto.go` | 提問當下交還的形狀：對話識別碼、回答識別碼、狀態 |
| `assistantAnswerWriter`（未匯出） | `internal/domain/service/assistant_answer_writer.go` | **一件正在跑的工作**：拿著它要寫回哪一列、以及寫完之後怎麼收尾 |

### `assistantAnswerWriter` 為什麼長在 service 旁邊而不是 `models/`

照 `.claude/rules/architecture.md`：它拿的是執行機制（一個跑在連線之外的
goroutine 要寫回哪一列），不是領域資料。拿掉「要寫回哪一列」之後它什麼都不剩，
所以它不是 Domain Model、不加任何後綴、住在它所屬的那個 Domain Service 旁邊。

**業務規則不在它身上。** 它只負責驅動與寫回；「答案長什麼樣」「用量怎麼算」
仍然是 `AssistantExchangeDomain` 的事。

---

## 4. Modified Components

### `AssistantTurn`（entity）

```
Status        string  not null            進行中／已回答／失敗
FailureReason string  not null, default '' 只有失敗時有內容
```

`Answer` 由 `not null` 維持不變，進行中時是空字串——空字串與「沒有答案」
在這裡是同一件事，而狀態欄已經說清楚了。

**既有列的預設值是已回答。** 它們都有答案，沒有別的狀態說得通。

### `IConversationRepository`

| 新 method | 用途 |
| :--- | :--- |
| `StartTurn` | 開一次回答：沒有對話就一併開一段，寫下狀態為進行中的那一列，回傳兩個識別碼 |
| `CompleteTurn` | 把那一列寫完：答案、用量、查了幾次、狀態 |
| `FailAllRunningTurns` | 啟動時把殘留的進行中全部轉成失敗 |
| `HasRunningTurn` | 那段對話上還有沒有進行中的 |

`StartTurn` 把「開對話」與「寫第一列」放在**同一個交易**裡。分成兩次寫的話，
中間掛掉會留下一段沒有任何訊息的空對話。

### `AssistantConversationService`

`Ask` 由「一次做完」拆成兩段：

```
Ask(askDto) → AssistantAnswerStartedDto
  1. 提問不可為空                     （既有，位置不變）
  2. 今日額度                         （既有，位置不變）
  3. 對話歸屬 + 近期訊息              （既有，位置不變）
  4. 那段對話上還有進行中的？→ 拒絕   【新】
  5. StartTurn → 拿到兩個識別碼       【新】
  6. 生一個 assistantAnswerWriter，go 出去
  7. 回傳識別碼與狀態

（連線之外）writer.write()
  ├─ 驅動 AssistantExchangeDomain（既有邏輯，一個字不改）
  ├─ 成功 → CompleteTurn(已回答, 答案, 用量, 查了幾次)
  └─ 失敗 → CompleteTurn(失敗, 原因, 用量＝0)
```

**前三道檢查的順序一個字都不動。** 它們現在的順序是「refusal 由便宜到貴」，
那個理由沒有變。第四道放在它們後面，因為它要先知道是哪一段對話。

**goroutine 用 `context.Background()` 起，不是請求的 context。** 這正是
「連線斷了照樣跑完」那條需求的落點。逾時仍由助手代理自己的 `ResponseTimeout` 管。

### `ConversationDomain`

多兩個行為：`ToDto()` 帶出每一則的狀態與失敗原因；
判斷一段對話上有沒有進行中的那一則。**規則在 domain model 上，不在 repository。**

### Controller

`Ask` 回 `202 Accepted` 與 `AssistantAnswerStartedDto`。

多一個哨兵錯誤 `ErrAssistantAnswerInProgress` → `409 Conflict`：
使用者要做的事（等前一次跑完）與其他四種都不同，塞進既有任何一個都會誤導。

### 組裝根

`main.go` 在起 server 之前呼叫一次「收殘留」，與既有做法一致。

---

## 5. Component Relationships

```
Controller ──▶ AssistantConversationApplication ──▶ AssistantConversationService
                                                         │
                                    ┌────────────────────┼────────────────────┐
                                    ▼                    ▼                    ▼
                          IConversationRepository  IAssistantProxy   assistantAnswerWriter
                                    │                                   （同層、旁邊）
                                    ▼                                        │
                          ConversationRepository ◀───────────────────────────┘
                                （寫回那一列）
```

---

## 6. Extensibility & Handoff Notes

- **前端靠定時回來問。** 讀一段對話已經回得到狀態，不必再開第二條路。
- **要做取消的話**：`assistantAnswerWriter` 就是它該拿取消函式的地方，
  而狀態要多第四種。這一版刻意不做——見 BRIEF。
- **要做推播的話**：`CompleteTurn` 成功之後是唯一該送的地方。

---

## 7. Traceability

| AC | 落在哪 |
| :--- | :--- |
| US-01 一被提出就留下 | `StartTurn` + `assistantAnswerWriter` |
| US-02 被擋下來什麼都不留 | 前三道檢查的位置（在 `StartTurn` 之前） |
| US-03 連線斷了照樣跑完 | goroutine 以 `context.Background()` 起 |
| US-04 重整看得到同一件事 | `AssistantTurn.Status` + `ConversationDomain.ToDto()` |
| US-05 一次只跑一個 | `HasRunningTurn` + `ErrAssistantAnswerInProgress` |
| US-06 重啟收殘留 | `FailAllRunningTurns`，由組裝根呼叫 |

---

## 8. Risks & Open Decisions

| 風險 | 緩解 |
| :--- | :--- |
| 既有列沒有狀態 | 欄位預設值為已回答；它們都有答案 |
| 改掉了「失敗不留存」這條既有規則 | UL-MAP 同步更新，理由寫在 PRD |
| goroutine 逸出（server 關掉時還在跑） | 不攔它——重啟時的收殘留就是為這件事存在的 |
| **那條 goroutine 裡的 panic 會帶走整個行程** | 驅動迴圈自己 `recover()`，把那一次收成失敗。在它離開請求之前，HTTP 層的防護接得住它；離開之後上面什麼都沒有了 |
| **「一段對話一次只跑一則」是先讀後寫** | 加一個只涵蓋進行中那幾列的唯一索引，把破壞它的寫入翻成同一句拒絕。ARCH 原本規劃的 `StartTurn` 具備這個原子性，實作換成「先讀一次、再 `AppendTurn`」時掉了 |
| 一次回答的用量在它跑完之前看不到 | 額度本來就是事後結算，UL-MAP 已載明 |

### Open decisions（交給實作）

- 失敗原因要不要分類（不可用／逾時／助手自己出錯）。建議**只存一句人看得懂的話**：
  使用者要做的事三種都一樣（稍後再試），分類是給日誌看的，不是給他看的。
