package notify

import (
	"testing"
)

func TestRegistryRegisterAndList(t *testing.T) {
	// 验证服务注册表基本功能
	providers := ListProviderMetadata()
	if len(providers) == 0 {
		t.Fatal("期望至少有一个已注册的推送服务")
	}

	found := false
	for name := range ListProviderMetadata() {
		if name == "pushplus" {
			found = true
			break
		}
	}
	if !found {
		t.Error("期望找到 pushplus 服务")
	}
}

func TestRegistryDuplicateRegisterPanics(t *testing.T) {
	// Register 不会 panic，只是覆盖，这里验证重复注册不会崩溃
	Register("test_dup", func(config map[string]interface{}) (Notifier, error) {
		return nil, nil
	})
	// 重复注册同一名称，应覆盖而非 panic
	Register("test_dup", func(config map[string]interface{}) (Notifier, error) {
		return nil, nil
	})
}

func TestValidateAccountConfig_UnknownService(t *testing.T) {
	err := ValidateAccountConfig("unknown_service", nil)
	if err == nil {
		t.Fatal("未知服务应返回错误")
	}
}

func TestValidateAccountConfig_PushPlusNoToken(t *testing.T) {
	err := ValidateAccountConfig("pushplus", map[string]interface{}{})
	if err != ErrMissingRequiredField {
		t.Fatalf("期望 ErrMissingRequiredField，得到 %v", err)
	}
}

func TestGlobalToggle(t *testing.T) {
	// 默认应关闭
	if IsEnabled() {
		t.Fatal("推送默认应关闭")
	}

	SetEnabled(true)
	if !IsEnabled() {
		t.Fatal("开启后 IsEnabled 应返回 true")
	}

	SetEnabled(false)
	if IsEnabled() {
		t.Fatal("关闭后 IsEnabled 应返回 false")
	}
}