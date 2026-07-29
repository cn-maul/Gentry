package monitor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	currencySymbols = map[string]string{
		"¥": "CNY", "￥": "CNY", "HK$": "HKD", "NT$": "TWD", "A$": "AUD",
		"C$": "CAD", "S$": "SGD", "$": "USD", "€": "EUR",
		"£": "GBP", "₩": "KRW", "₽": "RUB", "₹": "INR", "R$": "BRL", "円": "JPY",
	}
	currencyCodes = map[string]string{
		"cny": "CNY", "usd": "USD", "eur": "EUR", "gbp": "GBP",
		"hkd": "HKD", "twd": "TWD", "krw": "KRW", "rub": "RUB",
		"inr": "INR", "brl": "BRL", "aud": "AUD", "cad": "CAD",
		"sgd": "SGD", "jpy": "JPY",
	}
	currencyExponents = map[string]int{
		"JPY": 0, "KRW": 0,
		"BHD": 3, "JOD": 3, "KWD": 3,
	}
	numericTokenRegex = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
)

// NormalizeField 根据字段类型规范化值
func NormalizeField(value string, dataType string) TypedValue {
	switch dataType {
	case "money":
		return normalizeMoney(value)
	case "decimal":
		return normalizeDecimal(value)
	case "integer":
		return normalizeInteger(value)
	case "url":
		return normalizeURL(value)
	default:
		return TypedValue{Value: strings.TrimSpace(value), DataType: "text", Valid: true}
	}
}

// normalizeMoney 解析金额字符串
// 支持: ¥1,299.00, $99.9, 1299.00元, 1299 等
func normalizeMoney(value string) TypedValue {
	value = strings.TrimSpace(value)
	if value == "" {
		return TypedValue{DataType: "money", Valid: false}
	}

	currency := detectCurrency(value)
	exponent := currencyExponent(currency)
	minor, err := parseMinorAmount(value, exponent)
	if err != nil {
		return TypedValue{DataType: "money", Valid: false}
	}

	return TypedValue{
		Value:    currency + formatMinorNumber(minor, exponent),
		DataType: "money",
		Minor:    minor,
		Currency: currency,
		Valid:    true,
	}
}

func detectCurrency(s string) string {
	// 按长度降序匹配，避免把 HK$ 先识别成 USD。
	type symEntry struct{ sym, code string }
	var symbols []symEntry
	for sym, code := range currencySymbols {
		symbols = append(symbols, symEntry{sym, code})
	}
	sort.Slice(symbols, func(i, j int) bool { return len(symbols[i].sym) > len(symbols[j].sym) })
	for _, entry := range symbols {
		if strings.Contains(s, entry.sym) {
			return entry.code
		}
	}
	// 尝试匹配后缀代码
	lower := strings.ToLower(s)
	for code, normalized := range currencyCodes {
		if strings.Contains(lower, code) {
			return normalized
		}
	}
	// 默认为 CNY
	return "CNY"
}

func normalizeDecimal(value string) TypedValue {
	value = strings.TrimSpace(value)
	if value == "" {
		return TypedValue{DataType: "decimal", Valid: false}
	}
	minor, err := parseMinorAmount(value, 2)
	if err != nil {
		return TypedValue{DataType: "decimal", Valid: false}
	}
	return TypedValue{Value: formatMinorNumber(minor, 2), DataType: "decimal", Minor: minor, Valid: true}
}

func normalizeInteger(value string) TypedValue {
	value = strings.TrimSpace(value)
	if value == "" {
		return TypedValue{DataType: "integer", Valid: false}
	}
	parsed, err := parseMinorAmount(value, 0)
	if err != nil {
		return TypedValue{DataType: "integer", Valid: false}
	}
	return TypedValue{Value: fmt.Sprintf("%d", parsed), DataType: "integer", Minor: parsed, Valid: true}
}

func currencyExponent(currency string) int {
	if exponent, ok := currencyExponents[strings.ToUpper(currency)]; ok {
		return exponent
	}
	return 2
}

// parseMinorAmount 使用十进制定点方式解析金额，支持 1,299.99 和 1.299,99。
func parseMinorAmount(value string, exponent int) (int64, error) {
	trimmed := strings.TrimSpace(value)
	tokens := numericTokenRegex.FindAllString(trimmed, -1)
	if len(tokens) != 1 || strings.Contains(trimmed, "-") {
		return 0, fmt.Errorf("invalid non-negative amount")
	}
	cleaned := tokens[0]

	normalized, err := normalizeAmountSeparators(cleaned, exponent)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(normalized, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount")
	}
	whole := parts[0]
	if whole == "" {
		whole = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if exponent == 0 && fraction != "" {
		return 0, fmt.Errorf("currency does not support fractional units")
	}
	if len(fraction) > exponent {
		return 0, fmt.Errorf("too many fractional digits")
	}
	fraction += strings.Repeat("0", exponent-len(fraction))
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return 0, nil
	}
	minor, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount out of range: %w", err)
	}
	return minor, nil
}

func parseThresholdMinor(value, currency string) (int64, error) {
	return parseConfiguredMinor(value, currency, true)
}

// parseTargetMinor 将“价格 <= X”的目标值换算为币种最小单位。
// 当配置精度高于币种精度时必须向下取整，避免把 90.5 JPY 错当成 91 JPY。
func parseTargetMinor(value, currency string) (int64, error) {
	return parseConfiguredMinor(value, currency, false)
}

func parseConfiguredMinor(value, currency string, roundUp bool) (int64, error) {
	const thresholdPrecision = 3
	precise, err := parseMinorAmount(value, thresholdPrecision)
	if err != nil {
		return 0, err
	}
	exponent := currencyExponent(currency)
	if exponent >= thresholdPrecision {
		return precise, nil
	}
	divisor := int64(1)
	for i := exponent; i < thresholdPrecision; i++ {
		divisor *= 10
	}
	if roundUp {
		// 最低降价金额不能因精度收缩而变小。
		return (precise + divisor - 1) / divisor, nil
	}
	// 到价条件是 price <= target，向下取整才能保持原条件边界。
	return precise / divisor, nil
}

func normalizeAmountSeparators(value string, exponent int) (string, error) {
	dotCount := strings.Count(value, ".")
	commaCount := strings.Count(value, ",")

	if dotCount > 0 && commaCount > 0 {
		decimalSep := "."
		groupSep := ","
		if strings.LastIndex(value, ",") > strings.LastIndex(value, ".") {
			decimalSep, groupSep = ",", "."
		}
		value = strings.ReplaceAll(value, groupSep, "")
		if strings.Count(value, decimalSep) != 1 {
			return "", fmt.Errorf("invalid decimal separators")
		}
		return strings.Replace(value, decimalSep, ".", 1), nil
	}

	separator := ""
	count := 0
	if dotCount > 0 {
		separator, count = ".", dotCount
	} else if commaCount > 0 {
		separator, count = ",", commaCount
	}
	if separator == "" {
		return value, nil
	}

	parts := strings.Split(value, separator)
	if count > 1 {
		for _, group := range parts[1:] {
			if len(group) != 3 {
				return "", fmt.Errorf("invalid grouped amount")
			}
		}
		return strings.Join(parts, ""), nil
	}

	before, after := parts[0], parts[1]
	if after == "" {
		return "", fmt.Errorf("missing fractional digits")
	}
	// 单个分隔符后有三位数字时优先按千分位解释。
	if len(after) == 3 && len(before) > 0 {
		return before + after, nil
	}
	if exponent == 0 || len(after) > exponent {
		return "", fmt.Errorf("invalid fractional precision")
	}
	return before + "." + after, nil
}

func formatMinorNumber(minor int64, exponent int) string {
	if exponent <= 0 {
		return strconv.FormatInt(minor, 10)
	}
	scale := int64(1)
	for i := 0; i < exponent; i++ {
		scale *= 10
	}
	return fmt.Sprintf("%d.%0*d", minor/scale, exponent, minor%scale)
}

func normalizeURL(value string) TypedValue {
	value = strings.TrimSpace(value)
	if value == "" {
		return TypedValue{DataType: "url", Valid: false}
	}
	return TypedValue{Value: value, DataType: "url", Valid: true}
}

// GenerateItemKey 生成稳定条目标识
func GenerateItemKey(obs ExtractResult, identity IdentityConfig, sourceURL string) string {
	if identity.Source == "source_url" {
		return strings.TrimSpace(sourceURL)
	}
	if identity.Field != "" {
		if v, ok := obs[identity.Field]; ok {
			return strings.TrimSpace(fmt.Sprintf("%v", v))
		}
	}
	if len(identity.Fields) > 0 {
		var parts []string
		for _, f := range identity.Fields {
			v, ok := obs[f]
			if !ok {
				return ""
			}
			part := strings.TrimSpace(fmt.Sprintf("%v", v))
			if part == "" {
				return ""
			}
			parts = append(parts, part)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "|")
		}
	}
	return ""
}

// ComputeFingerprint 计算字段哈希用于快速比较
func ComputeFingerprint(fields map[string]interface{}) string {
	data, err := json.Marshal(fields)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", fields))
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}
