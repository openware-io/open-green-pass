package domain

import "testing"

func TestModelBindingValidate(t *testing.T) {
	if err := (ModelBinding{Model: "gpt", Provider: "openai"}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, binding := range []ModelBinding{{Provider: "openai"}, {Model: "gpt"}} {
		if err := binding.Validate(); err == nil {
			t.Fatalf("binding=%+v should be invalid", binding)
		}
	}
}
