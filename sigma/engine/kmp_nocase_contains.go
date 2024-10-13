package engine

import (
	"bytes"
	"errors"
	"strings"
)

var lowers = func() (bs []byte) {
	bs = make([]byte, 256)
	for i := range bs {
		bs[i] = byte(i)
		if 'A' <= bs[i] && bs[i] <= 'Z' {
			bs[i] = bs[i] + 'a' - 'A'
		}
	}
	return
}()

func ToLower(c byte) byte {
	// if 'A' <= c && c <= 'Z' {
	// 	return c + 'a' - 'A'
	// }
	// return c
	return lowers[c]
}

func computePrefix(pattern string) ([]int, error) {
	// sanity check
	len_p := len(pattern)
	if len_p < 2 {
		if len_p == 0 {
			return nil, errors.New("'pattern' must contain at least one character")
		}
		return []int{-1}, nil
	}
	next := make([]int, len_p)
	next[0], next[1] = -1, 0

	pos, count := 2, 0
	for pos < len_p {
		if pattern[pos-1] == pattern[count] {
			count++
			next[pos] = count
			pos++
		} else {
			if count > 0 {
				count = next[count]
			} else {
				next[pos] = 0
				pos++
			}
		}
	}
	return next, nil
}

func kmpIndexNoCaseMaker(substr string) (f func(str string) int) {
	substr = strings.ToLower(substr)

	next, err := computePrefix(substr)
	if err != nil {
		return func(str string) int {
			return -1
		}
	}
	lsubstr := len(substr)

	return func(str string) int {
		// sanity check
		if len(str) < lsubstr {
			return -1
		}
		i, j := 0, 0
		for i+j < len(str) {
			if substr[j] == ToLower(str[i+j]) {
				if j == lsubstr-1 {
					return i
				}
				j++
			} else {
				i = i + j - next[j]
				if next[j] > -1 {
					j = next[j]
				} else {
					j = 0
				}
			}
		}
		return -1 //跳出循环，要么i已经达到最大值，不能匹配子串
	}
}

func kmpContainsMaker(substr string) (f func(str string) bool) {
	next, err := computePrefix(substr)
	if err != nil {
		return func(str string) bool {
			return false
		}
	}
	lsubstr := len(substr)

	return func(str string) bool {
		// sanity check
		if len(str) < lsubstr {
			return false
		}
		l := len(str)
		i, j := 0, 0
		_ = str[l-1]
		_ = substr[lsubstr-1]
		for i+j < l {
			if substr[j] == str[i+j] {
				if j == lsubstr-1 {
					return true
				}
				j++
			} else {
				i = i + j - next[j]
				if next[j] > -1 {
					j = next[j]
				} else {
					j = 0
				}
			}
		}
		return false //跳出循环，要么i已经达到最大值，不能匹配子串
	}
}

func kmpContainsNoCaseMaker(substr string) (f func(str string) bool) {
	substr = strings.ToLower(substr)

	next, err := computePrefix(substr)
	if err != nil {
		return func(str string) bool {
			return false
		}
	}
	lsubstr := len(substr)

	return func(str string) bool {
		// sanity check
		if len(str) < lsubstr {
			return false
		}
		l := len(str)
		i, j := 0, 0
		_ = str[l-1]
		_ = substr[lsubstr-1]
		for i+j < l {
			if substr[j] == ToLower(str[i+j]) {
				if j == lsubstr-1 {
					return true
				}
				j++
			} else {
				i = i + j - next[j]
				if next[j] > -1 {
					j = next[j]
				} else {
					j = 0
				}
			}
		}
		return false //跳出循环，要么i已经达到最大值，不能匹配子串
	}
}

func kmpContainsNoCaseMaker2(substr string) (f func(str string) bool) {
	substr = strings.ToLower(substr)

	next, err := computePrefix(substr)
	if err != nil {
		return func(str string) bool {
			return false
		}
	}
	lsubstr := len(substr)

	substrUp := strings.ToUpper(substr)
	substr2 := make([]byte, len(substr)*2)
	for i := 0; i < lsubstr; i++ {
		substr2[i*2] = substr[i]
		substr2[i*2+1] = substrUp[i]
	}
	substrAll := string(substr2)

	return func(str string) bool {
		// sanity check
		if len(str) < lsubstr {
			return false
		}
		l := len(str)
		i, j := 0, 0
		_ = str[l-1]
		_ = substrAll[lsubstr*2-1]
		for i+j < l {
			// if c := str[i+j]; (1>>(c^substrAll[j*2]))+(1>>(c^substrAll[j*2+1])) == 1 {
			if c := str[i+j]; c == substrAll[j*2] || c == substrAll[j*2+1] {
				if j == lsubstr-1 {
					return true
				}
				j++
			} else {
				i = i + j - next[j]
				if next[j] > -1 {
					j = next[j]
				} else {
					j = 0
				}
			}
		}
		return false //跳出循环，要么i已经达到最大值，不能匹配子串
	}
}

func kmpContainsNoCaseMaker3(substr string) (f func(str string) bool) {
	substr = strings.ToLower(substr)
	subbs := []byte(substr)

	bs := make([]byte, 10240)

	return func(str string) bool {

		for i := 0; i < len(str); i++ {
			bs[i] = ToLower(str[i])
		}
		return bytes.Index(bs[:len(str)], subbs) > 0
	}
}
