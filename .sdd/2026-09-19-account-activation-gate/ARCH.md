# Architecture — 帳號開通把關

**Feature folder:** `.sdd/2026-09-19-account-activation-gate/`
**Designs against:** `PRD.md`（其 Gherkin 情境即本設計的契約）
**Status:** Draft

---

## 1. Design Goal

把「這個人放行了沒」變成**一個問題、一個答案**，而不是四十個 handler 各問一次。

既有的門（`AuthenticationMiddleware`）今天問的是「你是誰」。這個切片把它問的問題換成
**「給我這份憑證背後那位『已開通』的使用者」**——門本身不知道開通是什麼、不做布林判斷、
不碰設定。判斷住在 domain，門只收到兩種答案：一位使用者，或一則它認得的拒絕。

這樣做的理由不是整齊，是**下一個需求**。最可能來的下一件事是「停權」或「某幾條路待開通也能走」。
如果規則是門裡的一行 `if !user.IsEnabled`，那兩件事都要回來改門；
規則在 domain 裡，門一個字都不必動。

---

## 2. Change Scope

### 2.1 新增

| 檔案 | 型別 | 存在理由 |
| :--- | :--- | :--- |
| `internal/domain/models/vo/account_activation_policy_vo.go` | `AccountActivationPolicyVo` | 開通申請要寄去哪、主旨前綴是什麼。不可變、無行為，從設定給入 |
| `internal/domain/models/dto/account_activation_instruction_dto.go` | `AccountActivationInstructionDto` | **開通指示**交出去的形狀：信箱與**組好的完整主旨** |
| `internal/domain/models/domains/account_activation_domain.go` | `AccountActivationDomain` | 本切片**唯一**知道「待開通是什麼、主旨怎麼組、指示該不該附」的地方 |
| `internal/domain/models/domains/account_activation_errors.go` | `ErrAccountNotActivated`、`AccountNotActivatedError` | 讓門認得出這一種拒絕，並把**開通指示**一起帶到門口 |

### 2.2 修改

| 檔案 | 改動 | 對應情境 |
| :--- | :--- | :--- |
| `internal/domain/models/entities/user.go` | 加一個 `IsEnabled bool` 欄位；`ToDto()` 多帶它 | US-01、US-02 |
| `internal/domain/models/dto/user_dto.go` | 加 `IsEnabled`；加 `ActivationInstruction *AccountActivationInstructionDto`（`omitempty`） | US-01、US-02、US-04 |
| `internal/domain/service/user_service.go` | 多持有 `AccountActivationPolicyVo`；新增公開 `IdentifyActivatedUser`；`RegisterUser` 與 `IdentifyUser` 改由 `AccountActivationDomain` 產出對外形狀；抽一個**兩個公開方法共用**的私有 `identifiedUser` | 全部 |
| `internal/application/user_application.go` | 多一個 `IdentifyActivatedUser` 轉呼叫 | US-03 |
| `internal/controller/middlewares/authentication_middleware.go` | 改問 `IdentifyActivatedUser`；把「認不出你」與「認得你但沒開通」分成 401 與 403 兩種回覆 | US-03 |
| `internal/config/application_config.go` | 新增 `AccountActivationConfig` 與它的兩個環境變數 | NFR |
| `cmd/server/dependencies.go` | 把設定組成 `AccountActivationPolicyVo` 注入 `NewUserService` | — |
| `.env.example` | 記下兩個新設定 | — |
| `postman/` | 新增/更新受影響的請求 | — |

### 2.3 **刻意不動**

| 對象 | 為什麼 |
| :--- | :--- |
| `UserRepository` 與其介面 | 新欄位由 `AutoMigrate` 長出來（Code First）。**沒有**任何開通用的寫入方法——加了就等於系統提供了放行這件事，而 US-05 說它不存在 |
| `entities.User` 以外的每一個 entity | 開通是使用者身上的事，不是別人身上的事 |
| 續用（`RenewSession`）、登出（`RevokeSession`）、登入（`SignIn`） | PRD 明定這四件不受關卡影響。它們**本來就不在門後面**，所以「不動」等於「照常運作」——不必寫任何例外 |
| `GET /users/me` 的路由位置 | 它本來就不在門後面（自己讀 header），所以自動符合「待開通也問得出我是誰」。**這是既有設計替我們免費做到的** |
| 每一個功能的 service / application / controller | 它們一律在門後面，一個字都不必改就被擋住了。**這正是這道門的價值** |
| `go-trading-mcp` | 它今天就把 401 以外的失敗原封轉述給助手。403 帶著我們寫的那句話過去，助手照念。**預期零改動，實測確認** |
| 不需要登入的那些路由（`/k-candles`、`/trading-symbols`、`/watchlist`） | 那裡沒有一個「你」可以比對開通狀態。PRD 已列為 Out of Scope |

---

## 3. New Components

### 3.1 `AccountActivationPolicyVo`（vo）

```go
type AccountActivationPolicyVo struct {
    RequestMailbox string // 開通申請信箱
    SubjectPrefix  string // 開通申請主旨前綴
}
```

不可變、無行為。與 `SessionLifetimesVo` 同一類：**設定的值進到 domain 的形狀**。

### 3.2 `AccountActivationInstructionDto`（dto）

```go
type AccountActivationInstructionDto struct {
    RequestMailbox string `json:"requestMailbox"`
    Subject        string `json:"subject"`
}
```

**交出去的是組好的完整主旨，不是前綴加一個欄位讓呼叫端自己拼。**
拼接交給呼叫端就等於前端、MCP、未來任何一個客戶端各拼一次，而其中一個會拼錯——
而拼錯的主旨，管理者就是認不出來。

### 3.3 `AccountActivationDomain`（domains）

```go
func NewAccountActivationDomain(
    user entities.User, policy vo.AccountActivationPolicyVo,
) AccountActivationDomain

func (d AccountActivationDomain) Pending() bool
func (d AccountActivationDomain) ToUserDto() dto.UserDto
func (d AccountActivationDomain) NotActivatedError() error
```

- **單一職責**：一位使用者的開通處境，以及由它衍生的一切（要不要附指示、主旨長什麼樣、該回哪一種拒絕）。
- **建構方式沿用既有的路**：`domains.NewXxxDomain(entity)`（同 `NewSessionDomain`）。
  **不寫成 `user.ToActivationDomain(...)`**——`domains` 已經 import `entities`，反過來會是 import 迴圈。
- `ToUserDto()` 內部呼叫 `user.ToDto()` 再補上指示，**不重抄三個欄位**：
  「一位使用者對外長什麼樣」仍然只有 entity 一個地方知道。
- **已開通時 `ActivationInstruction` 是 `nil`**（`omitempty` 於是整個欄位消失）。
  回一個空殼或回一份已經不適用的指示，都會讓前端得自己判斷「這份指示算不算數」。

**深度檢查**：呼叫端問一次、拿到成品，不需要依序呼叫任何東西；
方法名裡沒有 And／Then；參數不隨需求增長（新的開通相關規則進建構子，不進參數列）。

### 3.4 `AccountNotActivatedError`（domains）

```go
var ErrAccountNotActivated = errors.New("帳號尚未開通，請寄信申請開通")

type AccountNotActivatedError struct {
    Instruction dto.AccountActivationInstructionDto
}

func (e AccountNotActivatedError) Error() string
func (e AccountNotActivatedError) Is(target error) bool // 對上 ErrAccountNotActivated
```

**為什麼是帶資料的錯誤、不是光禿禿的哨兵**：門必須把開通指示放進回覆，
而門沒有、也不該有那份設定。讓錯誤自己把指示帶到門口，是唯一不讓設定漏進 controller 的走法。
`Is` 讓既有的 `errors.Is` 寫法照常成立，呼叫端要拿資料才用 `errors.As`。

---

## 4. Modified Components

### 4.1 `entities.User`

多一個欄位：

```go
IsEnabled bool `gorm:"not null;default:false"`
```

- **預設 `false` 由兩層各說一次**：Go 的零值，與資料庫的 `default:false`。
  前者保證 `UserRegistrationDomain.ToEntity` **沒有地方可以指定它**（US-01 的「無從指定」
  因此是型別做到的，不是有人記得檢查）；後者保證這道關卡上線時，
  既有的每一列都落在待開通（US-01 最後一個情境）。
- `ToDto()` 多帶 `IsEnabled`。它仍然是純形狀轉換，不需要設定，所以留在 entity 上是對的。

### 4.2 `UserService`

新增第四個公開 use-case 方法：

```go
func (s *UserService) IdentifyActivatedUser(
    ctx context.Context, accessToken string,
) (dto.UserDto, error)
```

- **它與 `IdentifyUser` 互不呼叫**（規則：同一個 service 的公開方法互不呼叫）。
  兩者共用一個私有 `identifiedUser(ctx, token) (entities.User, error)`——
  **恰好兩個公開方法在用，過得了「只有一個呼叫者就 inline」那道門檻。**
- 拒絕的順序是**先認人、後看開通**。倒過來的話，一份壞掉的憑證會得到「帳號尚未開通」，
  而那句話對一個根本沒被認出來的人毫無意義。
- `RegisterUser` 與 `IdentifyUser` 的回傳改成
  `domains.NewAccountActivationDomain(user, s.activationPolicy).ToUserDto()`。
  **對外交出使用者的地方只有這三處，且三處走同一條路**，所以「忘了附指示」不會零星發生。

### 4.3 `AuthenticationMiddleware.Handle`

```go
userDto, err := m.userApplication.IdentifyActivatedUser(ctx, accessTokenOf(c))

var notActivated domains.AccountNotActivatedError
if errors.As(err, &notActivated) {
    c.AbortWithStatusJSON(403, gin.H{
        "message":               notActivated.Error(),
        "activationInstruction": notActivated.Instruction,
    })
    return
}
if err != nil {
    c.AbortWithStatusJSON(401, gin.H{"message": domains.ErrAuthenticationRequired.Error()})
    return
}
```

- **403 而不是 401**，而且這個選擇與既有的 `ErrCurrentPasswordRejected` 用 403 是同一個道理：
  在這個系統裡 **401 只有一個意思——「你的登入不算數了，回去重登」**，
  呼叫端看到它就會把人丟回登入畫面。這個人的登入好好的，把他丟回登入畫面他照做也不會有任何改變。
- **門放進 `currentUserKey` 的仍然只有識別碼**，不放開通狀態——
  能走到 handler 的人一定已經開通，handler 沒有第二種情況要分。

### 4.4 `AccountActivationConfig`

```go
type AccountActivationConfig struct {
    RequestMailbox string // ACCOUNT_ACTIVATION_REQUEST_MAILBOX
    SubjectPrefix  string // ACCOUNT_ACTIVATION_SUBJECT_PREFIX
}
```

預設值：`james.afternoon.dev@gmail.com` 與 `go-trading 開通申請`。

**這兩個有預設值，而簽章鑰匙與封存鑰匙沒有**——差別在於它們不是鑰匙。
一個猜得到的信箱不會讓任何東西失守；缺了它而讓整個系統起不來才是真的壞事。

---

## 5. The Axis of Change

**最可能的下一個需求，以及它會打在哪裡：**

| 下一個需求 | 會改到哪 | 為什麼只有那裡 |
| :--- | :--- | :--- |
| **停權**（已開通 → 停權） | `AccountActivationDomain` 與 `IsEnabled` 換成狀態值 | 門問的問題沒變（「給我已開通的那位」），四十個 handler 也沒變 |
| **待開通也能設定 Telegram** | 那幾條路由改掛一個只認人的門 | 兩道門是兩個公開方法，本來就都在；不必在任何一個 handler 裡寫例外 |
| **系統代寄申請信** | 新增一個 `IEmailProxy` 與它的實作，由 `RegisterUser` 呼叫 | 開通指示已經是一份組好的 DTO，寄信的人直接拿去用，不必再拼一次主旨 |
| **後台放行操作** | 新增一個 repository 寫入方法與一條路由 | 今天**刻意沒有**寫入方法，所以「誰都放行不了」是缺了那個方法造成的，不是有人記得檢查 |

**seam 就是 `IdentifyActivatedUser` 這個問題本身。**
只要每一條需要認人的路都是問它，換掉「什麼叫可以用」永遠是換 domain 裡的一個答案。

---

## 6. Traceability

| PRD 情境 | 由誰滿足 |
| :--- | :--- |
| US-01 新建立一律待開通 | `entities.User.IsEnabled` 零值 + `UserRegistrationDomain.ToEntity` 沒有那個欄位 |
| US-01 建立當場拿到開通指示 | `UserService.RegisterUser` → `AccountActivationDomain.ToUserDto()` |
| US-01 主旨用留存下來的電子郵件 | `AccountActivationDomain` 讀的是 entity 上已正規化的 `Email` |
| US-01 指定不了已開通 | `entities.User` 的欄位沒有出現在 `UserRegistrationDto` 或 `ToEntity` 任何一處 |
| US-01 既有使用者也是待開通 | GORM `default:false` 欄位預設 |
| US-02 登入不受影響 | `SignIn` **不動** |
| US-02 問得出自己是誰與開通狀態 | `UserService.IdentifyUser` → `AccountActivationDomain.ToUserDto()` |
| US-02 續用與登出照常 | `RenewSession`／`RevokeSession` **不動**，且不在門後面 |
| US-03 待開通被擋、附指示 | `IdentifyActivatedUser` → `AccountNotActivatedError` → 門回 403 |
| US-03 不會被誤認成「密碼不正確」 | 門在 handler 之前就中止，`ChangePassword` 根本沒被呼叫 |
| US-03 與「請先登入」是兩句話 | 門的兩個分支：403 帶開通指示／401 帶 `ErrAuthenticationRequired` |
| US-03 呼叫端分辨得出來 | 兩個不同的狀態碼（403／401），不必讀說明文字 |
| US-04 放行後不必重登 | `identifiedUser` 每次都重讀 entity；開通狀態不寫進登入憑證 |
| US-04 放行後不再附指示 | `AccountActivationDomain.ToUserDto()` 在已開通時回 `nil` 指示 |
| US-05 放行不了自己或別人 | repository **沒有**任何寫入開通狀態的方法，也沒有任何路由 |

---

## 7. Testing Plan

依 `.claude/rules/testing.md`：只測業務行為、只 mock 介面。

| 層 | 測什麼 |
| :--- | :--- |
| `AccountActivationDomain`（domain model 單元測試） | 待開通／已開通兩種下的 `Pending()`、主旨組法、指示的有無 |
| `UserService`（注入 mock repository／proxy） | `IdentifyActivatedUser` 的三種結局：認不出人（401 那一種）、認得但待開通、認得且已開通；`RegisterUser` 一律待開通 |
| `AuthenticationMiddleware` | 403 與 401 的分流，以及 403 的回覆裡確實帶著開通指示 |
| Application 測試 | 沿用既有力度放大：注入真實 service，只 mock 最外層 |
| `config` | 兩個新設定的預設值與覆寫 |

既有測試中凡是「已登入就能做某件事」的，其固定資料需改成**已開通**的使用者——
這是這個切片會擴散到最多既有測試的一處，預期即此，不是設計出了問題。

---

## 8. Rollout Note（必須寫進交付說明）

這道門上線的那一刻，**現有的每一位使用者都被關在門外，包含這台操作台的主人**。
部署後的第一件事是把自己打開：

```
UPDATE "Users" SET is_enabled = true WHERE email = '<你的電子郵件>';
```

資料庫裡另有十餘筆測試留下的帳號，**刻意不一併打開**。
