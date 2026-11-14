package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const MaxPriceCents int64 = 99999999999

var pricePattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)

// PriceInput distinguishes an omitted field (preserve on update) from null/blank
// (clear). Decimal strings and JSON numbers are parsed without floating point.
type PriceInput struct {
	Set   bool
	Cents *int64
}

func (p PriceInput) IsZero() bool                 { return !p.Set }
func (p PriceInput) MarshalJSON() ([]byte, error) { return json.Marshal(PriceText(p.Cents)) }

func (p *PriceInput) UnmarshalJSON(raw []byte) error {
	*p = PriceInput{Set: true}
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		return nil
	}
	value := string(raw)
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	invalid := fmt.Errorf("%w: 价格应为 0~999999999.99 元，最多两位小数", ErrInvalid)
	if !pricePattern.MatchString(value) {
		return invalid
	}
	whole, fraction, _ := strings.Cut(value, ".")
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	if len(whole) > 9 {
		return invalid
	}
	if len(fraction) == 0 {
		fraction = "00"
	} else if len(fraction) == 1 {
		fraction += "0"
	}
	cents, err := strconv.ParseInt(whole+fraction, 10, 64)
	if err != nil || cents > MaxPriceCents {
		return invalid
	}
	p.Cents = &cents
	return nil
}

func PriceText(cents *int64) *string {
	if cents == nil {
		return nil
	}
	value := fmt.Sprintf("%d.%02d", *cents/100, *cents%100)
	return &value
}
