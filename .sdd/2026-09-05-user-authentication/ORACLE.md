# 使用者登入 — Oracle（預期結果，先於實作寫下）

每一列的「預期結果」都只從 `PRD.md` 的 AC 場景推導，**不參考任何實作**。
測試的每一個斷言值一律取自本表；若某個值只能靠跑實作才得知，就是本表不完整，回頭補或問。

## EmailDomain — 電子郵件的唯一一套規則（US-01、US-03）

| # | 輸入 | 預期結果 |
|---|---|---|
| E1 | `"james@example.com"` | 通過，`Value()` = `"james@example.com"` |
| E2 | `"  james@example.com  "` | 通過，`Value()` = `"james@example.com"` |
| E3 | `"James@Example.com"` | 通過，`Value()` = `"james@example.com"` |
| E4 | `"　JAMES@Example.com　"`（全形空白） | 通過，`Value()` = `"james@example.com"` |
| E5 | `""` | 拒絕，`ErrUserValidation`，訊息含「必須給一個電子郵件」 |
| E6 | `"   "` | 拒絕，`ErrUserValidation`，訊息含「必須給一個電子郵件」 |
| E7 | `"　"`（僅全形空白） | 拒絕，`ErrUserValidation`，訊息含「必須給一個電子郵件」 |
| E8 | `"not-an-email"` | 拒絕，`ErrUserValidation`，訊息含「電子郵件」與「格式」 |
| E9 | `"james@"` | 拒絕，`ErrUserValidation` |
| E10 | `"@example.com"` | 拒絕，`ErrUserValidation` |
| E11 | `"james example@x.com"`（中間有空白） | 拒絕，`ErrUserValidation` |
| E12 | `"James Hsueh <james@example.com>"`（帶顯示名） | 拒絕，`ErrUserValidation`——帳號是那個位址本身，不是一列通訊錄 |
| E13 | `"james@example.com\x00"`（含空字元） | 拒絕，`ErrUserValidation` |

## PasswordDomain — 密碼的長度規則（US-02）

長度下限 8 個**字元**（rune），上限 72 個**位元組**（UTF-8）。

| # | 輸入 | 預期結果 |
|---|---|---|
| P1 | `"correct horse"`（13 字元） | 通過，`Value()` 與輸入完全相同 |
| P2 | `"12345678"`（剛好 8 字元） | 通過 |
| P3 | `"1234567"`（7 字元） | 拒絕，`ErrUserValidation`，訊息含「至少」與「8」 |
| P4 | `""` | 拒絕，`ErrUserValidation`，訊息含「必須給一組密碼」 |
| P5 | `"        "`（8 個半形空白） | 通過——密碼**不去空白**，空白是密碼的一部分 |
| P6 | 72 個 `"a"`（72 位元組） | 通過 |
| P7 | 73 個 `"a"`（73 位元組） | 拒絕，`ErrUserValidation`，訊息含「72」；**不截短** |
| P8 | 24 個中文字（72 位元組、24 字元） | 通過 |
| P9 | 25 個中文字（75 位元組、25 字元） | 拒絕，`ErrUserValidation`，訊息含「72」 |
| P10 | 3 個中文字（9 位元組、3 字元） | 拒絕，`ErrUserValidation`（下限數的是字元，3 < 8） |

## UserRegistrationDomain — 一次建立的完整把關（US-01、US-02）

| # | 輸入 | 預期結果 |
|---|---|---|
| N1 | `"James@Example.com"` / `"correct horse"` | 通過；`ToEntity("proof")` 的 `Email` = `"james@example.com"`、`PasswordProof` = `"proof"`、`ID` = 0 |
| N2 | 電子郵件不合格、密碼合格 | 拒絕，`ErrUserValidation`，訊息是電子郵件那一條 |
| N3 | 電子郵件合格、密碼不合格 | 拒絕，`ErrUserValidation`，訊息是密碼那一條 |
| N4 | 兩者皆不合格 | 拒絕，`ErrUserValidation`，訊息是**電子郵件**那一條（先驗電子郵件） |
| N5 | 通過後讀 `Password()` | 等於原始密碼（要拿去算證明），且 `ToEntity` 產出的 entity **沒有任何欄位帶著它** |

## SignInDomain — 登入的把關，只有一種說法（US-03、US-04）

| # | 輸入 | 預期結果 |
|---|---|---|
| S1 | `"james@example.com"` / `"correct horse"` | 通過；`Email()` = `"james@example.com"`、`Password()` = `"correct horse"` |
| S2 | `"　JAMES@Example.com　"` / `"correct horse"` | 通過；`Email()` = `"james@example.com"` |
| S3 | `""` / `"correct horse"` | 拒絕，`ErrCredentialsRejected`，訊息 = `"電子郵件或密碼不正確"` |
| S4 | `"not-an-email"` / `"correct horse"` | 拒絕，`ErrCredentialsRejected`——**不是** `ErrUserValidation`；連格式都不透露 |
| S5 | `"james@example.com"` / `""` | 拒絕，`ErrCredentialsRejected` |
| S6 | `"james@example.com"` / `"1234567"`（比下限短） | **通過**——建立時的長度規則在這裡刻意不套用。短密碼不是格式問題，它只是不對，所以由後面的比對回絕（見 `UserService.SignIn` 的 I11），而不是在這裡 |
| S7 | S3 與 S4 的錯誤訊息 | **一字不差**相同 |

## User entity — 對外的形狀（US-01、US-05）

| # | 輸入 | 預期結果 |
|---|---|---|
| U1 | `User{ID: 7, Email: "james@example.com", PasswordProof: "$2a$..."}` | `ToDto()` = `UserDto{ID: 7, Email: "james@example.com"}` |
| U2 | `UserDto` 的欄位 | **只有** `ID` 與 `Email`；型別上沒有放密碼證明的位置 |

## UserService.RegisterUser — 建立使用者（US-01、US-02）

| # | 情境 | 預期結果 |
|---|---|---|
| R1 | 合格輸入 | 呼叫 `Prove(密碼)` 一次；`Save` 收到的 entity `ID` = 0、`Email` 已小寫、`PasswordProof` = `Prove` 的回傳；回覆 `UserDto{ID, Email}` |
| R2 | 電子郵件不合格 | 回 `ErrUserValidation`；**`Prove` 與 `Save` 都不被呼叫** |
| R3 | 密碼不合格 | 回 `ErrUserValidation`；**`Prove` 與 `Save` 都不被呼叫** |
| R4 | `Prove` 失敗 | 回該錯誤；`Save` 不被呼叫 |
| R5 | `Save` 回 `ErrEmailAlreadyRegistered` | 原樣回傳（controller 對映 409） |
| R6 | `Save` 回其他儲存失敗 | 原樣回傳 |

## UserService.SignIn — 登入（US-03、US-04）

有效期限 24 小時；`IClockProxy.Now()` 固定為 `2026-09-05T08:00:00Z`。

| # | 情境 | 預期結果 |
|---|---|---|
| I1 | 電子郵件與密碼都對 | `Issue` 收到 `userID` = 該使用者、`expiresAt` = `2026-09-06T08:00:00Z`；回覆 `AccessTokenDto{Token, ExpiresAt}` |
| I2 | `FindOneByEmail` 收到的參數 | 是**去空白轉小寫後**的電子郵件 |
| I3 | 密碼對不上 | 回 `ErrCredentialsRejected`，訊息 = `"電子郵件或密碼不正確"`；**`Issue` 不被呼叫** |
| I4 | `FindOneByEmail` 回 `ErrUserNotFound` | 回 `ErrCredentialsRejected`（同一句）；**`Matches` 仍被呼叫一次，且第二個參數是 `DecoyProof()` 的回傳**；`Issue` 不被呼叫 |
| I5 | I3 與 I4 的錯誤訊息 | **一字不差**相同 |
| I6 | 登入內容空白 / 格式不對 | 回 `ErrCredentialsRejected`；儲存層**完全不被觸碰** |
| I7 | `FindOneByEmail` 回其他儲存失敗 | 原樣回傳（**不**偽裝成 `ErrCredentialsRejected`——那是系統壞了，不是密碼錯） |
| I8 | `Issue` 失敗 | 原樣回傳 |
| I9 | 全程 | **沒有任何寫入**：`Save` 一次都不被呼叫 |
| I10 | 有效期限 1 小時 | `expiresAt` = `2026-09-05T09:00:00Z` |
| I11 | 密碼比建立時的下限還短、但確實對得上留存的證明 | 登入**成功**——長度規則管的是密碼設得成什麼，不是它現在對不對 |

## UserService.IdentifyUser — 我是誰（US-05）

| # | 情境 | 預期結果 |
|---|---|---|
| D1 | 憑證認得出使用者 7，且該使用者存在 | 回 `UserDto{ID: 7, Email: ...}` |
| D2 | `UserIdentifiedBy` 回 `ErrAuthenticationRequired` | 原樣回傳；`FindOne` 不被呼叫 |
| D3 | 憑證認得出使用者 7，但 `FindOne` 回 `ErrUserNotFound` | 回 `ErrAuthenticationRequired`，訊息含「重新登入」 |
| D4 | 憑證是空字串 | 回 `ErrAuthenticationRequired`；**proxy 不被呼叫** |
| D5 | `FindOne` 回其他儲存失敗 | 原樣回傳（不偽裝成要重新登入） |

## BcryptPasswordProofProxy — 密碼證明（US-02）

| # | 情境 | 預期結果 |
|---|---|---|
| B1 | `Prove("correct horse")` | 回一段**不等於** `"correct horse"` 的字串 |
| B2 | 同一組密碼 `Prove` 兩次 | 兩段證明**彼此不同**（摻了不同的隨機料） |
| B3 | B2 的兩段證明 | `Matches("correct horse", 證明)` 兩者皆 true |
| B4 | `Matches("wrong horse", 正確證明)` | false |
| B5 | `Matches("correct horse", "not-a-proof")` | false（壞掉的證明不算對上，也不 panic） |
| B6 | `Prove` 73 個位元組的密碼 | 回錯誤——**寧可失敗也不悄悄只算前 72 個** |
| B7 | `DecoyProof()` | 回一段非空字串，且 `Matches(任意密碼, 它)` 為 false |
| B8 | `DecoyProof()` 呼叫兩次 | 兩次相同（建構時算一次記著，不是每次重算） |

## JwtAccessTokenProxy — 登入憑證（US-03、US-05）

鑰匙 = `"test-signing-key"`；`expiresAt` = `2026-09-06T08:00:00Z`。

| # | 情境 | 預期結果 |
|---|---|---|
| J1 | `Issue(7, expiresAt)` | 回 `AccessTokenVo`：`Token` 非空、`ExpiresAt` = `2026-09-06T08:00:00Z` |
| J2 | J1 的憑證交給 `UserIdentifiedBy` | 回 `7` |
| J3 | 把 J1 憑證的最後一個字元換掉 | 回 `ErrAuthenticationRequired` |
| J4 | `Issue(7, 已經過去的時刻)` 產出的憑證 | `UserIdentifiedBy` 回 `ErrAuthenticationRequired` |
| J5 | `UserIdentifiedBy("")` | 回 `ErrAuthenticationRequired` |
| J6 | `UserIdentifiedBy("not.a.token")` | 回 `ErrAuthenticationRequired` |
| J7 | 用鑰匙 A 簽、用鑰匙 B 驗 | 回 `ErrAuthenticationRequired` |
| J8 | 鑰匙為空字串時 `Issue` | 回錯誤且 `errors.Is(err, ErrAccessTokenUnavailable)` 為 true |
| J9 | 鑰匙為空字串時 `UserIdentifiedBy(任意)` | 回 `ErrAuthenticationRequired`（沒有鑰匙就誰都認不得） |
| J10 | 一份把簽章演算法改成 `none` 的憑證 | 回 `ErrAuthenticationRequired` |

## UserController — 狀態碼對映（全部 US）

| # | 情境 | 預期結果 |
|---|---|---|
| C1 | `POST /users` 成功 | 201，body = `{"id":7,"email":"james@example.com"}`，**不含任何密碼欄位** |
| C2 | `POST /users` 的 body 不是合法 JSON | 400 |
| C3 | `POST /users` 回 `ErrUserValidation` | 400，body 帶 `message` |
| C4 | `POST /users` 回 `ErrEmailAlreadyRegistered` | 409 |
| C5 | `POST /users` 回其他錯誤 | 502 |
| C6 | `POST /sessions` 成功 | 200，body 帶 `accessToken` 與 `expiresAt`（RFC3339、UTC） |
| C7 | `POST /sessions` 回 `ErrCredentialsRejected` | 401，`message` = `"電子郵件或密碼不正確"` |
| C8 | `POST /sessions` 回 `ErrAccessTokenUnavailable` | 503 |
| C9 | `POST /sessions` 回其他錯誤 | 502 |
| C10 | `GET /users/me` 帶 `Authorization: Bearer <憑證>` | 200，body = `{"id":7,"email":"..."}` |
| C11 | `GET /users/me` 沒帶 `Authorization` | 401，`message` 含「重新登入」；**application 不被呼叫** |
| C12 | `GET /users/me` 帶 `Authorization: <憑證>`（少了 `Bearer `） | 401；application 不被呼叫 |
| C13 | `GET /users/me` 帶 `Authorization: Bearer `（憑證是空的） | 401；application 不被呼叫 |
| C14 | `GET /users/me` 的 `bearer` 小寫 | 200——標頭的類型不分大小寫是 HTTP 的規定 |
| C15 | `GET /users/me` 回 `ErrAuthenticationRequired` | 401 |
| C16 | `GET /users/me` 回其他錯誤 | 502 |

## UserRepository — 儲存（需 `TEST_POSTGRES_DSN`，未設則跳過）

| # | 情境 | 預期結果 |
|---|---|---|
| Q1 | `Save` 一位新使用者 | 回傳的 `ID` > 0、`CreatedAt` 非零值 |
| Q2 | `Save` 同一個電子郵件兩次 | 第二次回 `ErrEmailAlreadyRegistered`，訊息含該電子郵件；**資料庫裡仍然只有一位** |
| Q3 | `FindOneByEmail("james@example.com")` | 回該使用者，`PasswordProof` 與存進去的相同 |
| Q4 | `FindOneByEmail("nobody@example.com")` | 回 `ErrUserNotFound` |
| Q5 | `FindOne(存在的識別碼)` | 回該使用者 |
| Q6 | `FindOne(不存在的識別碼)` | 回 `ErrUserNotFound` |
| Q7 | 連線已關閉時的每一個方法 | 回儲存失敗，**不是**「找不到」 |
| Q8 | `SchemaMigrator.Migrate()` | 回傳的表名清單含 `Users` |
