# Architecture — 登入失敗鎖定

**Feature:** 登入失敗鎖定
**PRD:** `.sdd/2026-09-19-sign-in-attempt-lockout/PRD.md`
**Status:** Draft

---

## 1. Design Goal

把「這個帳號還能不能試」變成**一個問題、一個答案**，而不是散在登入流程裡的一串 if。

登入這條路上已經有三件事要按順序做對（找人、花掉比對密碼的時間、簽發憑證），
而且每一件的順序都是刻意的、都寫了理由。這個切片再塞進來的是**第四件**，
所以它必須以**一個物件**的形式進來：`UserService.SignIn` 多問它兩句話
（「這一次要不要當場拒絕」「這一次之後這個帳號變成什麼樣子」），
而不是多出六行關於次數、時刻與比大小的程式碼。

次數怎麼加、期滿之後從 0 還是從 3 數起、哪一刻算解開——這些**全部住在那個物件裡面**。
下一個需求（階梯式加長鎖定、鎖住時通知本人、改成認來源）才會是改一個檔案，
而不是回頭重讀登入流程的每一行注解，判斷自己插在哪裡才不會破壞既有的三個順序。

---

## 2. Change Scope

### 動到的

| 元件 | 層 | 為什麼 |
| :--- | :--- | :--- |
| `entities.User` | domain / models / entities | 多兩個欄位：連續失敗次數、鎖定解除時刻。它是乾淨的資料模型，只加欄位與持久化標註 |
| `IUserRepository` | domain / interface | 多一個寫入鎖定狀態的方法；既有的 `ChangePasswordProof` **擴充語意**（同一個交易裡一併清掉鎖定） |
| `UserRepository` | infrastructure / persistence | 實作上面兩件事 |
| `UserService.SignIn` | domain / service | 插入「先問鎖定」與「記下這一次的結果」兩步 |
| `UserController.respondWithError` | controller | 多一條錯誤對映（被鎖住 → 429） |
| `ApplicationConfig` | config | 多一組門檻與鎖定期長的設定，兩者都有預設值 |
| `dependencies.go` | 組裝根 | 把設定組成政策 VO 注入 `UserService` |

### 新增的

| 元件 | 層 | 單一職責 |
| :--- | :--- | :--- |
| `domains.SignInLockoutDomain` | domain / models / domains | **這個帳號現在還能不能試，以及這一次之後它變成什麼樣子**。整個切片的業務規則只住在這裡 |
| `vo.SignInLockoutPolicyVo` | domain / models / vo | 政策：連續失敗幾次鎖、鎖多久 |
| `vo.SignInLockoutStateVo` | domain / models / vo | 一個帳號的鎖定狀態（次數 + 解除時刻），是 domain 算出來、要被寫回去的那個形狀 |
| `domains.SignInLockedError` | domain / models / domains | 被鎖住的拒絕，**帶著解除時刻**——說得出「什麼時候能再試」是這個拒絕存在的理由 |

### 刻意**不**動的

- **`SignInDomain`。** 它管的是「這一次送進來的東西成不成立」，與「這個帳號還能不能試」是兩件事。
  合進去會讓一個不需要碰儲存的模型突然需要知道一位使用者長什麼樣。
- **續用（`RenewSession`）與登出（`RevokeSession`）。** PRD 明說鎖定不影響它們。
  兩條路完全不讀鎖定狀態——**不是讀了之後放行，是根本不讀**，
  因為「讀了但決定不管」下一個人維護時會忍不住加上判斷。
- **`AccountActivationDomain` 與開通那條路。** 開通狀態與鎖定互不相干（US-05）。
- **`Session` 這個 entity。** 鎖的是「重新拿一份身分證明」，不是已經發出去的那些。
- **任何登入失敗的流水帳 / 稽核表。** 只記兩個欄位在使用者自己那一列上（PRD Out of Scope）。

---

## 3. New Components

### `vo.SignInLockoutPolicyVo`

```go
type SignInLockoutPolicyVo struct {
    FailureThreshold int           // 連續失敗幾次就鎖，預設 3
    LockoutDuration  time.Duration // 鎖多久，預設 7 天
}
```

是**設定**而不是常數，與 `AccountActivationPolicyVo`、憑證有效期限同一個理由：
測試要撥得動它，而不是為了跑一個「鎖了一週」的案例真的等一週。
兩者都有預設值、都不是鑰匙，讀不到就用預設值啟動，不因為一個設定讓整台操作台起不來。

### `vo.SignInLockoutStateVo`

```go
type SignInLockoutStateVo struct {
    FailedSignInCount int
    LockedUntil       *time.Time // nil = 沒有被鎖住
}
```

`*time.Time` 而不是零值時間：**空的代表「沒有這回事」，過去的時刻代表「曾經有過」**，
兩者要分得出來（UL-MAP 已記）。這是 domain 算完之後交給 repository 寫回去的形狀，
所以它是 VO 不是 DTO——它沒有離開 domain。

### `domains.SignInLockoutDomain`

```go
func NewSignInLockoutDomain(
    user entities.User, policy vo.SignInLockoutPolicyVo, now time.Time,
) SignInLockoutDomain

// Refusal 是這一次在還沒比對密碼之前就該得到的拒絕，沒有就是 nil。
func (d SignInLockoutDomain) Refusal() error

// AfterFailure 是密碼對不上之後，這個帳號變成什麼樣子。
func (d SignInLockoutDomain) AfterFailure() vo.SignInLockoutStateVo

// AfterSuccess 是登入成功之後，這個帳號變成什麼樣子。
func (d SignInLockoutDomain) AfterSuccess() vo.SignInLockoutStateVo
```

**三個方法，三個完整的答案，呼叫端一個判斷都不用做。**

`Refusal()` 回 `error` 而不是 `Locked() bool` + `LockedError()`：
一個問題問兩次，中間就有機會只問了前半句。回 `error` 的話，
「沒被鎖住」與「被鎖住而且這是那句話」是同一次呼叫的兩種結果，漏不掉。

**藏在裡面的複雜度**（這是它作為深模組的全部價值）：

- **哪一刻算解開**——`now` 不早於解除時刻即視為沒被鎖住（`!now.Before(*lockedUntil)`）。
  PRD 要求「到了那一刻就算解開」「差一秒不算」，兩個邊界都落在這一個比較上。
- **期滿之後從幾數起**——`AfterFailure()` 先問「現在還算被鎖住嗎」，
  不算的話**基數是 0 而不是留存的那個數字**，所以期滿後第一次失敗的結果是 1，不是 4。
  這條規則不寫在 service 裡，因為它是「連續」這個詞的定義，不是登入流程的一個步驟。
- **鎖住期間不累加、不延後**——`Refusal()` 非 nil 的那些路徑，service 根本不會走到
  `AfterFailure()`，所以「不延長」不是一條要記得遵守的規則，而是**沒有那條路可以走**。
- **到達門檻的那一次本身就被拒絕**——`AfterFailure()` 算出次數後**當場**判斷要不要訂解除時刻，
  而 service 無論如何都回拒絕。沒有「先放行再鎖」的中間狀態存在。

### `domains.SignInLockedError`

比照 `AccountNotActivatedError` 的既有作法：哨兵錯誤 `ErrSignInLocked` 給 `errors.Is`，
結構錯誤 `SignInLockedError{LockedUntil}` 給需要那個時刻的 `errors.As`，並實作 `Is`。

**解除時刻走在錯誤裡面**，理由與開通指示完全相同：
說出那句話的是 controller，而它是最不該知道鎖定規則的地方。

---

## 4. Modified Components

### `entities.User`

```go
FailedSignInCount int        `gorm:"not null;default:0"`
LockedUntil       *time.Time `gorm:"type:timestamptz"`
```

`default:0` 與開通欄位的 `default:false` 同一個用意：
**這道鎖上線之前就存在的每一列，一律從沒有歷史開始**（PRD edge case），
由欄位預設值保證，而不是由一支要記得跑的補資料程式。

### `IUserRepository`

**新增**：

```go
// SaveSignInLockoutState 寫回一次登入之後這個帳號的鎖定狀態。
SaveSignInLockoutState(executionContext context.Context, userID uint, state vo.SignInLockoutStateVo) error
```

**擴充**（不是新方法）：`ChangePasswordProof` 在**同一個交易裡**多做一件事——
把次數歸零、解除時刻清空。

這裡不開第二個方法，是延續這個介面上已經寫下的理由：
換密碼證明與作廢每一段登入階段本來就綁在一起，因為「中間那個瞬間」正是這個功能要消滅的東西。
解鎖是**同一個瞬間**的第三件事——分成兩次呼叫的話，
「密碼換好了但人還被鎖在外面」就變成一個真的存在的狀態，
而它存不存在會取決於下一個維護的人記不記得呼叫順序。寫在裡面，就沒有順序可以弄錯。

### `UserService.SignIn`

```
找人（既有）
  ↓
lockout := NewSignInLockoutDomain(user, policy, now)
if refusal := lockout.Refusal(); refusal != nil { return refusal }   ← 新增
  ↓
比對密碼（既有）
  ├─ 對不上 → SaveSignInLockoutState(lockout.AfterFailure()) → 回 ErrCredentialsRejected   ← 新增
  └─ 對得上 → SaveSignInLockoutState(lockout.AfterSuccess()) → 簽發憑證（既有）            ← 新增
```

**三個既有順序都沒有被破壞：**

1. **查無此人仍然花掉比對密碼的時間。** 找不到人時 `user` 是零值，
   `Refusal()` 對零值使用者回 nil（`LockedUntil` 是 nil），所以流程照舊往下走到比對，
   照舊回同一句話，而 `SaveSignInLockoutState` **不會被呼叫**（`userID` 為 0 即不寫）。
   US-04 的兩個情境落在這裡。
2. **鎖定檢查在比對密碼之前。** PRD 的效能要求：鎖住期間不該再花掉一次刻意很慢的比對。
3. **寫入失敗就是整次登入失敗。** `SaveSignInLockoutState` 的錯誤直接往上回，
   不吞掉——吞掉的話這道鎖會在儲存出問題的那段時間完全不存在，而沒有人會知道。

### `UserService.ChangePassword`

**一行都不改。** 解鎖發生在 `ChangePasswordProof` 裡面（見上），
所以 US-03 的兩個變更密碼情境不需要 service 多做任何事。
這正是把它放進 repository 那一個方法、而不是開第二個方法的回報。

### `UserController.respondWithError`

```go
if errors.Is(err, domains.ErrSignInLocked) {
    ginContext.JSON(http.StatusTooManyRequests, gin.H{"message": err.Error()})
    return
}
```

**429 而不是 401。** 401 在這個系統裡只有一個意思——「這次登入不算了，回去重新登入」，
而呼叫端會照做。被鎖住的人照做只會再得到同一句話。
也不是 423：那說的是「這個東西被鎖著」，而這裡真正的原因是**試太多次了**，
呼叫端該做的事是**等**，429 正是那句話。

錯誤訊息本身由 `SignInLockedError.Error()` 組出來，**含解除時刻**。

---

## 5. The Axis of Change

**最可能的下一個需求：鎖定時間階梯式加長**——第一次鎖一小時、第二次一天、第三次一週。
道理很直：一週對一個真的記錯密碼的人太重，對一台機器第一次就一週又太急。

**它會打在哪裡：`SignInLockoutDomain.AfterFailure()` 與 `SignInLockoutPolicyVo`，就這兩個。**

- 政策 VO 從「一個期長」變成「一串期長」。
- `AfterFailure()` 依「這是第幾次被鎖」挑一個。
- entity 多一個「被鎖過幾次」的欄位。
- **`UserService.SignIn` 一行都不用改**，因為它問的問題沒有變：
  「這一次要不要拒絕」「這一次之後變成什麼樣子」。
- **`UserRepository` 一行都不用改**，因為它寫的還是同一個 `SignInLockoutStateVo`
  （多一個欄位，但形狀的角色沒變）。

**次可能的需求：鎖住的那一刻通知本人。**
打在 `SignInLockoutDomain` 之外——它是 application 層看到「這一次之後是被鎖住的狀態」
所做的事。domain 算出狀態、application 決定要不要因此發訊息，
這條界線與既有的通知功能一致。

**明確**不**為它留座位的：認來源（IP / 裝置）。**
PRD 已把它排除，而且它不是這個模型的延伸——它換掉的是「什麼東西被鎖」這個主詞，
那會是一個新的模型，不是這一個多一個欄位。現在先替它挖一個空位，
只會變成一個永遠是 nil 的參數。

---

## 6. Traceability

| PRD Scenario | 由誰滿足 |
| :--- | :--- |
| 沒被鎖住的帳號用正確密碼照常登入 | `SignInLockoutDomain.Refusal()`（回 nil）+ `AfterSuccess()` |
| 第一次猜錯只是記一筆，還沒被鎖住 | `AfterFailure()`（次數 1 < 門檻 → 解除時刻仍為 nil） |
| 第三次猜錯的那一次本身就被拒絕，而且當場鎖住 | `AfterFailure()`（次數 3 = 門檻 → 訂解除時刻）+ `SignIn` 一律回拒絕 |
| 第三次猜對了就不鎖 | `AfterSuccess()`（次數歸零、解除時刻清空） |
| 鎖住期間用錯誤密碼，得到的是被鎖住的那句話 | `Refusal()` → `SignInLockedError` |
| 鎖住期間正確的密碼一樣進不來 | `Refusal()` 在比對密碼**之前**回傳 |
| 鎖住期間一直試，解除時刻不往後延、次數不累加 | `Refusal()` 非 nil 時 `SignIn` 直接回傳，**走不到**寫入 |
| 到了解除時刻那一刻就算解開 | `Refusal()` 的 `!now.Before(*lockedUntil)` |
| 差一秒到不算解開 | 同上 |
| 鎖定期滿之後的第一次失敗從 1 重新數起 | `AfterFailure()` 的「期滿即基數歸零」 |
| 變更密碼會把還沒滿的失敗次數歸零 | `UserRepository.ChangePasswordProof` |
| 變更密碼會解除鎖定 | 同上 |
| 查無此電子郵件仍然只有原本那一句 | `SignIn`：零值使用者 → `Refusal()` 回 nil → 照舊比對 → `ErrCredentialsRejected` |
| 不存在的電子郵件試幾次都不會變成被鎖住的那句話 | `SignIn`：`userID` 為 0 即不寫入，什麼都沒被記下 |
| 被鎖住的人照常續用身分證明 | `RenewSession` **不讀**鎖定狀態（刻意不動） |
| 被鎖住的人照常登出 | `RevokeSession` **不讀**鎖定狀態（刻意不動） |
| 待開通的人一樣會被鎖住 | `SignInLockoutDomain` 不讀 `IsEnabled` |

---

## 7. Risks & Trade-offs

- **讀後寫的競態。** 兩次失敗同時進來，可能都讀到 2、都寫 3，於是實際上第四次才被鎖住。
  以這台操作台的使用人數，代價是「偶爾多放過一次嘗試」，
  而修掉它要一個帶條件的原子更新，會把一條看得懂的規則換成一句寫在儲存層的計算。
  **接受，並在程式碼裡寫明為什麼接受**，免得下一個人以為是漏掉的。
- **鎖住的帳號回得比較快**（沒有花掉比對密碼的時間），這是一個時間差。
  它不洩漏任何新東西——那句拒絕本來就直接說了這個帳號被鎖住（PRD 已論證過這個取捨）。
- **`SaveSignInLockoutState` 是登入路徑上多出來的一次寫入**，每一次登入都會發生。
  以這個規模不是問題；值得記下的是它**不會多查一次使用者**——
  狀態跟著找人那一次一起取得，寫回去只用得到識別碼。
