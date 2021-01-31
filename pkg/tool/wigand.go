package tool

import (
	"fmt"
	"strconv"
)

// WigantUintToFasality преобразует единый номер виганда из number в фасалити fasality и число num.
// Если номер равен 0, возаращается результаты тоже в виде 0
func WigantUintToFasality(number uint) (fasality uint, num uint) {
	if number == 0 {
		return 0, 0
	}
	fasality = number >> 16
	num = number>>16<<16 ^ number
	return fasality, num
}

// WigantFasalityToUint преобразование фасалити fasality и номера number виганда в единое число
func WigantFasalityToUint(fasality, number uint) uint {
	s, _ := strconv.ParseUint(fmt.Sprintf("%b%016b", fasality, number), 2, 32)
	return uint(s)
}
