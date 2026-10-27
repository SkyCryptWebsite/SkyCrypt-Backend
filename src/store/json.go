package store

import (
	"encoding/json"
	"errors"
)

// Small helpers for walking raw JSON without decoding it, so stored bodies keep every byte Hypixel sent.

var errBadJSON = errors.New("store: malformed JSON")

// skipSpace returns the index of the first non-whitespace byte at or after i.
func skipSpace(data []byte, i int) int {
	for i < len(data) {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// skipString returns the index just past the string whose opening quote is at i.
func skipString(data []byte, i int) (int, error) {
	for i++; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i + 1, nil
		}
	}
	return 0, errBadJSON
}

// skipValue returns the index just past the JSON value that starts at i (after any whitespace).
func skipValue(data []byte, i int) (int, error) {
	i = skipSpace(data, i)
	if i >= len(data) {
		return 0, errBadJSON
	}

	switch data[i] {
	case '"':
		return skipString(data, i)
	case '{', '[':
		depth := 0
		for ; i < len(data); i++ {
			switch data[i] {
			case '"':
				end, err := skipString(data, i)
				if err != nil {
					return 0, err
				}
				i = end - 1
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return i + 1, nil
				}
			}
		}
		return 0, errBadJSON
	default:
		start := i
		for i < len(data) {
			switch data[i] {
			case ',', '}', ']', ' ', '\t', '\n', '\r':
				if i == start {
					return 0, errBadJSON
				}
				return i, nil
			}
			i++
		}
		return i, nil
	}
}

type field struct {
	key    string
	rawKey []byte // the key as written, quotes included
	value  []byte
}

// objectFields splits a JSON object into its top-level fields. Values alias data.
func objectFields(data []byte) ([]field, error) {
	i := skipSpace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return nil, errBadJSON
	}

	var fields []field
	i = skipSpace(data, i+1)
	if i < len(data) && data[i] == '}' {
		return fields, nil
	}

	for i < len(data) {
		i = skipSpace(data, i)
		if i >= len(data) || data[i] != '"' {
			return nil, errBadJSON
		}
		keyEnd, err := skipString(data, i)
		if err != nil {
			return nil, err
		}
		var key string
		if err := json.Unmarshal(data[i:keyEnd], &key); err != nil {
			return nil, errBadJSON
		}
		rawKey := data[i:keyEnd]

		i = skipSpace(data, keyEnd)
		if i >= len(data) || data[i] != ':' {
			return nil, errBadJSON
		}
		valueStart := skipSpace(data, i+1)
		valueEnd, err := skipValue(data, valueStart)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field{key: key, rawKey: rawKey, value: data[valueStart:valueEnd]})

		i = skipSpace(data, valueEnd)
		if i >= len(data) {
			return nil, errBadJSON
		}
		if data[i] == '}' {
			return fields, nil
		}
		if data[i] != ',' {
			return nil, errBadJSON
		}
		i++
	}
	return nil, errBadJSON
}

// arrayValues splits a JSON array into its elements. Values alias data.
func arrayValues(data []byte) ([][]byte, error) {
	i := skipSpace(data, 0)
	if i >= len(data) || data[i] != '[' {
		return nil, errBadJSON
	}

	var values [][]byte
	i = skipSpace(data, i+1)
	if i < len(data) && data[i] == ']' {
		return values, nil
	}

	for i < len(data) {
		start := skipSpace(data, i)
		end, err := skipValue(data, start)
		if err != nil {
			return nil, err
		}
		values = append(values, data[start:end])

		i = skipSpace(data, end)
		if i >= len(data) {
			return nil, errBadJSON
		}
		if data[i] == ']' {
			return values, nil
		}
		if data[i] != ',' {
			return nil, errBadJSON
		}
		i++
	}
	return nil, errBadJSON
}
