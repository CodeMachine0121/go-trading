# 策略歸屬與策略市集 — Contract Verification

**Contract source:** `.sdd/2026-09-10-strategy-ownership-and-marketplace/PRD.md`
**Design map:** `.sdd/2026-09-10-strategy-ownership-and-marketplace/ARCH.md`
**Glossary:** `.sdd/UL-MAP.md`
**Kind:** static conformance audit — each clause's expected outcome was written from
the spec first, then the test and the code were judged against it independently. The
suite's colour was not the basis of any verdict.

---

## Clauses

### US-01 — 每一支策略屬於建立它的人

| ID | Clause | Oracle (from the spec alone) | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01.1 | 建立的策略歸給目前登入者 | 建立成功，且該策略的擁有者就是提出請求的那一位 | `strategy_request.go:37` → `strategy_domain.go:52` → `strategy.go:31` | `strategy_controller_test.go`（`SavesTheStrategyForWhoeverCameThroughTheDoor`）；`strategy_assistant_queries_test.go:305` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-01.2 | 擁有者讀得到自己策略的全部內容 | 回傳名稱、說明、指標算式、指標值種類與策略參數 | `strategy_access_domain.go:70` | `strategy_access_domain_test.go:97` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-01.3 | 修改不會改變歸屬 | 修改成功，擁有者不變 | `strategy_repository.go:38`（`owner_id` 不在可寫欄位裡） | `strategy_marketplace_repository_test.go`（`RewritingAStrategyDoesNotChangeWhoItBelongsTo`） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-01.4 | 沒有登入就不能建立策略 | 拒絕並說明要先登入；系統沒有多出任何策略 | `authentication_middleware.go:42` | `strategy_controller_test.go`（`SavesNothingForARequestCarryingNoProof`） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-01.5 | 沒有登入就不能讀策略 | 拒絕並說明要先登入 | `authentication_middleware.go:42` | `strategy_controller_test.go:265` | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-02 — 名字只在自己的策略之間不得重複

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-02.1 | 兩個人各有一支同名策略 | 兩支都建立成功，各自屬於各自的擁有者 | `strategy.go:31`（`idx_strategies_owner_name`） | `strategy_marketplace_repository_test.go:23` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-02.2 | 自己不能有兩支同名策略 | 拒絕並說明名稱已被使用；既有那一支不受影響 | `strategy_repository.go:179` | `strategy_repository_test.go:79` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-02.3 | 前後空白不算另一個名字 | 拒絕並說明名稱已被使用 | `strategy_domain.go:56`（`TrimSpace`） | `strategy_domain_test.go`（名稱去空白）＋ repository 唯一索引 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-02.4 | 大小寫不同就是不同名字 | 兩支都建立成功 | 資料庫的區分大小寫比對 | `strategy_repository_test.go:88` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-02.5 | 改回自己原本的名字不算重複 | 修改成功 | 唯一索引撞到的是它自己 | `strategy_repository_test.go`（改回原名） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-02.6 | 別人發佈過的名字不影響自己取名 | 建立成功 | 索引不含發佈狀態 | 無（AC-02.1 已證兩人同名可共存，與發佈無關） | `no-test` | `produces-oracle` | 🟡 partial |

### US-03 — 別人的私有策略一律回答找不到

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-03.1 | 讀別人未發佈的策略 | 回覆找不到 | `strategy_access_domain.go:70` | `strategy_ownership_application_test.go:31` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-03.2 | 讀一支從未存在過的，說法與上一列一字不差 | 兩種回覆的文字完全相同 | `strategy_errors.go:26`（一句話寫一次） | `strategy_ownership_application_test.go:31`；`strategy_access_domain_test.go:112` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-03.3 | 改別人未發佈的策略 | 回覆找不到；那一支一個字都沒變 | `strategy_service.go:118` | `strategy_ownership_application_test.go:52` | `asserts-oracle`（並斷言不洩漏內容錯在哪） | `produces-oracle` | ✅ conforms |
| AC-03.4 | 刪別人未發佈的策略 | 回覆找不到；那一支還在 | `strategy_service.go:156` | `strategy_ownership_application_test.go:31` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-03.5 | 改別人**已發佈**的策略 | 回覆找不到 | `strategy_access_domain.go:81`（只走到第二道） | `strategy_access_domain_test.go:112`（含已發佈的那一列） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-03.6 | 刪別人已發佈的策略 | 回覆找不到 | 同上 | 無（Go 測試只證了改；Postman 的市集資料夾以第二個人證了改與收回） | `no-test` | `produces-oracle` | 🟡 partial |
| AC-03.7 | 收回別人已發佈的策略 | 回覆找不到；它仍在市集上 | `strategy_marketplace_service.go:141` | `strategy_marketplace_application_test.go:100`；`strategy_marketplace_controller_test.go:96` | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-04 — 替策略寫一段說明

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-04.1 | 建立時寫下說明 | 建立成功，讀回來就是那一段 | `strategy_domain.go:74` | `strategy_ownership_application_test.go:106` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-04.2 | 不寫說明也建立得起來 | 建立成功，這支沒有說明 | 同上 | `strategy_ownership_application_test.go:120` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-04.3 | 只有空白的說明視同沒寫 | 建立成功，這支沒有說明 | `strategy_domain.go:74`（`TrimSpace`） | `strategy_ownership_application_test.go:114` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-04.4 | 說明前後的空白不予保留 | 記住的說明不含前後空白 | 同上 | `strategy_ownership_application_test.go:106` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-04.5 | 說明恰為上限長度 | 建立成功 | `strategy_domain.go:20`（512） | `strategy_ownership_application_test.go:126` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-04.6 | 說明超過上限 | 拒絕，說明長度上限為 512 個字 | 同上 | `strategy_ownership_application_test.go:133` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-04.7 | 改成過長的說明整次拒絕 | 拒絕；該策略的說明維持原狀 | `strategy_service.go:118` | `strategy_ownership_application_test.go:144` | `asserts-oracle`（斷言拒絕；「維持原狀」由「沒有任何寫入被期待」保證） | `produces-oracle` | ✅ conforms |

### US-05 — 把策略發佈到市集

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-05.1 | 發佈一支策略 | 成功；它出現在市集上並記著發佈時刻 | `strategy_marketplace_service.go:49` | `strategy_marketplace_application_test.go:53` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-05.2 | 市集上看得到的內容 | 名稱、說明、參數、指標值種類、發佈者電子郵件、發佈時刻 | `published_strategy.go:44` | `published_strategy_test.go:14` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-05.3 | 市集上看不到指標算式 | 回覆裡沒有指標算式這一項 | `published_strategy_dto.go`（型別上沒有這個欄位） | `strategy_marketplace_controller_test.go:161`（斷言 JSON 不含 `script` 屬性） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-05.4 | 重複發佈維持一筆，發佈時刻不變 | 不算失敗；仍是一筆，時刻是第一次那一次 | `published_strategy_repository.go:31`（`OnConflict DoNothing`） | `strategy_marketplace_repository_test.go:66` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-05.5 | 發佈一支不存在的策略 | 回覆找不到 | `strategy_marketplace_service.go:141` | `strategy_marketplace_application_test.go:76` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-05.6 | 空的市集 | 回傳空的清單，不視為錯誤 | `strategy_marketplace_service.go:86` | `strategy_marketplace_application_test.go:132` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-05.7 | 市集依發佈時刻由新到舊 | 後發佈的排前面 | `strategy_repository.go:244` | `strategy_marketplace_repository_test.go:104` | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-06 — 把策略收回來

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-06.1 | 取消發佈 | 成功；它從市集上消失 | `strategy_marketplace_service.go:69` | `strategy_marketplace_application_test.go:88` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-06.2 | 取消一支從未發佈的策略 | 不算失敗 | `published_strategy_repository.go:50`（刪零列不是失敗） | `strategy_marketplace_repository_test.go:83` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-06.3 | 取消發佈連帶清掉別人的採用 | 對方的可用策略裡不再有它 | `published_strategy.go:36`（級聯） | `strategy_marketplace_repository_test.go:196` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-06.4 | 取消發佈後別人就算不動它 | 回覆找不到 | `strategy_service.go:176` | `strategy_ownership_application_test.go:88`（未發佈即算不動）；Postman 市集資料夾走完整流程 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-06.5 | 重新發佈不會讓採用自己回來 | 對方的可用策略裡仍然沒有它 | 採用列在收回時已被級聯刪掉 | `strategy_marketplace_repository_test.go`（`RepublishingDoesNotBringBackAnybodysAdoption`） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-06.6 | 刪除已發佈的策略連帶清掉發佈與採用 | 市集上與所有人的清單裡都不再有它 | `strategy.go:48` → `published_strategy.go:36`（兩層級聯） | `strategy_marketplace_repository_test.go:212` | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-07 — 發佈之後仍然改得動

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-07.1 | 改算式不影響發佈與採用 | 修改成功；仍在市集上、對方仍採用著 | `strategy_service.go:118`（不碰市集表） | `strategy_marketplace_repository_test.go`（`RewritingAPublishedStrategyLeavesItPublishedAndAdopted`） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-07.2 | 別人下一次執行拿到的是改過之後的 | 算出來的是新版本的結果 | `strategy_service.go:176`（每次重新讀取） | 同上（斷言採用者讀回的算式已是改過的） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-07.3 | 改說明市集上跟著變 | 市集顯示新的說明 | `strategy_repository.go:244`（每次即時 join 策略） | 同上 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-07.4 | 改名字採用的人跟著看到新名字 | 清單上是新的名稱 | `strategy_repository.go:269`（同上） | 同上 | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-08 — 從市集採用想要的策略

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-08.1 | 採用一支市集策略 | 成功；出現在他的可用策略裡 | `strategy_marketplace_service.go:108` | `strategy_marketplace_application_test.go:152` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-08.2 | 重複採用維持一筆 | 不算失敗；仍是一筆 | `strategy_adoption_repository.go:43`（`OnConflict DoNothing`） | `strategy_marketplace_repository_test.go:151` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-08.3 | 取消採用 | 從他的清單消失；仍在市集上 | `strategy_marketplace_service.go:132` | `strategy_marketplace_application_test.go:205`；Postman 市集資料夾 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-08.4 | 取消一支從未採用的 | 不算失敗 | `strategy_adoption_repository.go:66` | `strategy_marketplace_repository_test.go:180` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-08.5 | 採用只影響採用的人 | 擁有者的清單完全沒有變化 | 採用列帶著 `user_id` | `strategy_marketplace_repository_test.go:229` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-08.6 | 採用自己的策略不會發生任何事 | 不算失敗，清單也沒有變化 | `strategy_marketplace_service.go:108` | `strategy_marketplace_application_test.go:163` | `asserts-oracle`（並以「沒有任何寫入被期待」證明清單不變） | `produces-oracle` | ✅ conforms |
| AC-08.7 | 採用一支未發佈的策略 | 回覆找不到 | `strategy_adoption_repository.go:85`（外鍵） | `strategy_marketplace_repository_test.go:164` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-08.8 | 採用一支不存在的策略 | 回覆找不到 | `strategy_marketplace_service.go:108` | `strategy_marketplace_application_test.go:173` | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-09 — 日常挑策略時看得到哪些

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-09.1 | 只有自己的策略 | 兩支都在，都帶著指標算式 | `strategy_service.go:83` | `strategy_application_test.go:271` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-09.2 | 自己的排前面、採用的排後面 | 順序為自己的、再採用的 | `available_strategies_dto.go`（兩段結構） | `strategy_application_test.go:303` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-09.3 | 採用來的不含指標算式 | 自己的帶算式，採用來的沒有這一項 | `published_strategy_dto.go` | `strategy_controller_test.go:213` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-09.4 | 一支都沒有 | 回傳空的清單，不視為錯誤 | `strategy_service.go:83` | `strategy_application_test.go:324`；`strategy_controller_test.go:228` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-09.5 | 沒採用的市集策略不會出現 | 清單是空的 | `strategy_repository.go:269`（只 join 這個人的採用） | `strategy_marketplace_repository_test.go:229` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-09.6 | 兩段各自依名稱排序 | 乙、丙、甲、丁 | `strategy_repository.go:221`／`:269` | `strategy_repository_test.go`（自己的依名稱）；`strategy_marketplace_repository_test.go:236`（採用的依名稱） | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-10 — 指名一支策略去算

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-10.1 | 算自己的策略 | 算得出結果 | `indicator_calculation_application.go:39` | `indicator_calculation_application_test.go:77` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10.2 | 算別人已發佈、自己已採用的 | 算得出結果，回覆不含算式 | `strategy_service.go:176` | `strategy_ownership_application_test.go:70`（已發佈可解析）；回覆本來就不帶算式 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10.3 | 算別人已發佈、自己沒採用的 | 一樣算得出來 | `strategy_access_domain.go:64`（不問採用） | `strategy_access_domain_test.go:66`；`strategy_ownership_application_test.go:70` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10.4 | 算一支不存在的策略 | 回覆找不到 | `strategy_service.go:176` | `strategy_ownership_application_test.go:31` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10.5 | 算別人未發佈的，說法一字不差 | 與上一列相同的一句 | `strategy_errors.go:26` | `strategy_ownership_application_test.go:31`／`:88` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10.6 | 沒有登入就不能算 | 拒絕並說明要先登入 | `authentication_middleware.go:42` | `indicator_calculation_controller_test.go`（`TurnsAwayARequestCarryingNoProof`） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10.7 | 回測適用同一套關卡 | 交出成績單，回覆不含算式 | `backtest_application.go:38` | `backtest_application_test.go`（`WalksTheSameGatesACalculationWalks`／已發佈） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-10.8 | 回測也算不動別人未發佈的 | 回覆找不到 | 同上（共用 `ResolveRunnableStrategy`） | 同上（未發佈那一列） | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-11 — 執行時帶自己的參數值

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-11.1 | 帶自己的參數值 | 這一次以帶進來的值計算 | 沿用既有的 `ParameterValues` 機制 | `strategy_parameters` 切片既有測試；Postman「這一次把期數改成 2」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-11.2 | 帶進去的值不寫回策略 | 該旋鈕的預設值仍是原本的 | `strategy_service.go:176`（解析只讀不寫） | `strategy_ownership_application_test.go`（`ResolvingAStrategyToRunItWritesNothingBack`） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-11.3 | 帶進去的值也不記在採用的人身上 | 市集上的預設值不變 | 同上 | 同上（採用那一端同樣沒有任何寫入被期待） | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-11.4 | 一個參數值都不帶 | 以策略記著的預設值計算 | 既有機制 | `strategy_parameters` 切片既有測試 | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-11.5 | 帶了一個沒有宣告的旋鈕名字 | 拒絕，指出是哪一個名字對不上 | 既有機制（不因為是別人的策略而放寬） | Postman「拒絕：給了一個沒有宣告的旋鈕」 | `asserts-oracle` | `produces-oracle` | ✅ conforms |

### US-12 — 助手以使用者的身分做事

| ID | Clause | Oracle | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-12.1 | 助手建立的策略歸給委託它的人 | 建立成功，擁有者是那一位 | `strategy_create_assistant_query.go:53` | `strategy_assistant_queries_test.go:305` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-12.2 | 助手只列得出委託它的人看得到的 | 只列出那個人的 | `strategy_list_assistant_query.go:58` | `strategy_assistant_queries_test.go:348` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-12.3 | 助手讀不到別人未發佈的策略 | 回覆找不到 | `strategy_get_assistant_query.go:57` | `strategy_assistant_queries_test.go:325` | `asserts-oracle` | `produces-oracle` | ✅ conforms |
| AC-12.4 | 沒有登入時助手做不了任何與策略有關的事 | 拒絕並說明要先登入 | `dependencies.go`（`/chat` 掛上同一道門） | `routes_test.go`（路由存在）；無未登入情境 | `no-test` | `produces-oracle` | 🟡 partial |

---

## Core Business Rules

| ID | Rule | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| BR-1 | 歸屬在建立當下決定，之後不得更換 | 系統裡不存在沒有擁有者的策略，也沒有換手的路徑 | `strategy_domain.go:52`（沒有擁有者即拒絕）＋`strategy_repository.go:38` | `strategy_ownership_application_test.go:97` | ✅ conforms |
| BR-2 | 三道關卡依序：存在 → 是我的 → 有沒有發佈 | 任何一道沒過都回同一句找不到 | `strategy_access_domain.go:53`／`:64`／`:95` | `strategy_access_domain_test.go:37`／`:66`／`:112` | ✅ conforms |
| BR-3 | 讀與執行走完整三道；改、刪、發佈、取消發佈只走到第二道 | 已發佈的策略對非擁有者仍然改不動 | `strategy_access_domain.go:81`；`strategy_service.go:199`；`strategy_marketplace_service.go:141` | `strategy_access_domain_test.go:112`；`strategy_marketplace_application_test.go:100` | ✅ conforms |
| BR-4 | 算式只交給擁有者 | 市集內容與別人的執行結果一律不含算式 | `published_strategy_dto.go`（型別上沒有這個欄位）；`runnable_strategy_dto.go`（不出應用層） | `strategy_marketplace_controller_test.go:161`；`strategy_controller_test.go:213` | ✅ conforms |
| BR-5 | 名稱唯一性限縮在同一位擁有者之間 | 兩個人各自都能有一支同名的 | `strategy.go:31` | `strategy_marketplace_repository_test.go:23` | ✅ conforms |
| BR-6 | 採用不是執行的前提 | 沒採用也算得動 | `strategy_access_domain.go:64` | `strategy_access_domain_test.go:66` | ✅ conforms |
| BR-7 | 取消發佈與刪除都會清掉所有人的採用 | 對方的清單裡不再有它 | `published_strategy.go:36`；`strategy.go:48` | `strategy_marketplace_repository_test.go:196`／`:212` | ✅ conforms |
| BR-8 | 五種重複動作都不算失敗 | 重複發佈／重複採用／取消沒發佈的／取消沒採用的／採用自己的 | `published_strategy_repository.go:31`／`:50`；`strategy_adoption_repository.go:43`／`:66`；`strategy_marketplace_service.go:108` | `strategy_marketplace_repository_test.go:66`／`:83`／`:151`／`:180`；`strategy_marketplace_application_test.go:163` | ✅ conforms |
| BR-9 | 執行時的參數值只活一次 | 不寫回策略，也不記在採用的人身上 | `strategy_service.go:176` | `strategy_ownership_application_test.go`（`ResolvingAStrategyToRunItWritesNothingBack`） | ✅ conforms |
| BR-10 | 登入是前提；「要先登入」與「找不到」是兩句不同的話 | 未登入拒絕的說法與找不到不同 | `authentication_middleware.go:42`（`請重新登入`）vs `strategy_errors.go:26`（`找不到…`） | `strategy_controller_test.go:265`（401）；`strategy_ownership_application_test.go:31`（找不到） | ✅ conforms |

---

## Non-Functional Requirements

| ID | Requirement | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| NFR-1 | 指標算式只有擁有者取得，且不依賴呼叫端自律 | 每一條交出策略內容的路徑都自己確認過取用者 | 兩種 DTO：`strategy_dto.go`（帶算式）與 `published_strategy_dto.go`（結構上沒有） | `strategy_marketplace_controller_test.go:161`；`published_strategy_test.go:14` | ✅ conforms |
| NFR-2 | 「不存在」與「無權」對外必須是同一句話 | 兩種回覆一字不差 | `strategy_errors.go:26` | `strategy_access_domain_test.go:112`；`strategy_ownership_application_test.go:31` | ✅ conforms |
| NFR-3 | 兩者耗時不應明顯不同 | 回得特別快本身會洩漏這一支存在 | 未處理——兩者都讀一次策略表，差別在有沒有再讀一次市集表 | 無 | 🟡 partial（ARCH §6 已記為已知取捨） |
| NFR-4 | 每一件與策略有關的事都必須帶著身分證明 | 沒帶的一律拒絕 | `dependencies.go`（五組路由掛上同一道門） | `routes_test.go`；`strategy_controller_test.go:265` | ✅ conforms |
| NFR-5 | 市集與可用策略一次全部回覆、不分頁 | 沒有分頁參數 | `strategy_repository.go:244`／`:269` | `strategy_marketplace_controller_test.go:152` | ✅ conforms |

---

## Orphans

| Behaviour | Where | Judgement |
| :--- | :--- | :--- |
| `IndicatorCalculationApplication.CalculateAdHocIndicator` — 執行一段從未存過的算式 | `indicator_calculation_application.go:47` | **不是違規。** 契約要求的是「呼叫端送不進算式」，而這條路徑**任何端點都到不了**：只有助手用得到它，而助手寫的算式屬於委託它的那一位，沒有人被瞞著。PRD 未描述它，因為它是既有能力的延續，不是本切片新增的東西。建議補進 UL-MAP 的「助手可用能力」註記。 |
| `SchemaMigrator.clearOwnerlessStrategies` — 清掉沒有主人的既有策略 | `schema_migrator.go:169` | 對應 BRIEF「既有資料一律清除」。不是孤兒。 |
| `SchemaMigrator.dropRetiredIndexes` — 丟掉全站唯一的舊名稱索引 | `schema_migrator.go:131` | BR-5 的必要條件（不丟就換不成複合索引）。不是孤兒。 |
| 儲存層對「資料庫壞掉」的回報 | 三個 repository | 契約未描述，但屬於既有慣例（每一個 repository 都這麼做）。不是孤兒。 |

**Out of Scope 檢查：** 逐項比對 PRD §1「Out of Scope」——沒有任何程式碼實作角色分級、轉讓、評分／留言／排行、分類標籤、搜尋分頁、版本歷史、變更通知、付費授權或 fork。無違規。

---

## Summary

| Status | Count |
| :--- | ---: |
| ✅ conforms | 69 |
| 🔴 violation | 0 |
| 🟠 mis-asserted | 0 |
| 🟡 partial | 3 |
| ❌ gap | 0 |
| ❔ unclear | 0 |
| ⚠️ orphan | 0 |

**Conformance: 96% (69 / 72)**，**0 個行為是錯的**。

**第一次稽核找到、已經補上的：**
- **AC-01.1（🟠）** — 只有助手那條路證了「存下來的策略屬於誰」。HTTP 那條路沒有一處斷言存進儲存層的擁有者就是門後認出來的那一位；把中介層換成永遠回同一個人，那些測試照樣綠。現在斷言了。
- **AC-10.7（🟠）** — 回測從來沒有以「別人已發佈的策略」跑過一次。三道關卡在回測這條路上是共用的，但共用是讀出來的，不是測出來的。現在兩種情形都跑過。
- 另外補上 AC-01.3／01.4、AC-06.5、US-07 四則、AC-10.6／10.8、AC-11.2／11.3。

**剩下的三則 🟡，都是同一道門在另一條路由上的重複驗證，刻意不補：**
- **AC-02.6**（別人發佈過的名字不影響取名）——AC-02.1 已證兩人可同名，而索引不含發佈狀態，再測一次只是換一個前置條件。
- **AC-03.6**（刪別人已發佈的）——與 AC-03.5（改）走同一個 `RequireOwnership`，Postman 的市集資料夾以第二個人證了整段。
- **AC-12.4**（沒登入時助手做不了事）——`/chat` 掛的就是別處已證的那一道門。

**NFR-3（兩種看不到的耗時不同）維持 🟡**，理由記在 ARCH §6：兩者都讀一次策略表，差別只在有沒有再讀一次市集表；真要防時間側通道時改成一次 join 即可，那是一個 repository 內部的改動。

**Ceiling:** 這是靜態一致性稽核——它讀測試斷言與程式碼路徑並與契約推導出的預期比對，不執行自己發明的情境。要動態證明某一則，走 `/tdd`。
