package afx

import (
	"encoding/json"
	"fmt"
)

func Bind[T any](input map[string]any) (T, error) {
	var out T
	b, err := json.Marshal(input)
	if err != nil {
		return out, fmt.Errorf("marshal reasoner input: %w", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("decode reasoner input: %w", err)
	}
	return out, nil
}

func Map(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func Decode[T any](v any) (T, error) {
	var out T
	b, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}
