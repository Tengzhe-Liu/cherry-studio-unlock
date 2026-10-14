// Package store 负责读写 Cherry Studio 的 SQLite 数据。
package store

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// base62Digits 与 Cherry Studio 内部使用的 fractional-indexing@3.2.0 完全一致。
//
// Cherry Studio 的 order_key 列由该算法生成，我们写入的键必须与 App 自己生成的
// 键逐字节可比，否则新 Agent 的排序会错位，或与后续新建的 Agent 抢同一个键。
// 因此这里是对原实现的逐行移植，而不是"差不多能用"的重写。
const base62Digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func strPtr(s string) *string { return &s }

func charAt(s string, i int) string {
	if i >= 0 && i < len(s) {
		return s[i : i+1]
	}
	return ""
}

func lastByte(s string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1:]
}

func sliceFrom(s string, i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}
	return s[i:]
}

func digitIndex(digits, c string) int {
	if c == "" {
		return -1
	}
	return strings.Index(digits, c)
}

// midpoint 等价于 fractional-indexing 的 midpoint(a, b, digits)。
// a 永不为 nil；b 为 nil 表示"到末尾为止"。
func midpoint(a string, b *string, digits string) (string, error) {
	zero := digits[:1]

	if b != nil && a >= *b {
		return "", fmt.Errorf("midpoint: %q >= %q", a, *b)
	}
	if lastByte(a) == zero || (b != nil && *b != "" && lastByte(*b) == zero) {
		return "", errors.New("midpoint: trailing zero")
	}

	if b != nil && *b != "" {
		n := 0
		for {
			av := charAt(a, n)
			if av == "" {
				av = zero
			}
			bv := charAt(*b, n)
			if bv == "" || av != bv {
				break
			}
			n++
		}
		if n > 0 {
			rest, err := midpoint(sliceFrom(a, n), strPtr(sliceFrom(*b, n)), digits)
			if err != nil {
				return "", err
			}
			return (*b)[:n] + rest, nil
		}
	}

	digitA := 0
	if a != "" {
		digitA = digitIndex(digits, charAt(a, 0))
	}
	// 注意：b 非 nil 但为空串时，原实现在这里取到 -1，必须保持一致。
	digitB := len(digits)
	if b != nil {
		digitB = digitIndex(digits, charAt(*b, 0))
	}

	if digitB-digitA > 1 {
		return charAt(digits, int(math.Round(0.5*float64(digitA+digitB)))), nil
	} else if b != nil && len(*b) > 1 {
		return (*b)[:1], nil
	}

	rest, err := midpoint(sliceFrom(a, 1), nil, digits)
	if err != nil {
		return "", err
	}
	return charAt(digits, digitA) + rest, nil
}

func getIntegerLength(head string) (int, error) {
	switch {
	case head >= "a" && head <= "z":
		return int(head[0]-'a') + 2, nil
	case head >= "A" && head <= "Z":
		return int('Z'-head[0]) + 2, nil
	default:
		return 0, fmt.Errorf("invalid order key head: %q", head)
	}
}

func getIntegerPart(key string) (string, error) {
	if key == "" {
		return "", errors.New("invalid order key: empty")
	}
	l, err := getIntegerLength(key[:1])
	if err != nil {
		return "", err
	}
	if l > len(key) {
		return "", fmt.Errorf("invalid order key: %s", key)
	}
	return key[:l], nil
}

func validateInteger(x, digits string) error {
	if x == "" {
		return errors.New("invalid integer part of order key: empty")
	}
	l, err := getIntegerLength(x[:1])
	if err != nil {
		return err
	}
	if len(x) != l {
		return fmt.Errorf("invalid integer part of order key: %s", x)
	}
	return nil
}

func validateOrderKey(key, digits string) error {
	if key == "A"+strings.Repeat(digits[:1], 26) {
		return fmt.Errorf("invalid order key: %s", key)
	}
	i, err := getIntegerPart(key)
	if err != nil {
		return err
	}
	if lastByte(sliceFrom(key, len(i))) == digits[:1] {
		return fmt.Errorf("invalid order key: %s", key)
	}
	return nil
}

// incrementInteger 返回 (结果, 是否成功)。第二个返回值为 false 表示已无法再进位（原实现返回 null）。
func incrementInteger(x, digits string) (string, bool, error) {
	if err := validateInteger(x, digits); err != nil {
		return "", false, err
	}
	head := x[:1]
	digs := []byte(x[1:])
	carry := true
	for i := len(digs) - 1; carry && i >= 0; i-- {
		d := strings.IndexByte(digits, digs[i]) + 1
		if d == len(digits) {
			digs[i] = digits[0]
		} else {
			digs[i] = digits[d]
			carry = false
		}
	}
	if carry {
		switch head {
		case "Z":
			return "a" + digits[:1], true, nil
		case "z":
			return "", false, nil
		}
		h := string(head[0] + 1)
		if h > "a" {
			digs = append(digs, digits[0])
		} else if len(digs) > 0 {
			digs = digs[:len(digs)-1]
		}
		return h + string(digs), true, nil
	}
	return head + string(digs), true, nil
}

// decrementInteger 返回 (结果, 是否成功)。第二个返回值为 false 表示已无法再退位（原实现返回 null）。
func decrementInteger(x, digits string) (string, bool, error) {
	if err := validateInteger(x, digits); err != nil {
		return "", false, err
	}
	head := x[:1]
	digs := []byte(x[1:])
	borrow := true
	for i := len(digs) - 1; borrow && i >= 0; i-- {
		d := strings.IndexByte(digits, digs[i]) - 1
		if d == -1 {
			digs[i] = digits[len(digits)-1]
		} else {
			digs[i] = digits[d]
			borrow = false
		}
	}
	if borrow {
		switch head {
		case "a":
			return "Z" + digits[len(digits)-1:], true, nil
		case "A":
			return "", false, nil
		}
		h := string(head[0] - 1)
		if h < "Z" {
			digs = append(digs, digits[len(digits)-1])
		} else if len(digs) > 0 {
			digs = digs[:len(digs)-1]
		}
		return h + string(digs), true, nil
	}
	return head + string(digs), true, nil
}

// GenerateKeyBetween 返回一个严格位于 a 与 b 之间的排序键。
// a 为 nil 表示插入到最前，b 为 nil 表示追加到最后。
func GenerateKeyBetween(a, b *string) (string, error) {
	return generateKeyBetweenDigits(a, b, base62Digits)
}

func generateKeyBetweenDigits(a, b *string, digits string) (string, error) {
	if a != nil {
		if err := validateOrderKey(*a, digits); err != nil {
			return "", err
		}
	}
	if b != nil {
		if err := validateOrderKey(*b, digits); err != nil {
			return "", err
		}
	}
	if a != nil && b != nil && *a >= *b {
		return "", fmt.Errorf("order keys out of range: %q >= %q", *a, *b)
	}

	if a == nil {
		if b == nil {
			return "a" + digits[:1], nil
		}
		ib, err := getIntegerPart(*b)
		if err != nil {
			return "", err
		}
		fb := sliceFrom(*b, len(ib))
		if ib == "A"+strings.Repeat(digits[:1], 26) {
			rest, err := midpoint("", strPtr(fb), digits)
			if err != nil {
				return "", err
			}
			return ib + rest, nil
		}
		if ib < *b {
			return ib, nil
		}
		res, ok, err := decrementInteger(ib, digits)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", errors.New("cannot decrement any more")
		}
		return res, nil
	}

	if b == nil {
		ia, err := getIntegerPart(*a)
		if err != nil {
			return "", err
		}
		fa := sliceFrom(*a, len(ia))
		i, ok, err := incrementInteger(ia, digits)
		if err != nil {
			return "", err
		}
		if !ok {
			rest, err := midpoint(fa, nil, digits)
			if err != nil {
				return "", err
			}
			return ia + rest, nil
		}
		return i, nil
	}

	ia, err := getIntegerPart(*a)
	if err != nil {
		return "", err
	}
	fa := sliceFrom(*a, len(ia))
	ib, err := getIntegerPart(*b)
	if err != nil {
		return "", err
	}
	fb := sliceFrom(*b, len(ib))

	if ia == ib {
		rest, err := midpoint(fa, strPtr(fb), digits)
		if err != nil {
			return "", err
		}
		return ia + rest, nil
	}
	i, ok, err := incrementInteger(ia, digits)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("cannot increment any more")
	}
	if i < *b {
		return i, nil
	}
	rest, err := midpoint(fa, nil, digits)
	if err != nil {
		return "", err
	}
	return ia + rest, nil
}

// NextOrderKey 是常见用法的封装：追加到末尾（b 为 nil）。
// maxKey 为 nil 或空串时返回第一个键 "a0"。
func NextOrderKey(maxKey *string) (string, error) {
	if maxKey != nil && *maxKey == "" {
		maxKey = nil
	}
	return GenerateKeyBetween(maxKey, nil)
}
