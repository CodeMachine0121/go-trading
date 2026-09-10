# Contract Traceability Matrix — Telegram 投遞

Contract: `.sdd/2026-09-10-telegram-delivery/PRD.md`
Design map: `.sdd/2026-09-10-telegram-delivery/ARCH.md`
Implementation: `internal/{domain,application,controller,infrastructure}` · `cmd/server`
Oracle: Acceptance Criteria（28 個情境）＋ 核心業務規則（11 條）＋ 非功能需求（4 條）

> 這是一次**靜態一致性稽核**：逐條把測試的斷言與程式的執行路徑各自對照規格導出的預期結果，
> 不是「跑一次測試看綠燈」。判定不採信 pass/fail。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 第一次設定 | 成功；讀回來看到聊天室代號與金鑰結尾 | `telegram_delivery_service.go:68` · `telegram_delivery_repository.go:54` | `telegram_delivery_application_test.go:136` · `telegram_delivery_repository_test.go:33` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 再設定一次是覆蓋，不是多一份 | 仍然只有一份，內容是新的那一份 | `telegram_delivery_repository.go:54`（`OnConflict`）· entity `UserID` 唯一索引 | `telegram_delivery_repository_test.go:49`（並斷言列數為 1） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 金鑰不得為空白 | 拒絕並顯示「必須給一組機器人金鑰」 | `telegram_delivery_domain.go:36` | `telegram_delivery_domain_test.go:62` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 聊天室代號不得為空白 | 拒絕並顯示「必須給一個聊天室代號」 | `telegram_delivery_domain.go:43` | `telegram_delivery_domain_test.go:62` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 前後空白不予保留 | 成功；讀回來沒有前後空白 | `telegram_delivery_domain.go:35/42` | `telegram_delivery_domain_test.go:13` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 沒有登入就設定不了 | 拒絕並顯示「請重新登入」 | 路由掛在 `requiresSignIn` 後 | `telegram_delivery_controller_test.go:144` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 讀回設定只看得到金鑰結尾 | 只看到結尾；回覆裡找不到完整金鑰 | `dto/telegram_delivery_dto.go`（型別上無此欄位）· 存下來的 `BotTokenTail` | `telegram_delivery_controller_test.go:101`（斷言 body 不含 sealed token） · `telegram_delivery_application_test.go:72` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 很短的金鑰仍然遮到拼不回去 | 回覆裡看不到那 4 個字的完整內容 | `telegram_delivery_domain.go:83`（長度 ≤ 4 回空） | `telegram_delivery_domain_test.go:119` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 留存處沒有可以直接拿去用的金鑰 | 找不到金鑰原文 | `aes_secret_seal_proxy.go:69` | `aes_secret_seal_proxy_test.go:31` · `telegram_delivery_domain_test.go:164` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 系統開不了鎖時寧可拒絕 | 拒絕並說明系統存不了；不以不上鎖的方式存下去 | `aes_secret_seal_proxy.go:70`（無「就先不上鎖」分支） | `aes_secret_seal_proxy_test.go`（三種壞鑰匙皆 `ErrSecretSealUnavailable` 且不吐出可存的東西）· `telegram_delivery_application_test.go:171`（未註冊 `Upsert`，觸及即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 還沒設定過不是錯誤 | 回一個說自己還沒設定的答案 | `telegram_delivery_service.go:52` | `telegram_delivery_application_test.go:105` · `telegram_delivery_controller_test.go:118`（200） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 看不到別人的 | 乙看到「還沒有設定」，看不到甲的聊天室代號 | `telegram_delivery_repository.go:34`（依 `user_id` 收斂） | `telegram_delivery_repository_test.go:98` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 改不動別人的 | 甲那一份原封不動；兩人各有各的 | 同上 | `telegram_delivery_repository_test.go:98/142` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 送得出去 | 回報送出成功 | `telegram_delivery_service.go:110` | `telegram_delivery_application_test.go:259` · `telegram_message_delivery_proxy_test.go:40` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 還沒設定就送不了 | 拒絕並說要先完成設定 | `telegram_delivery_repository.go:39` → `ErrTelegramDeliveryNotConfigured` | `telegram_delivery_application_test.go:322` · `telegram_delivery_controller_test.go:310`（409） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 空白訊息被拒絕 | 拒絕並顯示「訊息不得為空白」 | `test_message_domain.go:31` | `test_message_domain_test.go:52` · `telegram_delivery_application_test.go:334` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 剛好 4096 個字元送得出去 | 送出成功 | `test_message_domain.go:36` | `test_message_domain_test.go:12` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 4097 個字元被拒絕而不是被截短 | 拒絕並說明上限；沒有把前 4096 個送出去 | `test_message_domain.go:36`（拒絕，不截斷） | `test_message_domain_test.go:52` · `telegram_delivery_application_test.go:334`（未註冊 `Deliver`，觸及即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 用的是存起來的那一份設定 | 訊息被送往已存的聊天室 | `telegram_delivery_service.go:122` · request body 無金鑰欄位 | `telegram_delivery_application_test.go:259`（斷言 credential 內容） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 金鑰不被接受 | 回報金鑰不被接受；已存的設定原封不動 | `telegram_message_delivery_proxy.go:120`（401／404） | `telegram_message_delivery_proxy_test.go:78` · `telegram_delivery_application_test.go:287` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 找不到這個聊天室 | 回報找不到聊天室；已存的設定原封不動 | `telegram_wire.go`（`BlamesTheChat`）· proxy `reasonFrom` | `telegram_message_delivery_proxy_test.go:78`（三種措辭） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 連不上 Telegram | 回報連不上；已存的設定原封不動 | `telegram_message_delivery_proxy.go:93` | `telegram_message_delivery_proxy_test.go:201` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 等太久 | 回報等太久；已存的設定原封不動 | `telegram_message_delivery_proxy.go:89`（`DeadlineExceeded`） | `telegram_message_delivery_proxy_test.go:176/188` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 四種原因彼此不同 | 四次得到的說明各不相同 | `vo/delivery_failure_reason_vo.go`（四個取值） | `delivery_failure_reason_vo_test.go`（斷言四個取值互不重複）· `telegram_delivery_controller_test.go:276` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 移除之後就送不出訊息 | 移除成功；之後送測試訊息會被要求先設定 | `telegram_delivery_repository.go:76` | `telegram_delivery_repository_test.go:116` · `telegram_delivery_controller_test.go:310` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 沒有設定的人要求移除也不算失敗 | 回報現在沒有設定，不算失敗 | `telegram_delivery_repository.go:76`（刪 0 列不報錯）· controller 回 204 | `telegram_delivery_repository_test.go:132` · `telegram_delivery_controller_test.go:221` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 移除的是自己的 | 甲沒有設定了；乙那一份仍在 | `telegram_delivery_repository.go:79` | `telegram_delivery_repository_test.go:142` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 送出時剛好整份設定被移除（Edge Case） | 送出失敗並要求先完成設定 | `telegram_delivery_service.go:117` | `telegram_delivery_application_test.go:322`（同一條路徑） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 每人最多一份 | 見 AC-2 | entity 唯一索引 | `telegram_delivery_repository_test.go:49` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 只碰得到自己的 | 讀寫刪送皆限於本人 | 每一個 repository 方法皆以 `userID` 收斂；request 無使用者欄位 | `telegram_delivery_repository_test.go:85/98/142` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 金鑰是單向給出去的 | 沒有任何一條路把完整金鑰交回呼叫端 | `dto/telegram_delivery_dto.go` 型別上無此欄位 | `telegram_delivery_controller_test.go:101/154`（斷言回覆不含） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 金鑰上鎖留存，鑰匙不放旁邊 | 留存內容不是金鑰；同一金鑰鎖兩次結果不同 | `aes_secret_seal_proxy.go:69`（AES-GCM＋每次新 nonce） | `aes_secret_seal_proxy_test.go:31` 與「每次不同」那一則 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 開不了鎖就不存 | 見 AC-10 | `aes_secret_seal_proxy.go:70` | 同 AC-10 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 聊天室代號不上鎖、照原樣顯示 | 讀回來看得到完整的聊天室代號 | `entities/telegram_delivery.go:53` | `telegram_delivery_controller_test.go:101` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 送出用的是已存的那一份 | 沒有「這次先用這一組試試看」的路 | `models/test_message_request.go`（只有 message 欄位） | `telegram_delivery_application_test.go:259` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 訊息超過上限是拒絕，不是截斷 | 見 AC-18 | `test_message_domain.go:36` | `test_message_domain_test.go:52` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 四種投遞失敗原因不得合成一句 | 見 AC-24 | `vo/delivery_failure_reason_vo.go` | `delivery_failure_reason_vo_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 送不出去不改動已存的設定 | 設定原封不動 | `telegram_delivery_service.go:110`（`SendTestMessage` 只讀不寫） | `telegram_delivery_application_test.go:287`（未註冊 `Upsert`／`DeleteByUser`，觸及即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 移除一份不存在的設定不算失敗 | 見 AC-26 | `telegram_delivery_repository.go:76` | `telegram_delivery_repository_test.go:132` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | Edge Case：使用者被刪除，設定跟著不在 | 那把鑰匙不比它的主人活得久 | `entities/user.go`（`OnDelete:CASCADE`） | `telegram_delivery_repository_test.go:163` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-13 | Edge Case：開鎖失敗不是「金鑰不被接受」 | 回報系統問題，不是使用者填錯 | `telegram_delivery_service.go:130` | `telegram_delivery_application_test.go:366` · `telegram_delivery_controller_test.go:333`（503） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-14 | Edge Case：沒見過的拒絕歸到「連不上」 | 回報連不上，不猜 | `telegram_message_delivery_proxy.go:129` | `telegram_message_delivery_proxy_test.go:78`（「措辭不認得」那一列） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-15 | Edge Case：訊息照字面送出，不做格式解讀 | 送出去的就是他打的那串 | proxy 送 `text`，未設 `parse_mode` | `telegram_message_delivery_proxy_test.go:51`（斷言 body 的 `text`） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 送出有等待上限（10 秒等級） | 超過即當作沒送成 | `config/application_config.go`（`TELEGRAM_REQUEST_TIMEOUT_SECONDS`，預設 10） | `config/tests/application_config_test.go`（預設與覆寫）· `telegram_message_delivery_proxy_test.go:176` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 金鑰全程當鑰匙對待：上鎖、只給結尾、不寫進紀錄檔、不回覆 | 任何輸出都不含完整金鑰 | proxy 的錯誤不含位址（位址內含金鑰） | `telegram_message_delivery_proxy_test.go:232`（斷言錯誤訊息不含金鑰與位址）；**紀錄檔本身無測試** | shallow（涵蓋回傳值，未涵蓋 log） | produces-oracle（全路徑無 log 呼叫） | 🟠 mis-asserted |
| NFR-3 | 開鎖的東西不與留存處放在一起 | 缺鑰匙即拒絕留存 | `SECRET_SEAL_KEY` 無預設值 | `application_config_test.go`（斷言預設為空） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 轉達 Telegram 的說明時不得夾帶金鑰 | 往上傳的訊息不含金鑰 | proxy 不轉貼原始請求 | `telegram_message_delivery_proxy_test.go:219/232` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `telegram_message_delivery_proxy.go:71` | `json.Marshal` 失敗的分支 | 防禦性；輸入是兩個字串，無法由測試觸發。非違規 |
| `aes_secret_seal_proxy.go:52/57/76` | `aes.NewCipher`／`cipher.NewGCM`／`rand.Read` 的錯誤分支 | 防禦性；鑰匙長度已先驗過，前兩者不可能失敗。非違規 |
| `telegram_delivery_controller.go:31` GET 回 200 而非 404 | PRD §5 有寫，未列為獨立 AC | 建議日後升格為 AC |

## Summary

- Conforms: 45/46 clauses ✅（98%）
- Violations: 無
- Mis-asserted: `NFR-2`（「不寫進紀錄檔」沒有測試守著。位址與金鑰不外流**有**測試；
  「沒有任何一行 log 寫出金鑰」目前只靠人工檢視。這是本切片最容易在日後被破壞的一條）
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 3（皆非違規；兩個是無法由測試觸發的防禦性分支，一個是設計決定未升格為 AC）

**已知未涵蓋的程式路徑**：上表 Orphans 前兩列。刻意保留而不刪除——它們是驅動層失敗的傳遞。

**這次稽核的天花板**：靜態一致性稽核。它閱讀測試的斷言與程式的執行路徑並各自對照規格，
**不自行撰寫或執行新的探針**。動態證明請走 `/tdd`。
