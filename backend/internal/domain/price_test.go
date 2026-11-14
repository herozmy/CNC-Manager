package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestPriceInputExactDecimalsAndPresence(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		set       bool
	}{
		{`{}`, "", false}, {`{"price":null}`, "", true}, {`{"price":"  "}`, "", true},
		{`{"price":0}`, "0.00", true}, {`{"price":"0.29"}`, "0.29", true},
		{`{"price":12.3}`, "12.30", true}, {`{"price":" 0012.30 "}`, "12.30", true},
		{`{"price":"999999999.99"}`, "999999999.99", true},
	} {
		var in DrawingInput
		if err := json.Unmarshal([]byte(tc.raw), &in); err != nil {
			t.Fatal(tc.raw, err)
		}
		got := PriceText(in.Price.Cents)
		if in.Price.Set != tc.set || (tc.want == "" && got != nil) || (tc.want != "" && (got == nil || *got != tc.want)) {
			t.Fatalf("%s: %+v %v", tc.raw, in.Price, got)
		}
		encoded, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip DrawingInput
		if err := json.Unmarshal(encoded, &roundtrip); err != nil || roundtrip.Price.Set != tc.set {
			t.Fatalf("roundtrip %s %v", encoded, err)
		}
	}
}

func TestPriceRejectsInvalidWithoutRounding(t *testing.T) {
	for _, raw := range []string{`-0.01`, `"-0"`, `0.001`, `"12.345"`, `1e2`, `true`, `[]`, `{}`, `"NaN"`, `"1,000"`, `1000000000`, `"999999999999999999999.00"`} {
		var in DrawingInput
		if err := json.Unmarshal([]byte(`{"price":`+raw+`}`), &in); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}
