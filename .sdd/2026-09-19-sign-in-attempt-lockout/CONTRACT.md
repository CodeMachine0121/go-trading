# Contract Verification — 登入失敗鎖定

**Oracle:** `PRD.md` §3 Acceptance Criteria（17 個 Scenario）＋ §6 Non-Functional Requirements
**Scope:** `go-trading`
**Ceiling:** 靜態一致性稽核。逐條把**測試的斷言**與**程式碼的路徑**各自對照規格推導出的預期，
**不以整套測試綠燈為判準**，也不自行撰寫新的探針。

---

## Clauses

| ID | Clause（Scenario） | Oracle（只由規格推導） | Implementation | Test | Test audit | Code audit | Status |
| :-- | :--- | :--- | :--- | :--- | :--- | :--- | :-- |
| AC-1 | 沒被鎖住的帳號用正確密碼照常登入 | 登入成功拿到身分證明；次數仍為 0 | `user_service.go:127,144` | `user_application_test.go:1439` | asserts-oracle | produces-oracle | ✅ |
| AC-2 | 第一次猜錯只是記一筆，還沒被鎖住 | 拒絕「電子郵件或密碼不正確」；次數 1；未被鎖住 | `sign_in_lockout_domain.go:64` | `user_application_test.go:1406` | asserts-oracle | produces-oracle | ✅ |
| AC-3 | 第三次猜錯的那一次本身就被拒絕，而且當場鎖住 | 拒絕；次數 3；解除時刻 = 此刻 + 7 天 | `sign_in_lockout_domain.go:64` | `user_application_test.go:1421` | asserts-oracle | produces-oracle | ✅ |
| AC-4 | 第三次猜對了就不鎖 | 登入成功；次數 0；未被鎖住 | `sign_in_lockout_domain.go:87` | `user_application_test.go:1439` | asserts-oracle | produces-oracle | ✅ |
| AC-5 | 鎖住期間用錯誤密碼，得到的是被鎖住的那句話 | 拒絕，說已被鎖住且說出解除時刻 | `sign_in_lockout_domain.go:49` | `user_application_test.go:1455` | asserts-oracle | produces-oracle | ✅ |
| AC-6 | 鎖住期間正確的密碼一樣進不來 | 同 AC-5，且**比對密碼不被呼叫** | `user_service.go:127`（在比對之前） | `user_application_test.go:1475` | asserts-oracle | produces-oracle | ✅ |
| AC-7 | 鎖住期間一直試，解除時刻不往後延、次數不累加 | 每次都拒絕；解除時刻不變；次數不變 | `user_service.go:127`（提前 return，**走不到**寫入） | `user_application_test.go:1455,1475`（`assert.Empty(states)`） | asserts-oracle | produces-oracle | ✅ |
| AC-8 | 到了解除時刻那一刻就算解開 | 登入成功；次數 0；未被鎖住 | `sign_in_lockout_domain.go:100`（`now.Before`） | `user_application_test.go:1488`、`sign_in_lockout_domain_test.go:41` | asserts-oracle | produces-oracle | ✅ |
| AC-9 | 差一秒到不算解開 | 拒絕，說已被鎖住 | `sign_in_lockout_domain.go:100` | `sign_in_lockout_domain_test.go:41`（「one second of lock left」） | asserts-oracle | produces-oracle | ✅ |
| AC-10 | 鎖定期滿之後的第一次失敗從 1 重新數起 | 拒絕；次數 **1**（非 4）；未被鎖住 | `sign_in_lockout_domain.go:114`（`lockServed`） | `user_application_test.go:1504`、`sign_in_lockout_domain_test.go:129` | asserts-oracle | produces-oracle | ✅ |
| AC-11 | 變更密碼會把還沒滿的失敗次數歸零 | 變更成功；次數 0 | `user_repository.go:137` | `user_repository_test.go:334` | asserts-oracle | produces-oracle | ✅ |
| AC-12 | 變更密碼會解除鎖定 | 變更成功；次數 0；未被鎖住 | `user_repository.go:137`（同一交易） | `user_repository_test.go:334` | asserts-oracle | produces-oracle | ✅ |
| AC-13 | 查無此電子郵件仍然只有原本那一句 | 拒絕「電子郵件或密碼不正確」 | `user_service.go:186`（`userID == 0` 不寫入） | `user_application_test.go:1521` | asserts-oracle | produces-oracle | ✅ |
| AC-14 | 不存在的電子郵件試幾次都不會變成被鎖住的那句話 | 5 次都是同一句；**不留下任何東西** | `user_service.go:186` | `user_application_test.go:1521`（5 次迴圈＋`assert.Empty`） | asserts-oracle | produces-oracle | ✅ |
| AC-15 | 被鎖住的人照常續用身分證明 | 續用成功，換到一對全新的憑證 | `user_service.go` `RenewSession`——**完全不讀**鎖定狀態 | `user_application_test.go:1606` | asserts-oracle | produces-oracle | ✅ |
| AC-16 | 被鎖住的人照常登出 | 登出成功 | `user_service.go` `RevokeSession`——**完全不讀**鎖定狀態 | `user_application_test.go:1636` | asserts-oracle | produces-oracle | ✅ |
| AC-17 | 待開通的人一樣會被鎖住 | 拒絕；被鎖住 | `sign_in_lockout_domain.go`——不讀 `IsEnabled` | `user_application_test.go:1547` | asserts-oracle | produces-oracle | ✅ |
| NFR-1 | 鎖定狀態要在比對密碼之前就看 | 鎖住的帳號不花掉一次密碼比對 | `user_service.go:127` 在 `:135` 之前 | `user_application_test.go:1455`（未對 `Matches` 設期望 → 被呼叫即失敗） | asserts-oracle | produces-oracle | ✅ |
| NFR-2 | 被鎖住的拒絕只對存在的帳號說 | 查無此人永遠只有原本那句 | `user_service.go:186` | `user_application_test.go:1521`（`NotErrorIs(ErrSignInLocked)`） | asserts-oracle | produces-oracle | ✅ |
| NFR-3 | 記下失敗這件事失敗時，整次登入就是失敗 | 回傳儲存的錯誤，不是「密碼不正確」；不發憑證 | `user_service.go:133,144` | `user_application_test.go:1565,1583` | asserts-oracle | produces-oracle | ✅ |
| NFR-4 | 不得額外多查一次使用者 | 狀態與找人同一次取得 | `user_service.go:120`（`FindOneByEmail` 之後直接包成 domain） | *(靜態閱讀)* | n/a | produces-oracle | ✅ |
| NFR-5 | 一律以世界標準時間留存與比較 | 解除時刻為 UTC | `sign_in_lockout_domain.go:78`（`.UTC()`） | `sign_in_lockout_domain_test.go:129`（期望值為 UTC 常數） | asserts-oracle | produces-oracle | ✅ |
| NFR-6 | 一個帳號**被鎖住的那一刻**要留下一筆紀錄 | 鎖住時留下紀錄；一般的單次失敗**不留** | `user_service.go`（`countFailedSignIn` 內，僅在 `LockedUntil != nil` 時 `log.Printf`） | *(靜態閱讀：條件只在剛訂出解除時刻時成立)* | n/a | produces-oracle | ✅ |
| NFR-7 | 並行送進來的嘗試不得互相覆蓋 | 每一次嘗試都被算到，門檻照常到達 | `user_repository.go`（CAS 守衛）+ `user_service.go`（`countFailedSignIn` 重讀） | `user_repository_test.go`（100 並行，全數算到）、`user_application_test.go`（重讀後從真實數字往上加） | asserts-oracle | produces-oracle | ✅ |

> **AC-1 與 AC-4 共用一個測試**（`:1439`）。兩條的 `Given` 不同（次數 0 vs 次數 2），
> 而該測試設的是次數 2，因此它直接釘的是 AC-4；AC-1 由 `:1488`（乾淨帳號成功路徑）
> 與既有的 `TestUserApplicationSignIn` 一併覆蓋。兩條的 `Then` 相同，判 conforms。

---

## Orphans

| 行為 | 位置 | 判定 |
| :--- | :--- | :--- |
| 次數**超過**門檻（非僅等於）時仍然上鎖 | `sign_in_lockout_domain.go:69`（`<` 而非 `==`） | **不是違規**——防禦性：門檻若被調小，既有超標的列仍須被鎖。已在 ORACLE A13 記錄並測試 |
| `SaveSignInLockoutState` 對不存在的識別碼回 `ErrUserNotFound` | `user_repository.go:180` | **不是違規**——PRD §6 Reliability 的直接後果（寫不進去即整體失敗） |
| 被鎖住回 **429** | `user_controller.go:211` | **不是違規**——PRD §5 要求「兩種失敗要分得出來」，狀態碼是 ARCH 決定的具體形狀 |

**Out of Scope 檢查**：PRD 排除的五項（認來源、自助解鎖、擋住已在線的人、主動通知、登入失敗流水帳）
**程式碼中皆無對應實作**。`entities.User` 只多兩個欄位，沒有任何稽核表或通知路徑。✅

---

## Summary

```
✅ 24 conforms · 🔴 0 violations · 🟠 0 mis-asserted · 🟡 0 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 0 orphans
Conformance: 100%（24 / 24）
```

> **第一版這份矩陣漏了一條，而且漏得不誠實。** PRD §6 Observability（帳號被鎖住的那一刻要留下紀錄）
> 當時**沒有實作**，而這份矩陣**沒有把它列進條款**——於是「22/22、100%」是把分母做小做出來的。
> 由 PR 審查抓出來。現在那條實作了、列進來了，並且多了一條 NFR-7（並行不得互相覆蓋）。

**這一輪稽核找到的唯一缺口，已經補上。**

第一次跑出來時 AC-15、AC-16 是 🟡 partial：程式碼是對的（續用與登出這兩條路
**完全不讀**鎖定狀態，所以被鎖住的人本來就不受影響），但**沒有任何測試釘住這件事**。

那正是 ARCH §2「刻意不動」點名的風險——下一個維護的人很可能覺得
「鎖住的人不該還能續用」而順手加上判斷，而當時沒有任何東西會因此變紅。
補上的兩個測試不是為了證明現在是對的，是為了讓未來那個改動撞牆；
把那個改動模擬加進 `RenewSession` 之後，測試確實變紅。
