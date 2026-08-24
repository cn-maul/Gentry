package notify

import (
	"fmt"
	"sync"
)

var (
	providers     = make(map[string]func(map[string]interface{}) (Notifier, error))
	providerMeta  = make(map[string]*ProviderMetadata)
	providersLock sync.RWMutex
)

// ProviderMetadata 推送服务元数据
type ProviderMetadata struct {
	Label          string   `json:"label"`
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields"`
}

// Register 注册推送服务
func Register(name string, creator func(map[string]interface{}) (Notifier, error)) {
	RegisterWithMetadata(name, creator, &ProviderMetadata{})
}

// RegisterWithMetadata 注册推送服务并附带元数据
func RegisterWithMetadata(name string, creator func(map[string]interface{}) (Notifier, error), meta *ProviderMetadata) {
	providersLock.Lock()
	defer providersLock.Unlock()
	providers[name] = creator
	providerMeta[name] = meta
}

// GetProviderMetadata 获取指定服务的元数据
func GetProviderMetadata(service string) *ProviderMetadata {
	providersLock.RLock()
	defer providersLock.RUnlock()
	return providerMeta[service]
}

// ListProviderMetadata 获取所有已注册服务的元数据
func ListProviderMetadata() map[string]*ProviderMetadata {
	providersLock.RLock()
	defer providersLock.RUnlock()
	result := make(map[string]*ProviderMetadata)
	for k, v := range providerMeta {
		result[k] = v
	}
	return result
}

// ValidateAccountConfig 在创建/更新账户时校验配置
func ValidateAccountConfig(service string, config map[string]interface{}) error {
	meta := GetProviderMetadata(service)
	if meta == nil {
		return fmt.Errorf("未注册的推送服务: %s", service)
	}

	for _, field := range meta.RequiredFields {
		if _, ok := config[field]; !ok {
			return ErrMissingRequiredField
		}
		if v, ok := config[field].(string); ok && v == "" {
			return ErrMissingRequiredField
		}
	}

	return nil
}

var ErrMissingRequiredField = &missingFieldError{}

type missingFieldError struct{}

func (e *missingFieldError) Error() string {
	return "缺少必需的配置字段"
}
