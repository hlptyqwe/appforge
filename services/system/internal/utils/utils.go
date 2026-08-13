package utils

import (
	"appforge/proto/system"
	"encoding/json"
)

func CheckConfig(key string, value string) error {
	switch key {
	case system.SysConfigType_OBJECT_STORAGE.String():
		var objectStorageConfig system.ObjectStorageConfig
		return json.Unmarshal([]byte(value), &objectStorageConfig)
	case system.SysConfigType_SYSTEM_CORE.String():
		var systemCore system.SystemCore
		return json.Unmarshal([]byte(value), &systemCore)
	case system.SysConfigType_ITICK_CONFIG.String():
		var itickConfig system.ITickConfig
		return json.Unmarshal([]byte(value), &itickConfig)
	case system.SysConfigType_RECHARGE_CONFIG.String():
		var rechargeConfig system.RechargeConfig
		return json.Unmarshal([]byte(value), &rechargeConfig)
	case system.SysConfigType_WITHDRAW_CONFIG.String():
		var withdrawConfig system.WithdrawConfig
		return json.Unmarshal([]byte(value), &withdrawConfig)
	case system.SysConfigType_EMAIL_CONFIG.String():
		var emailConfig system.EmailConfig
		return json.Unmarshal([]byte(value), &emailConfig)
	case system.SysConfigType_PHONE_CONFIG.String():
		var phoneConfig system.PhoneConfig
		return json.Unmarshal([]byte(value), &phoneConfig)
	case system.SysConfigType_CHAT_CONFIG.String():
		var chatConfig system.ChatConfig
		return json.Unmarshal([]byte(value), &chatConfig)
	default:
		return nil
	}
}
