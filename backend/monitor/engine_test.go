package monitor

import (
	"strings"
	"testing"
	"time"

	"github.com/cn-maul/Gentry/database"
)

func TestNormalizeMoney(t *testing.T) {
	tests := []struct {
		input    string
		minor    int64
		currency string
		valid    bool
	}{
		{"¥1,299.00", 129900, "CNY", true},
		{"$99.99", 9999, "USD", true},
		{"€12.50", 1250, "EUR", true},
		{"1299.00元", 129900, "CNY", true},
		{"1,234.56", 123456, "CNY", true},
		{"", 0, "", false},
		{"免费", 0, "", false},
		{"暂无报价", 0, "", false},
		{"原价100 现价90", 0, "", false},
		{"¥0.01", 1, "CNY", true},
		{"100", 10000, "CNY", true},
		{"HK$99.90", 9990, "HKD", true},
	}

	for _, tt := range tests {
		result := NormalizeField(tt.input, "money")
		if result.Valid != tt.valid {
			t.Errorf("NormalizeMoney(%q) valid = %v, want %v", tt.input, result.Valid, tt.valid)
		}
		if result.Valid {
			if result.Minor != tt.minor {
				t.Errorf("NormalizeMoney(%q) minor = %d, want %d", tt.input, result.Minor, tt.minor)
			}
			if result.Currency != tt.currency {
				t.Errorf("NormalizeMoney(%q) currency = %s, want %s", tt.input, result.Currency, tt.currency)
			}
		}
	}
}

func TestNormalizeDecimal(t *testing.T) {
	tests := []struct {
		input string
		minor int64
		valid bool
	}{
		{"12.34", 1234, true},
		{"100", 10000, true},
		{"", 0, false},
		{"abc", 0, false},
	}

	for _, tt := range tests {
		result := NormalizeField(tt.input, "decimal")
		if result.Valid != tt.valid {
			t.Errorf("NormalizeDecimal(%q) valid = %v, want %v", tt.input, result.Valid, tt.valid)
		}
		if result.Valid && result.Minor != tt.minor {
			t.Errorf("NormalizeDecimal(%q) minor = %d, want %d", tt.input, result.Minor, tt.minor)
		}
	}
}

func TestGenerateItemKey(t *testing.T) {
	obs := ExtractResult{"title": "测试商品", "url": "https://example.com/item/1", "sku": "SKU123"}

	// source_url
	key := GenerateItemKey(obs, IdentityConfig{Source: "source_url"}, "https://example.com/item/1")
	if key != "https://example.com/item/1" {
		t.Errorf("source_url key = %q, want %q", key, "https://example.com/item/1")
	}

	// single field
	key = GenerateItemKey(obs, IdentityConfig{Field: "sku"}, "")
	if key != "SKU123" {
		t.Errorf("field key = %q, want %q", key, "SKU123")
	}

	// multiple fields
	key = GenerateItemKey(obs, IdentityConfig{Fields: []string{"title", "url"}}, "")
	if key != "测试商品|https://example.com/item/1" {
		t.Errorf("fields key = %q, want %q", key, "测试商品|https://example.com/item/1")
	}
}

func TestPresenceDetector(t *testing.T) {
	detector := &PresenceDetector{}

	// First baseline - no events
	obs1 := []Observation{
		{ItemKey: "item1", Fields: map[string]TypedValue{"title": {Value: "A", Valid: true}}, SeenAt: time.Now()},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)
	if len(result.Events) != 0 {
		t.Errorf("first baseline should have no events, got %d", len(result.Events))
	}
	if len(result.NextSnapshots) != 1 {
		t.Errorf("should have 1 snapshot, got %d", len(result.NextSnapshots))
	}

	// Second run - new item
	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}
	obs2 := []Observation{
		{ItemKey: "item1", Fields: map[string]TypedValue{"title": {Value: "A", Valid: true}}, SeenAt: time.Now()},
		{ItemKey: "item2", Fields: map[string]TypedValue{"title": {Value: "B", Valid: true}}, SeenAt: time.Now()},
	}
	result2 := detector.Evaluate(previous, obs2)
	if len(result2.Events) != 1 {
		t.Errorf("should have 1 event (item2), got %d", len(result2.Events))
	}
	if result2.Events[0].EventType != "item_added" {
		t.Errorf("event type should be item_added, got %s", result2.Events[0].EventType)
	}
}

func TestFieldTransitionDetector_PriceDrop(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{
			{Field: "price", ValueType: "money", Operator: "decreased"},
		},
		OnFirstBaseline: "silent",
	})

	// First baseline
	obs1 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: "¥100.00", DataType: "money", Minor: 10000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)
	if len(result.Events) != 0 {
		t.Errorf("first baseline should have no events, got %d", len(result.Events))
	}

	// Price drop from 100 to 90
	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}
	obs2 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: "¥90.00", DataType: "money", Minor: 9000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result2 := detector.Evaluate(previous, obs2)
	if len(result2.Events) != 1 {
		t.Errorf("should have 1 price drop event, got %d", len(result2.Events))
	}
	if result2.Events[0].EventType != "price_dropped" {
		t.Errorf("event type should be price_dropped, got %s", result2.Events[0].EventType)
	}
	if result2.Events[0].ChangeAmount != 1000 {
		t.Errorf("change amount should be 1000 (¥1.00), got %d", result2.Events[0].ChangeAmount)
	}
	if result2.Events[0].ChangePercent < 9.9 || result2.Events[0].ChangePercent > 10.1 {
		t.Errorf("change percent should be ~10, got %.2f", result2.Events[0].ChangePercent)
	}

	// Price increase from 90 to 95 - no event
	previous2 := make(SnapshotSet)
	for _, s := range result2.NextSnapshots {
		previous2[s.ItemKey] = s
	}
	obs3 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: "¥95.00", DataType: "money", Minor: 9500, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result3 := detector.Evaluate(previous2, obs3)
	if len(result3.Events) != 0 {
		t.Errorf("price increase should not generate event, got %d", len(result3.Events))
	}

	// Another drop from 95 to 80 - should generate event
	previous3 := make(SnapshotSet)
	for _, s := range result3.NextSnapshots {
		previous3[s.ItemKey] = s
	}
	obs4 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: "¥80.00", DataType: "money", Minor: 8000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result4 := detector.Evaluate(previous3, obs4)
	if len(result4.Events) != 1 {
		t.Errorf("second price drop should generate event, got %d", len(result4.Events))
	}

	// Invalid price - should not generate event
	previous4 := make(SnapshotSet)
	for _, s := range result4.NextSnapshots {
		previous4[s.ItemKey] = s
	}
	obs5 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: "", DataType: "money", Minor: 0, Currency: "CNY", Valid: false},
			},
			SeenAt: time.Now(),
		},
	}
	result5 := detector.Evaluate(previous4, obs5)
	if len(result5.Events) != 0 {
		t.Errorf("invalid price should not generate event, got %d", len(result5.Events))
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		minor    int64
		currency string
		expected string
	}{
		{129900, "CNY", "¥1299.00"},
		{9999, "USD", "$99.99"},
		{1250, "EUR", "€12.50"},
		{1000, "JPY", "¥1000"},
	}

	for _, tt := range tests {
		result := formatPrice(tt.minor, tt.currency)
		if result != tt.expected {
			t.Errorf("formatPrice(%d, %s) = %q, want %q", tt.minor, tt.currency, result, tt.expected)
		}
	}
}

func TestSnapshotMissing(t *testing.T) {
	detector := &PresenceDetector{}

	// First run: item1 and item2
	obs1 := []Observation{
		{ItemKey: "item1", Fields: map[string]TypedValue{"title": {Value: "A", Valid: true}}, SeenAt: time.Now()},
		{ItemKey: "item2", Fields: map[string]TypedValue{"title": {Value: "B", Valid: true}}, SeenAt: time.Now()},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)

	// Second run: only item1 (item2 missing)
	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}
	obs2 := []Observation{
		{ItemKey: "item1", Fields: map[string]TypedValue{"title": {Value: "A", Valid: true}}, SeenAt: time.Now()},
	}
	result2 := detector.Evaluate(previous, obs2)

	// item2 should have MissingChecks = 1
	for _, s := range result2.NextSnapshots {
		if s.ItemKey == "item2" {
			if s.MissingChecks != 1 {
				t.Errorf("item2 missing_checks should be 1, got %d", s.MissingChecks)
			}
			return
		}
	}
	t.Error("item2 should still be in snapshots")
}

func TestFormatEvent(t *testing.T) {
	event := ChangeEvent{
		EventType:     "price_dropped",
		ItemKey:       "product1",
		Title:         "无线耳机",
		URL:           "https://example.com/product/1",
		OldValue:      "¥100.00",
		NewValue:      "¥90.00",
		ChangeAmount:  1000,
		ChangePercent: 10.0,
		Currency:      "CNY",
	}

	title, content := FormatEvent(event, "测试监控")
	if title != "降价提醒: 无线耳机" {
		t.Errorf("title = %q, want %q", title, "降价提醒: 无线耳机")
	}
	if content == "" {
		t.Error("content should not be empty")
	}
}

func TestFormatEventComplete(t *testing.T) {
	event := ChangeEvent{
		EventType:     "price_dropped",
		Title:         "无线耳机",
		URL:           "https://example.com/product/1",
		OldValue:      "¥100.00",
		NewValue:      "¥90.00",
		ChangeAmount:  1000,
		ChangePercent: 10.0,
		Currency:      "CNY",
	}

	title, content := FormatEvent(event, "测试监控")
	if !strings.Contains(content, "原价: ¥100.00") {
		t.Errorf("content should contain old price, got: %s", content)
	}
	if !strings.Contains(content, "现价: ¥90.00") {
		t.Errorf("content should contain new price, got: %s", content)
	}
	if !strings.Contains(content, "10.00%") {
		t.Errorf("content should contain percent, got: %s", content)
	}
	if !strings.Contains(content, "https://example.com/product/1") {
		t.Errorf("content should contain URL, got: %s", content)
	}
	_ = title
}

func TestParseDetectionRule(t *testing.T) {
	// Empty config
	rule, err := ParseDetectionRule("")
	if err != nil {
		t.Errorf("empty config should not error: %v", err)
	}
	if rule.Type != "presence" {
		t.Errorf("default type should be presence, got %s", rule.Type)
	}

	// Field transition config
	config := `{"type":"field_transition","identity":{"field":"url"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`
	rule, err = ParseDetectionRule(config)
	if err != nil {
		t.Errorf("valid config should not error: %v", err)
	}
	if rule.Type != "field_transition" {
		t.Errorf("type should be field_transition, got %s", rule.Type)
	}
	if len(rule.Conditions) != 1 {
		t.Errorf("should have 1 condition, got %d", len(rule.Conditions))
	}
}

// P0-2: 自定义价格字段能产生事件
func TestCustomPriceField(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{
			{Field: "sale_price", ValueType: "money", Operator: "decreased"},
		},
	})

	obs1 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title":      {Value: "商品A", Valid: true},
				"sale_price": {Value: "¥100.00", DataType: "money", Minor: 10000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)

	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}

	obs2 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title":      {Value: "商品A", Valid: true},
				"sale_price": {Value: "¥80.00", DataType: "money", Minor: 8000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result2 := detector.Evaluate(previous, obs2)
	if len(result2.Events) != 1 {
		t.Errorf("custom price field should generate event, got %d", len(result2.Events))
	}
}

// P0-2: 阈值生效
func TestPriceDropThreshold(t *testing.T) {
	// 5% 阈值
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{
			{Field: "price", ValueType: "money", Operator: "decreased", Threshold: &ThresholdConfig{Percent: 5}},
		},
	})

	obs1 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥100.00", DataType: "money", Minor: 10000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)

	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}

	// 降价 3% - 低于阈值，不应产生事件
	obs2 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥97.00", DataType: "money", Minor: 9700, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result2 := detector.Evaluate(previous, obs2)
	if len(result2.Events) != 0 {
		t.Errorf("3%% drop below 5%% threshold should not generate event, got %d", len(result2.Events))
	}

	// 更新快照
	previous2 := make(SnapshotSet)
	for _, s := range result2.NextSnapshots {
		previous2[s.ItemKey] = s
	}

	// 降价 10% - 超过阈值，应产生事件
	obs3 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥90.00", DataType: "money", Minor: 9000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result3 := detector.Evaluate(previous2, obs3)
	if len(result3.Events) != 1 {
		t.Errorf("10%% drop above 5%% threshold should generate event, got %d", len(result3.Events))
	}
}

// P0-3: identity.source=source_url 使用 Site.URL
func TestIdentitySourceURL(t *testing.T) {
	obs := ExtractResult{"title": "商品"}
	key := GenerateItemKey(obs, IdentityConfig{Source: "source_url"}, "https://example.com/product/1")
	if key != "https://example.com/product/1" {
		t.Errorf("source_url identity should use site URL, got %q", key)
	}
}

// P0-4: 100 → invalid → 90 仍然降价
func TestInvalidPricePreservesOld(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{
			{Field: "price", ValueType: "money", Operator: "decreased"},
		},
	})

	// 100
	obs1 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥100.00", DataType: "money", Minor: 10000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)

	// invalid price
	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}
	obs2 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "", DataType: "money", Minor: 0, Currency: "CNY", Valid: false},
			},
			SeenAt: time.Now(),
		},
	}
	result2 := detector.Evaluate(previous, obs2)

	// 验证旧价格被保留
	for _, s := range result2.NextSnapshots {
		if s.ItemKey == "product1" {
			if !s.PriceValid {
				t.Error("invalid price should not overwrite valid price, PriceValid should remain true")
			}
			if s.PriceMinor != 10000 {
				t.Errorf("invalid price should not overwrite valid price, PriceMinor should be 10000, got %d", s.PriceMinor)
			}
		}
	}

	// 90 - 基于旧价格 100 比较
	previous2 := make(SnapshotSet)
	for _, s := range result2.NextSnapshots {
		previous2[s.ItemKey] = s
	}
	obs3 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥90.00", DataType: "money", Minor: 9000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result3 := detector.Evaluate(previous2, obs3)
	if len(result3.Events) != 1 {
		t.Errorf("100 -> invalid -> 90 should still produce price drop event, got %d", len(result3.Events))
	}
}

// P0-6: 确定性去重键
func TestDedupeKeyDeterministic(t *testing.T) {
	key1 := GenerateDedupeKey(1, 1, "price_dropped", "item1", "fp1", "fp2")
	key2 := GenerateDedupeKey(1, 1, "price_dropped", "item1", "fp1", "fp2")
	if key1 != key2 {
		t.Errorf("same inputs should produce same dedupe key")
	}

	key3 := GenerateDedupeKey(1, 1, "price_dropped", "item1", "fp1", "fp3")
	if key1 == key3 {
		t.Errorf("different after_fingerprint should produce different dedupe key")
	}
	key4 := GenerateDedupeKey(1, 2, "price_dropped", "item1", "fp1", "fp2")
	if key1 == key4 {
		t.Error("different definition versions must produce different dedupe keys")
	}
	if len(key1) != 64 {
		t.Errorf("dedupe key should be a SHA-256 hex digest, got length %d", len(key1))
	}
}

// P0-3: 不同币种不产生降价事件
func TestCurrencyMismatchNoEvent(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{
			{Field: "price", ValueType: "money", Operator: "decreased"},
		},
	})

	obs1 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥100.00", DataType: "money", Minor: 10000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)

	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}

	// USD 90 - 币种不同，不应产生降价事件
	obs2 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "$90.00", DataType: "money", Minor: 9000, Currency: "USD", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result2 := detector.Evaluate(previous, obs2)
	if len(result2.Events) != 0 {
		t.Errorf("CNY -> USD should not produce price drop event, got %d", len(result2.Events))
	}

	// 验证旧 CNY 价格被保留
	for _, s := range result2.NextSnapshots {
		if s.ItemKey == "product1" {
			if s.Currency != "CNY" {
				t.Errorf("currency should remain CNY, got %s", s.Currency)
			}
			if s.PriceMinor != 10000 {
				t.Errorf("price should remain 10000, got %d", s.PriceMinor)
			}
		}
	}
}

// P0-2: 金额阈值按货币单位解释（10元 = 1000分）
func TestAmountThresholdInYuan(t *testing.T) {
	// 10元阈值
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{
			{Field: "price", ValueType: "money", Operator: "decreased", Threshold: &ThresholdConfig{Amount: "10"}},
		},
	})

	obs1 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥100.00", DataType: "money", Minor: 10000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)

	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}

	// 降价5元 (100→95) - 低于10元阈值，不产生事件
	obs2 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥95.00", DataType: "money", Minor: 9500, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result2 := detector.Evaluate(previous, obs2)
	if len(result2.Events) != 0 {
		t.Errorf("5元 drop below 10元 threshold should not generate event, got %d", len(result2.Events))
	}

	previous2 := make(SnapshotSet)
	for _, s := range result2.NextSnapshots {
		previous2[s.ItemKey] = s
	}

	// 降价20元 (95→80) - 超过10元阈值，应产生事件
	obs3 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"price": {Value: "¥80.00", DataType: "money", Minor: 8000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result3 := detector.Evaluate(previous2, obs3)
	if len(result3.Events) != 1 {
		t.Errorf("20元 drop above 10元 threshold should generate event, got %d", len(result3.Events))
	}
}

// P1-6: 无效价格保留完整旧快照
func TestInvalidPricePreservesFullPayload(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{
			{Field: "price", ValueType: "money", Operator: "decreased"},
		},
	})

	obs1 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: "¥100.00", DataType: "money", Minor: 10000, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(SnapshotSet{}, obs1)

	previous := make(SnapshotSet)
	for _, s := range result.NextSnapshots {
		previous[s.ItemKey] = s
	}

	// 无效价格
	obs2 := []Observation{
		{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: "", DataType: "money", Minor: 0, Currency: "CNY", Valid: false},
			},
			SeenAt: time.Now(),
		},
	}
	result2 := detector.Evaluate(previous, obs2)

	// 验证价格和 payload 都被保留
	for _, s := range result2.NextSnapshots {
		if s.ItemKey == "product1" {
			if !s.PriceValid {
				t.Error("PriceValid should remain true")
			}
			if s.PriceMinor != 10000 {
				t.Errorf("PriceMinor should be 10000, got %d", s.PriceMinor)
			}
			if s.Payload["title"] != "商品A" {
				t.Errorf("Payload title should be preserved, got %v", s.Payload["title"])
			}
		}
	}
}

func TestNormalizeMoneyCurrencyExponentAndLocale(t *testing.T) {
	tests := []struct {
		input     string
		minor     int64
		currency  string
		formatted string
	}{
		{"JPY 1000", 1000, "JPY", "¥1000"},
		{"₩12,000", 12000, "KRW", "₩12000"},
		{"€1.299,99", 129999, "EUR", "€1299.99"},
		{"$1,299.99", 129999, "USD", "$1299.99"},
	}
	for _, tt := range tests {
		value := NormalizeField(tt.input, "money")
		if !value.Valid {
			t.Fatalf("%q should be a valid amount", tt.input)
		}
		if value.Minor != tt.minor || value.Currency != tt.currency {
			t.Errorf("NormalizeField(%q) = %s/%d, want %s/%d", tt.input, value.Currency, value.Minor, tt.currency, tt.minor)
		}
		if got := formatPrice(value.Minor, value.Currency); got != tt.formatted {
			t.Errorf("formatPrice for %q = %q, want %q", tt.input, got, tt.formatted)
		}
	}
	if got, err := parseThresholdMinor("10.10", "JPY"); err != nil || got != 11 {
		t.Errorf("JPY threshold should round up to 11 minor units, got %d, err=%v", got, err)
	}
}

func TestNormalizeAndValidateSiteDefinition(t *testing.T) {
	site := &database.Site{
		URL:            "https://example.com/product/1",
		Container:      "body",
		StrategyType:   "field_transition",
		StrategyConfig: `{"type":"field_transition","identity":{"field":"sku"},"conditions":[{"field":"sale_price","value_type":"money","operator":"decreased","threshold":{"amount":"10.10","percent":5}}],"on_first_baseline":"silent"}`,
		Fields: []database.SiteField{
			{Name: "sku", Selector: ".sku", Type: "text"},
			{Name: "sale_price", Selector: ".price", Type: "text"},
		},
	}
	if err := NormalizeAndValidateSiteDefinition(site); err != nil {
		t.Fatalf("valid definition should pass: %v", err)
	}
	if !strings.Contains(site.FieldDataTypes, `"sale_price":"money"`) {
		t.Errorf("price field type should be normalized to money: %s", site.FieldDataTypes)
	}

	invalid := *site
	invalid.StrategyType = "presence"
	if err := NormalizeAndValidateSiteDefinition(&invalid); err == nil {
		t.Error("strategy type mismatch should be rejected")
	}

	priceIdentity := &database.Site{
		URL: "https://example.com/products", Container: ".products", Item: ".product", StrategyType: "field_transition",
		StrategyConfig: `{"type":"field_transition","identity":{"field":"price"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`,
		Fields:         []database.SiteField{{Name: "price", Selector: ".price", Type: "text"}},
	}
	if err := NormalizeAndValidateSiteDefinition(priceIdentity); err == nil {
		t.Error("price field must not be accepted as identity")
	}
}

func TestPriceEventDoesNotUseSKUAsURL(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type:       "field_transition",
		Conditions: []Condition{{Field: "price", ValueType: "money", Operator: "decreased"}},
	})
	previous := SnapshotSet{
		"SKU-1": {
			ItemKey: "SKU-1", Payload: map[string]interface{}{"price": "CNY100.00"},
			PriceMinor: 10000, PriceValid: true, Currency: "CNY", FirstSeenAt: time.Now(),
		},
	}
	result := detector.Evaluate(previous, []Observation{{
		ItemKey: "SKU-1",
		Fields: map[string]TypedValue{
			"title": {Value: "商品", Valid: true},
			"price": {Value: "CNY90.00", DataType: "money", Minor: 9000, Currency: "CNY", Valid: true},
		},
	}})
	if len(result.Events) != 1 {
		t.Fatalf("expected one price event, got %d", len(result.Events))
	}
	if result.Events[0].URL != "" {
		t.Errorf("detector should leave unknown URL empty, got %q", result.Events[0].URL)
	}
}

func TestPriceTargetReachedOnlyOnBoundaryCrossing(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{{
			Field: "price", ValueType: "money", Operator: "at_or_below",
			Threshold: &ThresholdConfig{Value: "90"},
		}},
	})

	previous := SnapshotSet{}
	check := func(minor int64) []ChangeEvent {
		result := detector.Evaluate(previous, []Observation{{
			ItemKey: "product1",
			Fields: map[string]TypedValue{
				"title": {Value: "商品A", Valid: true},
				"price": {Value: formatPrice(minor, "CNY"), DataType: "money", Minor: minor, Currency: "CNY", Valid: true},
			},
			SeenAt: time.Now(),
		}})
		previous = make(SnapshotSet, len(result.NextSnapshots))
		for _, snapshot := range result.NextSnapshots {
			previous[snapshot.ItemKey] = snapshot
		}
		return result.Events
	}

	steps := []struct {
		minor      int64
		wantEvents int
	}{
		{10000, 0}, // 首次只建立基线
		{9500, 0},  // 尚未到目标价
		{9000, 1},  // 跨入目标区间
		{8000, 0},  // 持续低于目标价不重复
		{10000, 0}, // 涨回目标价以上，重新武装
		{8500, 1},  // 再次跨入时再次通知
	}
	for _, step := range steps {
		events := check(step.minor)
		if len(events) != step.wantEvents {
			t.Fatalf("price %d produced %d events, want %d", step.minor, len(events), step.wantEvents)
		}
		if len(events) == 1 && events[0].EventType != "price_target_reached" {
			t.Fatalf("event type = %q, want price_target_reached", events[0].EventType)
		}
	}
}

func TestPriceTargetReachedFirstBaselineBelowTargetIsSilent(t *testing.T) {
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{{
			Field: "price", ValueType: "money", Operator: "at_or_below",
			Threshold: &ThresholdConfig{Value: "90"},
		}},
	})
	result := detector.Evaluate(SnapshotSet{}, []Observation{{
		ItemKey: "product1",
		Fields: map[string]TypedValue{
			"price": {Value: "¥80.00", DataType: "money", Minor: 8000, Currency: "CNY", Valid: true},
		},
	}})
	if len(result.Events) != 0 {
		t.Fatalf("first baseline below target produced %d events", len(result.Events))
	}
}

func TestPriceTargetUsesCorrectCurrencyBoundary(t *testing.T) {
	if got, err := parseTargetMinor("90.5", "JPY"); err != nil || got != 90 {
		t.Fatalf("JPY target should round down to 90, got %d, err=%v", got, err)
	}
	detector := NewFieldTransitionDetector(DetectionRule{
		Type: "field_transition",
		Conditions: []Condition{{
			Field: "price", ValueType: "money", Operator: "at_or_below",
			Threshold: &ThresholdConfig{Value: "90.5"},
		}},
	})
	previous := SnapshotSet{"product1": {
		ItemKey: "product1", Payload: map[string]interface{}{"price": "JPY100"},
		PriceMinor: 100, PriceValid: true, Currency: "JPY", FirstSeenAt: time.Now(),
	}}
	result := detector.Evaluate(previous, []Observation{{
		ItemKey: "product1",
		Fields: map[string]TypedValue{
			"price": {Value: "JPY91", DataType: "money", Minor: 91, Currency: "JPY", Valid: true},
		},
	}})
	if len(result.Events) != 0 {
		t.Fatalf("JPY 91 must not satisfy price <= 90.5, got %d events", len(result.Events))
	}
}

func TestPriceTargetRuleParsingAndValidation(t *testing.T) {
	config := `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"at_or_below","threshold":{"value":" 90.00 "}}],"on_first_baseline":"silent"}`
	rule, err := ParseDetectionRule(config)
	if err != nil {
		t.Fatalf("parse target rule: %v", err)
	}
	if rule.Conditions[0].Threshold == nil || rule.Conditions[0].Threshold.Value != "90.00" {
		t.Fatalf("target threshold was not preserved: %#v", rule.Conditions[0].Threshold)
	}

	validSite := &database.Site{
		URL: "https://example.com/product/1", Container: "body", StrategyType: "field_transition",
		StrategyConfig: config,
		Fields:         []database.SiteField{{Name: "price", Selector: ".price", Type: "text"}},
	}
	if err := NormalizeAndValidateSiteDefinition(validSite); err != nil {
		t.Fatalf("valid target rule should pass: %v", err)
	}

	for name, invalidConfig := range map[string]string{
		"missing target":  `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"at_or_below"}],"on_first_baseline":"silent"}`,
		"negative target": `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"at_or_below","threshold":{"value":"-1"}}],"on_first_baseline":"silent"}`,
	} {
		site := &database.Site{
			URL: "https://example.com/product/1", Container: "body", StrategyType: "field_transition",
			StrategyConfig: invalidConfig,
			Fields:         []database.SiteField{{Name: "price", Selector: ".price", Type: "text"}},
		}
		if err := NormalizeAndValidateSiteDefinition(site); err == nil {
			t.Errorf("%s should be rejected", name)
		}
	}
}

func TestFormatPriceTargetReachedEvent(t *testing.T) {
	event := ChangeEvent{
		EventType: "price_target_reached", Title: "无线耳机",
		URL: "https://example.com/product/1", OldValue: "¥100.00", NewValue: "¥90.00",
	}
	title, content := FormatEvent(event, "测试监控")
	if title != "到价提醒: 无线耳机" {
		t.Fatalf("title = %q", title)
	}
	for _, expected := range []string{"之前价格: ¥100.00", "当前价格: ¥90.00", "https://example.com/product/1"} {
		if !strings.Contains(content, expected) {
			t.Errorf("content should contain %q, got %q", expected, content)
		}
	}
}
