package vo

// ContractOrderFailureVo is how the venue answered one contract trading call; the empty value means it did what was asked.
type ContractOrderFailureVo string

const (
	ContractOrderFailureNone ContractOrderFailureVo = ""
	// ContractOrderFailureKeyRejected may also mean the key lacks contract trading; the caller asks again to tell the two apart.
	ContractOrderFailureKeyRejected           ContractOrderFailureVo = "keyRejected"
	ContractOrderFailureNoContractPermission  ContractOrderFailureVo = "noContractPermission"
	ContractOrderFailureInsufficientBalance   ContractOrderFailureVo = "insufficientBalance"
	ContractOrderFailureVenueRefused          ContractOrderFailureVo = "venueRefused"
	ContractOrderFailureAccountSettingRefused ContractOrderFailureVo = "accountSettingRefused"
	// ContractOrderFailureOtherRefusal is a refusal the venue gave in words this system does not recognise.
	ContractOrderFailureOtherRefusal ContractOrderFailureVo = "otherRefusal"
	// ContractOrderFailureNotFound answers a lookup for an order the venue has no record of.
	ContractOrderFailureNotFound ContractOrderFailureVo = "notFound"
	// ContractOrderFailureUncertain means the request may have reached the venue; whatever it asked for may have happened.
	ContractOrderFailureUncertain ContractOrderFailureVo = "uncertain"
)
