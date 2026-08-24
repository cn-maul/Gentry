package monitor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// PresenceDetector 检测新增条目
type PresenceDetector struct{}

func (d *PresenceDetector) Validate(schema ExtractionSchema, config json.RawMessage) error {
	rule, err := ParseDetectionRule(string(config))
	if err != nil {
		return err
	}
	if rule.Type != "presence" {
		return fmt.Errorf("PresenceDetector 不能处理策略 %s", rule.Type)
	}
	return validateDetectionRule(*rule, schema, extractionFieldNames(schema), map[string]string{})
}

func (d *PresenceDetector) Evaluate(previous SnapshotSet, current []Observation) EvaluationResult {
	now := time.Now()
	seen := make(map[string]bool)
	var nextSnapshots []Snapshot
	var events []ChangeEvent

	for _, obs := range current {
		itemKey := obs.ItemKey
		if itemKey == "" {
			continue
		}
		seen[itemKey] = true

		payload := make(map[string]interface{})
		for k, v := range obs.Fields {
			payload[k] = v.Value
		}
		payload["_item_key"] = itemKey
		fp := computeFingerprint(payload)

		ns := Snapshot{
			ItemKey:       itemKey,
			Payload:       payload,
			Fingerprint:   fp,
			LastSeenAt:    now,
			MissingChecks: 0,
		}

		existing, exists := previous[itemKey]
		if exists {
			ns.FirstSeenAt = existing.FirstSeenAt
			ns.DefinitionVersion = existing.DefinitionVersion
		} else {
			ns.FirstSeenAt = now
			if len(previous) > 0 {
				title := extractStr(payload, "title")
				urlStr := extractStr(payload, "url")
				events = append(events, ChangeEvent{
					EventType:  "item_added",
					ItemKey:    itemKey,
					Title:      title,
					URL:        urlStr,
					After:      payload,
					NewValue:   title,
					OccurredAt: now,
				})
			}
		}
		nextSnapshots = append(nextSnapshots, ns)
	}

	for key, snap := range previous {
		if !seen[key] {
			snap.MissingChecks++
			snap.LastSeenAt = now
			nextSnapshots = append(nextSnapshots, snap)
		}
	}

	return EvaluationResult{NextSnapshots: nextSnapshots, Events: events}
}

// FieldTransitionDetector 检测字段变化（价格下降等），基于 DetectionRule 配置
type FieldTransitionDetector struct {
	rule DetectionRule
}

func NewFieldTransitionDetector(rule DetectionRule) *FieldTransitionDetector {
	return &FieldTransitionDetector{rule: rule}
}

func (d *FieldTransitionDetector) Validate(schema ExtractionSchema, config json.RawMessage) error {
	if d.rule.Type != "field_transition" {
		return fmt.Errorf("FieldTransitionDetector 不能处理策略 %s", d.rule.Type)
	}
	return validateDetectionRule(d.rule, schema, extractionFieldNames(schema), map[string]string{})
}

func extractionFieldNames(schema ExtractionSchema) map[string]struct{} {
	fields := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		fields[field.Name] = struct{}{}
	}
	return fields
}

func (d *FieldTransitionDetector) Evaluate(previous SnapshotSet, current []Observation) EvaluationResult {
	now := time.Now()
	seen := make(map[string]bool)
	var nextSnapshots []Snapshot
	var events []ChangeEvent

	for _, obs := range current {
		itemKey := obs.ItemKey
		if itemKey == "" {
			continue
		}
		seen[itemKey] = true

		payload := make(map[string]interface{})
		for k, v := range obs.Fields {
			payload[k] = v.Value
		}
		payload["_item_key"] = itemKey

		// 为每个条件提取价格信息
		priceInfo := d.extractPriceInfo(obs)
		fp := computeFingerprint(payload)

		ns := Snapshot{
			ItemKey:       itemKey,
			Payload:       payload,
			Fingerprint:   fp,
			LastSeenAt:    now,
			MissingChecks: 0,
			Currency:      priceInfo.currency,
			PriceMinor:    priceInfo.minor,
			PriceValid:    priceInfo.valid,
		}

		existing, exists := previous[itemKey]
		if exists {
			ns.FirstSeenAt = existing.FirstSeenAt
			ns.DefinitionVersion = existing.DefinitionVersion

			// 只在旧价格有效且新价格也有效时进行比较
			if existing.PriceValid && priceInfo.valid {
				// P0-3: 币种不同时不比较
				if existing.Currency != priceInfo.currency {
					// 保留旧比较基线，不产生价格事件
					ns.Payload = existing.Payload
					ns.Fingerprint = existing.Fingerprint
					ns.PriceValid = existing.PriceValid
					ns.PriceMinor = existing.PriceMinor
					ns.Currency = existing.Currency
					nextSnapshots = append(nextSnapshots, ns)
					continue
				}

				eventType := d.matchingEventType(existing.PriceMinor, priceInfo.minor, priceInfo.currency)
				if eventType != "" {
					decrease := existing.PriceMinor - priceInfo.minor
					percent := float64(decrease) / float64(existing.PriceMinor) * 100

					oldPrice := formatPrice(existing.PriceMinor, existing.Currency)
					newPrice := formatPrice(priceInfo.minor, priceInfo.currency)

					title := extractStr(payload, "title")
					if title == "" {
						title = itemKey
					}
					urlStr := extractStr(payload, "url")

					events = append(events, ChangeEvent{
						EventType:     eventType,
						ItemKey:       itemKey,
						Title:         title,
						URL:           urlStr,
						Before:        existing.Payload,
						After:         payload,
						OldValue:      oldPrice,
						NewValue:      newPrice,
						ChangeAmount:  decrease,
						ChangePercent: math.Round(percent*100) / 100,
						Currency:      priceInfo.currency,
						OccurredAt:    now,
					})
				}
			} else if !priceInfo.valid {
				// 新价格无效：保留完整旧快照（Payload, Fingerprint, 价格字段）
				ns.Payload = existing.Payload
				ns.Fingerprint = existing.Fingerprint
				ns.PriceValid = existing.PriceValid
				ns.PriceMinor = existing.PriceMinor
				ns.Currency = existing.Currency
			}
		} else {
			ns.FirstSeenAt = now
			// 首次观测：如果价格无效，PriceValid 保持 false
		}

		nextSnapshots = append(nextSnapshots, ns)
	}

	for key, snap := range previous {
		if !seen[key] {
			snap.MissingChecks++
			snap.LastSeenAt = now
			nextSnapshots = append(nextSnapshots, snap)
		}
	}

	return EvaluationResult{NextSnapshots: nextSnapshots, Events: events}
}

func (d *FieldTransitionDetector) matchingEventType(oldMinor, newMinor int64, currency string) string {
	if len(d.rule.Conditions) == 0 {
		return ""
	}
	condition := d.rule.Conditions[0]
	switch condition.Operator {
	case "decreased":
		if newMinor < oldMinor && d.meetsThreshold(oldMinor, newMinor, currency) {
			return "price_dropped"
		}
	case "at_or_below":
		if condition.Threshold == nil || condition.Threshold.Value == "" {
			return ""
		}
		targetMinor, err := parseTargetMinor(condition.Threshold.Value, currency)
		if err == nil && oldMinor > targetMinor && newMinor <= targetMinor {
			return "price_target_reached"
		}
	}
	return ""
}

type priceInfo struct {
	minor    int64
	currency string
	valid    bool
}

func (d *FieldTransitionDetector) extractPriceInfo(obs Observation) priceInfo {
	for _, cond := range d.rule.Conditions {
		if cond.ValueType == "money" {
			if fv, ok := obs.Fields[cond.Field]; ok && fv.Valid {
				return priceInfo{minor: fv.Minor, currency: fv.Currency, valid: true}
			}
			return priceInfo{valid: false}
		}
	}
	return priceInfo{valid: false}
}

func (d *FieldTransitionDetector) meetsThreshold(oldMinor, newMinor int64, currency string) bool {
	for _, cond := range d.rule.Conditions {
		if cond.ValueType != "money" || cond.Threshold == nil {
			continue
		}
		if cond.Threshold.Amount != "" {
			minAmount, err := parseThresholdMinor(cond.Threshold.Amount, currency)
			if err != nil {
				return false
			}
			if minAmount > 0 && (oldMinor-newMinor) < minAmount {
				return false
			}
		}
		if cond.Threshold.Percent > 0 {
			decrease := oldMinor - newMinor
			percent := float64(decrease) / float64(oldMinor) * 100
			if percent < cond.Threshold.Percent {
				return false
			}
		}
	}
	return true
}

func computeFingerprint(m map[string]interface{}) string {
	return ComputeFingerprint(m)
}

func extractStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func formatPrice(minor int64, currency string) string {
	return currencySymbol(currency) + formatMinorNumber(minor, currencyExponent(currency))
}

// FormatPrice 按币种最小单位格式化金额，供 API 展示层复用。
func FormatPrice(minor int64, currency string) string {
	return formatPrice(minor, currency)
}

func currencySymbol(code string) string {
	symbols := map[string]string{
		"CNY": "¥", "USD": "$", "EUR": "€", "GBP": "£",
		"HKD": "HK$", "TWD": "NT$", "KRW": "₩", "JPY": "¥",
	}
	if s, ok := symbols[code]; ok {
		return s
	}
	return code + " "
}

// GenerateDedupeKey 生成确定性的事件去重键
func GenerateDedupeKey(siteID uint, definitionVersion int, eventType, itemKey, beforeFP, afterFP string) string {
	canonical := strings.Join([]string{
		fmt.Sprintf("%d", siteID),
		fmt.Sprintf("%d", definitionVersion),
		eventType,
		itemKey,
		beforeFP,
		afterFP,
	}, "\x00")
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", sum[:])
}
