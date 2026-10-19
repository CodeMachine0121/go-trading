package domains

import "errors"

var ErrSymbolScreeningValidation = errors.New("篩選的要求不正確")

var ErrSymbolScreeningProfileNotFound = errors.New("找不到這份篩選設定")

var ErrSymbolScreeningProfileNameTaken = errors.New("已經有同名的篩選設定")

var ErrSymbolScreeningProfileInUse = errors.New("有篩選正在使用這份設定，等它結束再刪除")

var ErrSymbolScreeningRunNotFound = errors.New("找不到這筆篩選")

var ErrSymbolScreeningRunAlreadyActive = errors.New("你已經有一筆篩選在排隊或進行中，等它結束再送")

var ErrSymbolScreeningSymbolNotTrading = errors.New("目前沒有在交易")

var ErrSymbolScreeningRunAlreadyEnded = errors.New("這筆篩選已經結束，不能取消")

// ErrSymbolScreeningBrokeDown stands for a run that panicked, so it ends without exposing the panic.
var ErrSymbolScreeningBrokeDown = errors.New("symbol screening broke down")
