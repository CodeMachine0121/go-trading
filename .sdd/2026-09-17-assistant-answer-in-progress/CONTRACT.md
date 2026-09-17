# Contract Traceability Matrix — assistant-answer-in-progress

Contract: `.sdd/2026-09-17-assistant-answer-in-progress/PRD.md`
Design map: `.sdd/2026-09-17-assistant-answer-in-progress/ARCH.md`
Implementation: `internal/domain/models/vo/assistant_turn_status_vo.go`,
`internal/domain/models/entities/assistant_turn.go`,
`internal/domain/models/domains/assistant_exchange_domain.go`,
`internal/domain/models/domains/conversation_domain.go`,
`internal/domain/service/assistant_conversation_service.go`,
`internal/domain/service/assistant_answer_writer.go`,
`internal/infrastructure/persistence/conversation_repository.go`,
`internal/controller/assistant_conversation_controller.go`, `cmd/server/main.go`
Oracle: Acceptance Criteria (18 clauses) + Core Business Rules (7 clauses)

Test files below are abbreviated:
`svc` = `internal/domain/service/tests/assistant_conversation_service_test.go`,
`ctrl` = `internal/controller/tests/assistant_conversation_controller_test.go`,
`exch` = `internal/domain/models/domains/tests/assistant_exchange_domain_test.go`,
`conv` = `internal/domain/models/domains/tests/conversation_domain_test.go`,
`repo` = `internal/infrastructure/persistence/tests/conversation_repository_test.go`.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 送出後馬上拿到回應 | 馬上拿到回答識別碼，狀態是進行中 | `assistant_conversation_service.go:94` `Ask` | `svc:179`, `ctrl:122` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 講完話轉成已回答 | 答案、用量、查了幾次都寫上去了 | `assistant_answer_writer.go:44` + `ToAnsweredTurn` | `svc:179`, `exch:191` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 助手不可用就轉成失敗 | 轉成失敗、說得出原因、用量不計入今日 | `assistant_answer_writer.go:50` + `ToFailedTurn` | `svc:638`, `exch:174` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 空白的提問什麼都不留 | 直接拒絕，連進行中都沒有 | `Ask` 第一道檢查在 `start` 之前 | `svc:291`, `ctrl:164` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 今日額度用完什麼都不留 | 直接拒絕，什麼都沒留下 | `Ask` 第二道檢查 | `svc:314`, `ctrl:164` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 接到別人的對話上 | 「找不到」，什麼都沒留下 | `recentMessagesOf` `RequireOwnership` | `svc:876`, `ctrl:164` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 關掉分頁之後照樣跑完 | 它照樣跑完，那一次轉成已回答 | `assistant_answer_writer.go:42` `context.Background()` | — | no-test | produces-oracle | 🟠 mis-asserted |
| AC-8 | 回來就看得到 | 看得到完整的答案 | `CompleteTurn` + `FindOne` | `repo:298` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 進行中的那一次讀得到 | 問句在、答案還沒有、狀態是進行中 | `conversation_domain.go` `messages()` | `conv:141` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 失敗的那一次也讀得到 | 狀態是失敗，而且說得出原因 | 同上 + `ConversationMessageDto.FailureReason` | `conv:141` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 混在一起時依時間排好 | 都在，依時間由早到晚 | `messages()` 依 Turns 順序展開 | `conv:141`（前段仍是已回答的那一次） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 前一次還在跑 | 被拒絕，並說明前一次還在跑 | `HasAnswerInFlight` + `ErrAssistantAnswerInProgress` | `svc:681`, `conv:236`, `ctrl:164` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 前一次剛回答完 | 送得出去 | 同上 | `svc:698`, `conv:236` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 前一次失敗了不擋路 | 送得出去 | 同上 | `svc:698`, `conv:236` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 重啟時殘留的轉成失敗 | 原因寫著被重新啟動中斷 | `FailAllRunningTurns` + `FailInterruptedAnswers` | `svc:767`, `repo:375` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 重啟之後讀得到那次失敗 | 看得到失敗，不是永遠在跑的圈 | `repo:375` 讀回驗證 | `repo:375` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 提問回 202 而非 200 | 回應說得出去哪裡找答案 | `assistant_conversation_controller.go` `Ask` | `ctrl:122` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 進行中時提問回 409 | 與其他四種狀態碼分開 | `respondWithError` | `ctrl:164` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 三種狀態且只有三種 | 進行中／已回答／失敗 | `AssistantTurnStatusVo` | `conv:141`, `conv:265` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 三道檢查通過之後才留下 | 沒通過什麼都不留 | `Ask` 的順序 | `svc:291`, `svc:314`, `svc:876` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 驅動那段不綁在請求上 | 連線斷了照樣跑完 | `assistant_answer_writer.go:42` | — | no-test | produces-oracle | 🟠 mis-asserted |
| BR-4 | 失敗留紀錄但用量不計 | 說得出原因，用量為零 | `ToFailedTurn` | `exch:174`, `svc:638`, `repo:337` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 讀回時三種狀態都讀得到 | 依時間由早到晚 | `messages()` | `conv:141` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 一段對話最多一次進行中 | 已有就拒絕新的 | `HasAnswerInFlight` | `svc:681`, `conv:236` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 啟動時收殘留 | 原因為「被重新啟動中斷」 | `main.go` + `FailInterruptedAnswers` | `svc:767`, `repo:375` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `assistant_turn_status_vo.go:38` 空字串讀作已回答 | 這個欄位存在之前寫的列都有答案，答完才是唯一為真的讀法 | PRD Risks「既有列要有合理的預設」的落點，非孤兒 |
| `assistant_turn_status_vo.go:43` 認不得的值讀作失敗 | 讀成進行中會是一個永遠轉不完的圈 | 同上，屬同一條風險的另一半，非孤兒 |
| `conversation_domain.go` `RecentMessages` 濾掉未答完的 | 一個沒有答案的提問會被助手讀成它拒答過 | ARCH 未載明，但屬 BR-5 同源的必要後果；已補測試 `conv:219` | 
| `assistant_answer_writer.go:71` 寫回失敗時吞掉 | 沒有人在等，能記錄這件事的正是寫不進去的那一列 | 由 BR-7 的掃殘留兜底，程式碼內已說明，非孤兒 |

## Summary

- Conforms: 23/25 clauses ✅ (92%)
- Violations: 無
- Mis-asserted: AC-7、BR-3（**同一件事的兩種說法**：答案跑在 `context.Background()`
  而不是請求的 context 上。它是一行程式碼、也是這整個切片存在的理由，但沒有一條測試
  直接釘它——要釘它得取消呼叫端的 context 再證明答案仍然寫得回去，而那需要一個會
  觀察 context 的假助手，也就是一個手寫的 Fake，`testing.md` 明文禁止。
  間接的證據是：`svc` 的每一條都用 `t.Context()` 呼叫 `Ask`、而 `Ask` 回來之後測試
  才等到 `CompleteTurn`——若答案綁在那個 context 上，測試結束時它會被砍掉。
  這是真的保護，只是它保護的方式是「若改壞了會有一整批測試掛掉」而不是一條專屬斷言）
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 0

本次稽核修補的缺口：`ConversationDomain` 的三種狀態、未答完的提問不給助手看、
以及「有沒有回答在進行中」原本完全沒有測試，已補上 `conv:141`／`conv:219`／
`conv:236`／`conv:265` 四組。

> 稽核性質：靜態一致性稽核。它比對測試斷言與程式路徑對上規格的預期結果，
> 不執行自行發明的情境，也不以整份測試套件的綠燈作為判準。
