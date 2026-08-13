package i18n

// Language 定义支持的语言类型
type Language string

const (
	EN Language = "en" // English
	ZH Language = "zh" // Chinese
)

// 错误/消息代码
const (
	OK                                     = 200
	ParamError                             = 400
	Unauthorized                           = 401
	Forbidden                              = 403
	NotFound                               = 404
	ConflictError                          = 409
	InternalServerError                    = 500
	ServiceUnavailable                     = 503
	OperationNotAllowed                    = 1001
	ResourceAlreadyExists                  = 1002
	InvalidRequest                         = 1003
	DataValidationError                    = 1004
	AuthenticationRequired                 = 1005
	UserNotFound                           = 1006
	InvalidCredentials                     = 1007
	TokenExpired                           = 1008
	PermissionDenied                       = 1009
	BusinessDataNotFound                   = 2001
	ApiURLRequired                         = 2002
	ApiTokenRequired                       = 2003
	SyncTaskAlreadyRunning                 = 2004
	DistributedLockAcquireFailed           = 2005
	SyncTaskCreateFailed                   = 2006
	CategoryNotFound                       = 2007
	ContractNotFound                       = 2008
	ContractNotTradable                    = 2009
	PriceFormatError                       = 2010
	QuantityFormatError                    = 2011
	ClientOrderIDAlreadyExists             = 2012
	PositionNotFound                       = 2013
	NoPermissionViewPosition               = 2014
	ContractPositionMismatch               = 2015
	ExerciseQuantityFormatError            = 2016
	ExercisableQuantityExceeded            = 2017
	OrderNotFound                          = 2018
	NoPermissionViewOrder                  = 2019
	NoPermissionOperateOrder               = 2020
	CurrentStatusCannotCancel              = 2021
	BillNotFound                           = 2022
	ContractCodeAlreadyExists              = 2023
	StrikePriceFormatError                 = 2024
	ContractUnitFormatError                = 2025
	MinOrderQuantityFormatError            = 2026
	MaxOrderQuantityFormatError            = 2027
	PriceTickFormatError                   = 2028
	QuantityStepFormatError                = 2029
	MultiplierFormatError                  = 2030
	MarketNotFound                         = 2031
	SettlementRecordNotFound               = 2032
	ExerciseRecordNotFound                 = 2033
	TradeNotFound                          = 2034
	AccountAssetNotFound                   = 2035
	UnderlyingPriceFormatError             = 2036
	MarkPriceFormatError                   = 2037
	LastPriceFormatError                   = 2038
	BidPriceFormatError                    = 2039
	AskPriceFormatError                    = 2040
	TheoreticalPriceFormatError            = 2041
	IntrinsicValueFormatError              = 2042
	TimeValueFormatError                   = 2043
	IVFormatError                          = 2044
	DeltaFormatError                       = 2045
	GammaFormatError                       = 2046
	ThetaFormatError                       = 2047
	VegaFormatError                        = 2048
	RhoFormatError                         = 2049
	RiskFreeRateFormatError                = 2050
	UserDisabled                           = 2051
	RoleNotFound                           = 2052
	ConfigValidationFailed                 = 2053
	InvalidQueryCondition                  = 2054
	RoleCodeAlreadyExists                  = 2055
	MenuNotFound                           = 2056
	Google2FACodeRequired                  = 2057
	Google2FACodeInvalid                   = 2058
	PasswordIncorrect                      = 2059
	CronJobNotFound                        = 2060
	ConfigAlreadyExists                    = 2061
	TenantCodeAlreadyExists                = 2062
	TenantNotFound                         = 2063
	TenantExpired                          = 2064
	TenantDisabled                         = 2065
	InviteCodeRequired                     = 2066
	InviterNotFound                        = 2067
	RegistrationTooFrequent                = 2068
	UserAlreadyExists                      = 2069
	NotifyRecordNotFound                   = 2070
	OnlyPaidOrdersCanRetryNotify           = 2071
	OnlyPendingPaymentOrdersCanMarkSuccess = 2072
	PaymentChannelNotFound                 = 2073
	PaymentChannelUnavailable              = 2074
	RechargeAmountOutOfLimit               = 2075
	ProductNotFound                        = 2076
	TenantPayAccountNotFound               = 2077
	PlatformNotFound                       = 2078
	OnlyPendingPaymentOrdersCanCancel      = 2080
	NoPermissionCancelOrder                = 2081
	OnlyUnpaidOrdersCanClose               = 2082
	ChannelRuleNotFound                    = 2083
	ChannelNotFound                        = 2084
	OnlyPendingReviewOrdersCanAudit        = 2085
	NoPermissionAccessOrder                = 2086
	NotifyLogNotFound                      = 2087
	RechargeStatNotFound                   = 2088
	NotifyChannelRuleNotFound              = 2089
	BankCardNotFound                       = 2090
	NoPermissionModify                     = 2091
	UserSecurityInfoNotFound               = 2092
	PasswordsDoNotMatch                    = 2093
	TokenExpiredOrInvalid                  = 2094
	AccountDisabled                        = 2095
	PleaseInitializeGoogle2FA              = 2096
	VerificationCodeInvalid                = 2097
	PleaseSwitchDeviceToLogin              = 2098
	SecretGenerationFailed                 = 2099
	PermissionDeniedForBankCard            = 2100
	NoPermissionOperateThisUser            = 2101
	NoPermissionModifyThisBankCard         = 2102
	NoPermissionDeleteThisBankCard         = 2103
	NoPermissionOperateThisBankCard        = 2104
	SecuritySettingsNotFound               = 2105
	UserIdentityInfoNotFound               = 2106
	PayPasswordNotSet                      = 2107
	InvalidMenuType                        = 2108
	MenuAlreadyExists                      = 2109
	MenuNotFoundEN                         = 2110
	Google2FANotInitialized                = 2111
	UserStatusUnchanged                    = 2112
	InvalidCronExpression                  = 2113
	CronJobAlreadyExists                   = 2114
	UserIDInvalid                          = 2115
	RoleSelectionRequired                  = 2116
	ValidRoleSelectionRequired             = 2117
	SomeRolesNotFound                      = 2118
	UsernameAlreadyExists                  = 2119
	UserDisabledForLogin                   = 2120
	AssetNotFound                          = 2121
	NoPermissionQueryOrder                 = 2122
	NoPermissionOperatePosition            = 2123
	RegistrationTooFrequentRetry           = 2124
	Google2FASecretFetchFailed             = 2125
	Google2FASecretExpired                 = 2126
	Generate2FASecretFailed                = 2127
	Store2FASecretFailed                   = 2128
	APIURLIsRequired                       = 2129
	TokenRequired                          = 2130
	CategoryRequired                       = 2131
	MarketRequired                         = 2132
	SymbolRequired                         = 2133
	UserNotFoundOrPasswordIncorrect        = 2134
	StakingProductUnavailable              = 2135
	StakeAmountInvalid                     = 2136
	StakeAmountBelowMinimum                = 2137
	StakeAmountAboveMaximum                = 2138
	StakeAmountStepInvalid                 = 2139
	ProductQuotaInsufficient               = 2140
	UserStakeLimitExceeded                 = 2141
	ProductNoAlreadyExists                 = 2142
	StakingOrderCannotRedeem               = 2143
	EarlyRedeemNotAllowed                  = 2144
	RedeemAmountInvalid                    = 2145
	RewardAmountInvalid                    = 2146
	SuperAdminCannotBeDeleted              = 2147
	TenantOwnerCannotBeDeleted             = 2148
	CryptoRechargeAddressNotConfigured     = 2149
	CryptoRechargeAddressInUse             = 2150
	CryptoRechargeAddressExpired           = 2151
	AmountMustBePositive                   = 2152
	InsufficientAvailableBalance           = 2153
	TransferCoinRequired                   = 2154
	WalletTypeRequired                     = 2155
	SameWalletCoinTransferNotNeeded        = 2156
	InvalidExchangeRate                    = 2157
	PostOnlyOrderWouldMatchImmediately     = 2158
	RiskTradeDisabled                      = 2159
	RiskOpenDisabled                       = 2160
	RiskOrderQuantityBelowMinimum          = 2161
	RiskOrderQuantityExceedsMaximum        = 2162
	RiskOrderAmountBelowMinimum            = 2163
	RiskOrderAmountExceedsMaximum          = 2164
	AssetRequestProcessing                 = 2165
	FreezeRecordNotFound                   = 2166
	LockRecordNotFound                     = 2167
	AssetTenantMismatch                    = 2168
	FreezeRecordNotReleasable              = 2169
	UnfreezeAmountExceedsFrozen            = 2170
	AssetUnfreezeFailed                    = 2171
	FreezeRecordUpdateFailed               = 2172
	AmountMustNotBeNegative                = 2173
	FreezeRecordNotDeductible              = 2174
	DeductAmountExceedsFrozen              = 2175
	DeductFrozenFailed                     = 2176
	FreezeRecordDeductUpdateFailed         = 2177
	DeductAmountExceedsLocked              = 2178
	DeductLockedFailed                     = 2179
	LockRecordUpdateFailed                 = 2180
	UnlockAmountExceedsLocked              = 2181
	AssetUnlockFailed                      = 2182
	EuropeanOptionNotExpired               = 2183
	OptionNotInTheMoney                    = 2184
	RequestRequired                        = 2185
	UnderlyingPriceMustBePositive          = 2186
	CryptoWalletAccountNotFound            = 2187
	CryptoRechargeTxNotFound               = 2188
	CryptoRechargeAddressNotFound          = 2189
	InvalidCryptoRechargeParams            = 2190
	TenantOwnerRoleTemplateNotFound        = 2191
	SyncTaskFailed                         = 2192
	AssetCoinConfigNotFound                = 2193
	SuperAdminUpdateForbidden              = 2194
	SuperAdminDeleteForbidden              = 2195
	PayProductCodeAlreadyExists            = 2196
	PayPlatformCodeAlreadyExists           = 2197
	TenantPayAccountCodeAlreadyExists      = 2198
	TenantPayChannelCodeAlreadyExists      = 2199
	PaymentRequiredParamsMissing           = 2200
	InvalidPaymentJSON                     = 2201
	InvalidPaymentAmountRange              = 2202
	PaymentRelationMismatch                = 2203
	CryptoResourceAlreadyExists            = 2204
	InvalidPaymentDecimal                  = 2205
)

// MessageMap 定义所有支持的错误消息翻译
var MessageMap = map[int32]map[Language]string{
	OK: {
		EN: "OK",
		ZH: "成功",
	},
	ParamError: {
		EN: "Parameter error",
		ZH: "参数错误",
	},
	Unauthorized: {
		EN: "Unauthorized",
		ZH: "未授权",
	},
	Forbidden: {
		EN: "Forbidden",
		ZH: "禁止访问",
	},
	NotFound: {
		EN: "Not found",
		ZH: "资源不存在",
	},
	ConflictError: {
		EN: "Conflict",
		ZH: "冲突",
	},
	InternalServerError: {
		EN: "Internal server error",
		ZH: "服务器内部错误",
	},
	ServiceUnavailable: {
		EN: "Service unavailable",
		ZH: "服务不可用",
	},
	OperationNotAllowed: {
		EN: "Operation not allowed",
		ZH: "操作不被允许",
	},
	ResourceAlreadyExists: {
		EN: "Resource already exists",
		ZH: "资源已存在",
	},
	InvalidRequest: {
		EN: "Invalid request",
		ZH: "无效请求",
	},
	DataValidationError: {
		EN: "Data validation error",
		ZH: "数据验证错误",
	},
	AuthenticationRequired: {
		EN: "Authentication required",
		ZH: "需要验证",
	},
	UserNotFound: {
		EN: "User not found",
		ZH: "用户不存在",
	},
	InvalidCredentials: {
		EN: "Invalid credentials",
		ZH: "凭证无效",
	},
	TokenExpired: {
		EN: "Token expired",
		ZH: "令牌已过期",
	},
	PermissionDenied: {
		EN: "Permission denied",
		ZH: "权限被拒绝",
	},
	BusinessDataNotFound: {
		EN: "Data not found",
		ZH: "数据不存在",
	},
	ApiURLRequired: {
		EN: "ApiUrl is required",
		ZH: "ApiUrl 不能为空",
	},
	ApiTokenRequired: {
		EN: "ApiToken is required",
		ZH: "ApiToken 不能为空",
	},
	SyncTaskAlreadyRunning: {
		EN: "A sync task is already running, please try again later",
		ZH: "已有同步任务正在执行，请稍后再试",
	},
	DistributedLockAcquireFailed: {
		EN: "Failed to acquire distributed lock",
		ZH: "获取分布式锁失败",
	},
	SyncTaskCreateFailed: {
		EN: "Failed to create sync task",
		ZH: "创建同步任务失败",
	},
	CategoryNotFound: {
		EN: "Category not found",
		ZH: "分类不存在",
	},
	ContractNotFound: {
		EN: "Contract not found",
		ZH: "合约不存在",
	},
	ContractNotTradable: {
		EN: "Contract is not tradable at the moment",
		ZH: "合约当前不可交易",
	},
	PriceFormatError: {
		EN: "Invalid price format",
		ZH: "price格式错误",
	},
	QuantityFormatError: {
		EN: "Invalid quantity format",
		ZH: "qty格式错误",
	},
	ClientOrderIDAlreadyExists: {
		EN: "Client order ID already exists",
		ZH: "client_order_id已存在",
	},
	PositionNotFound: {
		EN: "Position not found",
		ZH: "持仓不存在",
	},
	NoPermissionViewPosition: {
		EN: "No permission to view this position",
		ZH: "无权查看该持仓",
	},
	ContractPositionMismatch: {
		EN: "contract_id does not match the position",
		ZH: "contract_id与持仓不匹配",
	},
	ExerciseQuantityFormatError: {
		EN: "Invalid exercise quantity format",
		ZH: "exercise_qty格式错误",
	},
	ExercisableQuantityExceeded: {
		EN: "Exceeded exercisable quantity",
		ZH: "超过可行权数量",
	},
	OrderNotFound: {
		EN: "Order not found",
		ZH: "订单不存在",
	},
	NoPermissionViewOrder: {
		EN: "No permission to view this order",
		ZH: "无权查看该订单",
	},
	NoPermissionOperateOrder: {
		EN: "No permission to operate this order",
		ZH: "无权操作该订单",
	},
	CurrentStatusCannotCancel: {
		EN: "Current status does not allow cancellation",
		ZH: "当前状态不可撤单",
	},
	BillNotFound: {
		EN: "Bill record not found",
		ZH: "资金流水不存在",
	},
	ContractCodeAlreadyExists: {
		EN: "Contract code already exists",
		ZH: "合约编码已存在",
	},
	StrikePriceFormatError: {
		EN: "Invalid strike_price format",
		ZH: "strike_price格式错误",
	},
	ContractUnitFormatError: {
		EN: "Invalid contract_unit format",
		ZH: "contract_unit格式错误",
	},
	MinOrderQuantityFormatError: {
		EN: "Invalid min_order_qty format",
		ZH: "min_order_qty格式错误",
	},
	MaxOrderQuantityFormatError: {
		EN: "Invalid max_order_qty format",
		ZH: "max_order_qty格式错误",
	},
	PriceTickFormatError: {
		EN: "Invalid price_tick format",
		ZH: "price_tick格式错误",
	},
	QuantityStepFormatError: {
		EN: "Invalid qty_step format",
		ZH: "qty_step格式错误",
	},
	MultiplierFormatError: {
		EN: "Invalid multiplier format",
		ZH: "multiplier格式错误",
	},
	MarketNotFound: {
		EN: "Market data not found",
		ZH: "行情不存在",
	},
	SettlementRecordNotFound: {
		EN: "Settlement record not found",
		ZH: "结算记录不存在",
	},
	ExerciseRecordNotFound: {
		EN: "Exercise record not found",
		ZH: "行权记录不存在",
	},
	TradeNotFound: {
		EN: "Trade record not found",
		ZH: "成交不存在",
	},
	AccountAssetNotFound: {
		EN: "Account asset not found",
		ZH: "账户资产不存在",
	},
	UnderlyingPriceFormatError: {
		EN: "Invalid underlying_price format",
		ZH: "underlying_price格式错误",
	},
	MarkPriceFormatError: {
		EN: "Invalid mark_price format",
		ZH: "mark_price格式错误",
	},
	LastPriceFormatError: {
		EN: "Invalid last_price format",
		ZH: "last_price格式错误",
	},
	BidPriceFormatError: {
		EN: "Invalid bid_price format",
		ZH: "bid_price格式错误",
	},
	AskPriceFormatError: {
		EN: "Invalid ask_price format",
		ZH: "ask_price格式错误",
	},
	TheoreticalPriceFormatError: {
		EN: "Invalid theoretical_price format",
		ZH: "theoretical_price格式错误",
	},
	IntrinsicValueFormatError: {
		EN: "Invalid intrinsic_value format",
		ZH: "intrinsic_value格式错误",
	},
	TimeValueFormatError: {
		EN: "Invalid time_value format",
		ZH: "time_value格式错误",
	},
	IVFormatError: {
		EN: "Invalid iv format",
		ZH: "iv格式错误",
	},
	DeltaFormatError: {
		EN: "Invalid delta format",
		ZH: "delta格式错误",
	},
	GammaFormatError: {
		EN: "Invalid gamma format",
		ZH: "gamma格式错误",
	},
	ThetaFormatError: {
		EN: "Invalid theta format",
		ZH: "theta格式错误",
	},
	VegaFormatError: {
		EN: "Invalid vega format",
		ZH: "vega格式错误",
	},
	RhoFormatError: {
		EN: "Invalid rho format",
		ZH: "rho格式错误",
	},
	RiskFreeRateFormatError: {
		EN: "Invalid risk_free_rate format",
		ZH: "risk_free_rate格式错误",
	},
	UserDisabled: {
		EN: "User has been disabled",
		ZH: "用户已禁用",
	},
	RoleNotFound: {
		EN: "Role not found",
		ZH: "角色不存在",
	},
	ConfigValidationFailed: {
		EN: "Configuration validation failed",
		ZH: "配置项校验失败",
	},
	InvalidQueryCondition: {
		EN: "Invalid query condition",
		ZH: "无效的查询条件",
	},
	RoleCodeAlreadyExists: {
		EN: "Role code already exists",
		ZH: "角色编码已存在",
	},
	MenuNotFound: {
		EN: "Menu not found",
		ZH: "菜单不存在",
	},
	Google2FACodeRequired: {
		EN: "Please enter the Google 2FA verification code",
		ZH: "请输入 Google 2FA 验证码",
	},
	Google2FACodeInvalid: {
		EN: "Invalid Google 2FA verification code",
		ZH: "Google 2FA 验证码错误",
	},
	PasswordIncorrect: {
		EN: "Incorrect password",
		ZH: "密码错误",
	},
	CronJobNotFound: {
		EN: "Cron job not found",
		ZH: "定时任务不存在",
	},
	ConfigAlreadyExists: {
		EN: "Configuration item already exists",
		ZH: "配置项已存在",
	},
	TenantCodeAlreadyExists: {
		EN: "Tenant code already exists",
		ZH: "租户编码已存在",
	},
	TenantNotFound: {
		EN: "Tenant not found",
		ZH: "租户不存在",
	},
	TenantExpired: {
		EN: "Tenant has expired",
		ZH: "租户已过期",
	},
	TenantDisabled: {
		EN: "Tenant has been disabled",
		ZH: "租户被禁用",
	},
	InviteCodeRequired: {
		EN: "Invitation code is required",
		ZH: "邀请码不能为空",
	},
	InviterNotFound: {
		EN: "Inviting user not found",
		ZH: "邀请用户不存在",
	},
	RegistrationTooFrequent: {
		EN: "Registration is too frequent",
		ZH: "不能频繁注册",
	},
	UserAlreadyExists: {
		EN: "User already exists",
		ZH: "用户已存在",
	},
	NotifyRecordNotFound: {
		EN: "Notification record not found",
		ZH: "回调记录不存在",
	},
	OnlyPaidOrdersCanRetryNotify: {
		EN: "Only paid orders can retry notification",
		ZH: "只有已支付订单才能重试回调",
	},
	OnlyPendingPaymentOrdersCanMarkSuccess: {
		EN: "Only pending payment orders can be marked as successful",
		ZH: "只有待支付订单才能标记为成功",
	},
	PaymentChannelNotFound: {
		EN: "Payment channel not found",
		ZH: "支付通道不存在",
	},
	PaymentChannelUnavailable: {
		EN: "Payment channel is temporarily unavailable",
		ZH: "支付通道暂不可用",
	},
	RechargeAmountOutOfLimit: {
		EN: "Recharge amount exceeds the limit",
		ZH: "充值金额超出限制",
	},
	ProductNotFound: {
		EN: "Product not found",
		ZH: "产品不存在",
	},
	TenantPayAccountNotFound: {
		EN: "Account not found",
		ZH: "账户不存在",
	},
	PlatformNotFound: {
		EN: "Platform not found",
		ZH: "平台不存在",
	},
	OnlyPendingPaymentOrdersCanCancel: {
		EN: "Only pending payment orders can be canceled",
		ZH: "只能取消待支付的订单",
	},
	NoPermissionCancelOrder: {
		EN: "No permission to cancel this order",
		ZH: "无权取消该订单",
	},
	OnlyUnpaidOrdersCanClose: {
		EN: "Only unpaid orders can be closed",
		ZH: "只有未付款订单才能关闭",
	},
	ChannelRuleNotFound: {
		EN: "Rule not found",
		ZH: "规则不存在",
	},
	ChannelNotFound: {
		EN: "Channel not found",
		ZH: "通道不存在",
	},
	OnlyPendingReviewOrdersCanAudit: {
		EN: "Only pending review orders can be audited",
		ZH: "只有待审核订单才能审核",
	},
	NoPermissionAccessOrder: {
		EN: "No permission to access this order",
		ZH: "无权访问该订单",
	},
	NotifyLogNotFound: {
		EN: "Notification log not found",
		ZH: "回调日志不存在",
	},
	RechargeStatNotFound: {
		EN: "User recharge statistics not found",
		ZH: "用户充值统计不存在",
	},
	NotifyChannelRuleNotFound: {
		EN: "Channel rule not found",
		ZH: "通道规则不存在",
	},
	BankCardNotFound: {
		EN: "Bank card not found",
		ZH: "银行卡不存在",
	},
	NoPermissionModify: {
		EN: "No permission to modify",
		ZH: "无权限修改",
	},
	CryptoRechargeAddressNotConfigured: {
		EN: "Recharge address is not configured for this coin",
		ZH: "当前币种暂未配置充值地址",
	},
	CryptoRechargeAddressInUse: {
		EN: "Recharge address is in use, please try again later",
		ZH: "充值地址正在使用中，请稍后再试",
	},
	CryptoRechargeAddressExpired: {
		EN: "Recharge address has expired, please get a new address",
		ZH: "充值地址已过期，请重新获取地址",
	},
	AmountMustBePositive: {
		EN: "Amount must be greater than 0",
		ZH: "金额必须大于 0",
	},
	InsufficientAvailableBalance: {
		EN: "Insufficient available balance",
		ZH: "可用余额不足",
	},
	TransferCoinRequired: {
		EN: "Select transfer coins",
		ZH: "请选择划转币种",
	},
	WalletTypeRequired: {
		EN: "Select account type",
		ZH: "请选择账户类型",
	},
	SameWalletCoinTransferNotNeeded: {
		EN: "Same account and coin do not need a transfer",
		ZH: "同一账户同一币种无需划转",
	},
	InvalidExchangeRate: {
		EN: "Invalid exchange rate",
		ZH: "汇率异常",
	},
	PostOnlyOrderWouldMatchImmediately: {
		EN: "Post Only order would match immediately",
		ZH: "Post Only 订单会立即成交",
	},
	RiskTradeDisabled: {
		EN: "Trading is disabled",
		ZH: "交易已被限制",
	},
	RiskOpenDisabled: {
		EN: "Opening positions is disabled",
		ZH: "开仓已被限制",
	},
	RiskOrderQuantityBelowMinimum: {
		EN: "Order quantity is below the minimum",
		ZH: "下单数量低于最小限制",
	},
	RiskOrderQuantityExceedsMaximum: {
		EN: "Order quantity exceeds the maximum",
		ZH: "下单数量超过最大限制",
	},
	RiskOrderAmountBelowMinimum: {
		EN: "Order amount is below the minimum",
		ZH: "下单金额低于最小限制",
	},
	RiskOrderAmountExceedsMaximum: {
		EN: "Order amount exceeds the maximum",
		ZH: "下单金额超过最大限制",
	},
	AssetRequestProcessing: {
		EN: "Asset request is processing",
		ZH: "资产请求处理中",
	},
	FreezeRecordNotFound: {
		EN: "Freeze record not found",
		ZH: "冻结记录不存在",
	},
	LockRecordNotFound: {
		EN: "Lock record not found",
		ZH: "锁仓记录不存在",
	},
	AssetTenantMismatch: {
		EN: "Asset record tenant mismatch",
		ZH: "资产记录租户不匹配",
	},
	FreezeRecordNotReleasable: {
		EN: "Freeze record is not releasable",
		ZH: "冻结记录不可解冻",
	},
	UnfreezeAmountExceedsFrozen: {
		EN: "Unfreeze amount exceeds remaining frozen amount",
		ZH: "解冻金额超过剩余冻结金额",
	},
	AssetUnfreezeFailed: {
		EN: "Unfreeze failed",
		ZH: "解冻失败",
	},
	FreezeRecordUpdateFailed: {
		EN: "Freeze record update failed",
		ZH: "冻结记录更新失败",
	},
	AmountMustNotBeNegative: {
		EN: "Amount must not be negative",
		ZH: "金额不能小于 0",
	},
	FreezeRecordNotDeductible: {
		EN: "Freeze record is not deductible",
		ZH: "冻结记录不可扣减",
	},
	DeductAmountExceedsFrozen: {
		EN: "Deduct amount exceeds remaining frozen amount",
		ZH: "扣减金额超过剩余冻结金额",
	},
	DeductFrozenFailed: {
		EN: "Deduct frozen balance failed",
		ZH: "扣减冻结余额失败",
	},
	FreezeRecordDeductUpdateFailed: {
		EN: "Freeze record deduct update failed",
		ZH: "冻结扣减记录更新失败",
	},
	DeductAmountExceedsLocked: {
		EN: "Deduct amount exceeds locked amount",
		ZH: "扣减金额超过锁仓金额",
	},
	DeductLockedFailed: {
		EN: "Deduct locked balance failed",
		ZH: "扣减锁仓余额失败",
	},
	LockRecordUpdateFailed: {
		EN: "Lock record update failed",
		ZH: "锁仓记录更新失败",
	},
	UnlockAmountExceedsLocked: {
		EN: "Unlock amount exceeds locked amount",
		ZH: "解锁金额超过锁仓金额",
	},
	AssetUnlockFailed: {
		EN: "Unlock failed",
		ZH: "解锁失败",
	},
	EuropeanOptionNotExpired: {
		EN: "European options cannot be exercised before expiration",
		ZH: "欧式期权未到期不能提前行权",
	},
	OptionNotInTheMoney: {
		EN: "The option is not in the money and cannot be exercised",
		ZH: "当前不是价内期权，不能行权",
	},
	RequestRequired: {
		EN: "Request is required",
		ZH: "请求不能为空",
	},
	UnderlyingPriceMustBePositive: {
		EN: "underlying_price must be greater than 0",
		ZH: "underlying_price 必须大于 0",
	},
	CryptoWalletAccountNotFound: {
		EN: "Crypto wallet account not found",
		ZH: "链上钱包账号不存在",
	},
	CryptoRechargeTxNotFound: {
		EN: "Crypto recharge transaction not found",
		ZH: "链上充值交易不存在",
	},
	CryptoRechargeAddressNotFound: {
		EN: "Crypto recharge address not found",
		ZH: "链上充值地址不存在",
	},
	InvalidCryptoRechargeParams: {
		EN: "Invalid crypto recharge parameters",
		ZH: "链上充值参数无效",
	},
	TenantOwnerRoleTemplateNotFound: {
		EN: "Tenant owner role template not found",
		ZH: "租户主账号角色模板不存在",
	},
	SyncTaskFailed: {
		EN: "Sync task failed",
		ZH: "同步任务执行失败",
	},
	AssetCoinConfigNotFound: {
		EN: "Asset coin config not found",
		ZH: "资产币种配置不存在",
	},
	UserSecurityInfoNotFound: {
		EN: "User security information not found",
		ZH: "用户安全信息不存在",
	},
	PasswordsDoNotMatch: {
		EN: "The two password entries do not match",
		ZH: "两次密码输入不一致",
	},
	TokenExpiredOrInvalid: {
		EN: "Token has expired or is invalid",
		ZH: "Token已过期或无效",
	},
	AccountDisabled: {
		EN: "Account has been disabled",
		ZH: "账户被禁用",
	},
	PleaseInitializeGoogle2FA: {
		EN: "Please initialize Google 2FA first",
		ZH: "请先初始化Google 2FA",
	},
	VerificationCodeInvalid: {
		EN: "Invalid verification code",
		ZH: "验证码错误",
	},
	PleaseSwitchDeviceToLogin: {
		EN: "Please switch devices to log in",
		ZH: "请确更换设备登录",
	},
	SecretGenerationFailed: {
		EN: "Failed to generate secret key",
		ZH: "生成密钥失败",
	},
	PermissionDeniedForBankCard: {
		EN: "No permission to set this bank card",
		ZH: "无权设置此银行卡",
	},
	NoPermissionOperateThisUser: {
		EN: "No permission to operate this user",
		ZH: "无权操作此用户",
	},
	NoPermissionModifyThisBankCard: {
		EN: "No permission to modify this bank card",
		ZH: "无权修改此银行卡",
	},
	NoPermissionDeleteThisBankCard: {
		EN: "No permission to delete this bank card",
		ZH: "无权删除此银行卡",
	},
	NoPermissionOperateThisBankCard: {
		EN: "No permission to operate this bank card",
		ZH: "无权操作此银行卡",
	},
	SecuritySettingsNotFound: {
		EN: "Security settings not found",
		ZH: "安全设置不存在",
	},
	UserIdentityInfoNotFound: {
		EN: "User identity information not found",
		ZH: "用户身份信息不存在",
	},
	PayPasswordNotSet: {
		EN: "Payment password is not set",
		ZH: "支付密码未设置",
	},
	InvalidMenuType: {
		EN: "Invalid menu type",
		ZH: "Invalid menu type",
	},
	MenuAlreadyExists: {
		EN: "Menu already exists",
		ZH: "Menu already exists",
	},
	MenuNotFoundEN: {
		EN: "Menu not found",
		ZH: "Menu not found",
	},
	Google2FANotInitialized: {
		EN: "Google 2FA has not been initialized",
		ZH: "Google 2FA 尚未初始化",
	},
	UserStatusUnchanged: {
		EN: "User status is unchanged",
		ZH: "用户状态未改变",
	},
	InvalidCronExpression: {
		EN: "Invalid cron expression",
		ZH: "无效的 Cron 表达式",
	},
	CronJobAlreadyExists: {
		EN: "Cron job already exists",
		ZH: "定时任务已存在",
	},
	UserIDInvalid: {
		EN: "Invalid user ID",
		ZH: "用户ID错误",
	},
	RoleSelectionRequired: {
		EN: "Please select a role",
		ZH: "请选择角色",
	},
	ValidRoleSelectionRequired: {
		EN: "Please select a valid role",
		ZH: "请选择有效角色",
	},
	SomeRolesNotFound: {
		EN: "Some roles do not exist",
		ZH: "部分角色不存在",
	},
	UsernameAlreadyExists: {
		EN: "Username already exists",
		ZH: "用户名已存在",
	},
	UserDisabledForLogin: {
		EN: "User has been disabled",
		ZH: "用户已被禁用",
	},
	AssetNotFound: {
		EN: "Asset not found",
		ZH: "资产不存在",
	},
	NoPermissionQueryOrder: {
		EN: "No permission to query this order",
		ZH: "无权查询该订单",
	},
	NoPermissionOperatePosition: {
		EN: "No permission to operate this position",
		ZH: "无权操作该持仓",
	},
	RegistrationTooFrequentRetry: {
		EN: "Registration is too frequent, please try again later",
		ZH: "注册过于频繁，请稍后再试",
	},
	Google2FASecretFetchFailed: {
		EN: "Failed to fetch 2FA secret",
		ZH: "获取2FA secret失败",
	},
	Google2FASecretExpired: {
		EN: "2FA secret has expired, please initialize again",
		ZH: "2FA secret已过期，请重新初始化",
	},
	Generate2FASecretFailed: {
		EN: "Failed to generate 2FA secret",
		ZH: "生成2FA secret失败",
	},
	Store2FASecretFailed: {
		EN: "Failed to store 2FA secret",
		ZH: "存储2FA secret失败",
	},
	APIURLIsRequired: {
		EN: "apiURL is required",
		ZH: "apiURL is required",
	},
	TokenRequired: {
		EN: "token is required",
		ZH: "token is required",
	},
	CategoryRequired: {
		EN: "category is required",
		ZH: "category is required",
	},
	MarketRequired: {
		EN: "market is required",
		ZH: "market is required",
	},
	SymbolRequired: {
		EN: "symbol is required",
		ZH: "symbol is required",
	},
	UserNotFoundOrPasswordIncorrect: {
		EN: "User not found or password incorrect",
		ZH: "用户不存在或密码错误",
	},
	StakingProductUnavailable: {
		EN: "Staking product is unavailable",
		ZH: "质押产品当前不可用",
	},
	StakeAmountInvalid: {
		EN: "Invalid stake amount",
		ZH: "质押数量无效",
	},
	StakeAmountBelowMinimum: {
		EN: "Stake amount is below the minimum",
		ZH: "质押数量低于最小限制",
	},
	StakeAmountAboveMaximum: {
		EN: "Stake amount exceeds the maximum",
		ZH: "质押数量超过最大限制",
	},
	StakeAmountStepInvalid: {
		EN: "Stake amount does not match the step size",
		ZH: "质押数量不符合递增步长",
	},
	ProductQuotaInsufficient: {
		EN: "Insufficient product quota",
		ZH: "产品可质押额度不足",
	},
	UserStakeLimitExceeded: {
		EN: "User stake limit exceeded",
		ZH: "超过用户可质押额度限制",
	},
	ProductNoAlreadyExists: {
		EN: "Product number already exists",
		ZH: "产品编号已存在",
	},
	StakingOrderCannotRedeem: {
		EN: "The staking order cannot be redeemed",
		ZH: "当前质押订单不可赎回",
	},
	EarlyRedeemNotAllowed: {
		EN: "Early redeem is not allowed",
		ZH: "当前订单不允许提前赎回",
	},
	RedeemAmountInvalid: {
		EN: "Invalid redeem amount",
		ZH: "赎回数量无效",
	},
	RewardAmountInvalid: {
		EN: "Invalid reward amount",
		ZH: "收益数量无效",
	},
	SuperAdminCannotBeDeleted: {
		EN: "System super administrator cannot be deleted",
		ZH: "系统超级管理员不能删除",
	},
	TenantOwnerCannotBeDeleted: {
		EN: "Tenant owner account cannot be deleted",
		ZH: "租户主账号不能删除",
	},
	SuperAdminUpdateForbidden: {
		EN: "System super administrator cannot be modified",
		ZH: "系统超级管理员不能修改",
	},
	SuperAdminDeleteForbidden: {
		EN: "System super administrator cannot be deleted",
		ZH: "系统超级管理员不能删除",
	},
	PayProductCodeAlreadyExists: {
		EN: "Payment product code already exists",
		ZH: "支付产品编码已存在",
	},
	PayPlatformCodeAlreadyExists: {
		EN: "Payment platform code already exists",
		ZH: "支付平台编码已存在",
	},
	TenantPayAccountCodeAlreadyExists: {
		EN: "Tenant payment account code already exists",
		ZH: "租户支付账号编码已存在",
	},
	TenantPayChannelCodeAlreadyExists: {
		EN: "Tenant payment channel code already exists",
		ZH: "租户支付通道编码已存在",
	},
	PaymentRequiredParamsMissing: {
		EN: "Required payment parameters are missing",
		ZH: "支付必填参数缺失",
	},
	InvalidPaymentJSON: {
		EN: "Payment JSON configuration is invalid",
		ZH: "支付 JSON 配置格式不正确",
	},
	InvalidPaymentAmountRange: {
		EN: "Payment amount or level range is invalid",
		ZH: "支付金额或等级范围不正确",
	},
	PaymentRelationMismatch: {
		EN: "Payment platform, product, account, or channel relation does not match",
		ZH: "支付平台、产品、账号或通道归属关系不匹配",
	},
	CryptoResourceAlreadyExists: {
		EN: "Crypto address, account, or transaction already exists",
		ZH: "链上地址、账号或交易已存在",
	},
	InvalidPaymentDecimal: {
		EN: "Payment decimal value is invalid",
		ZH: "支付小数参数格式不正确",
	},
}
