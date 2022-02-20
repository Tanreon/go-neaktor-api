package neaktor_api

import (
	"fmt"
	"strings"
)

func parseErrorCode(code string, message string) error {
	if strings.EqualFold(code, ErrCode403.Error()) {
		return fmt.Errorf("%w: %s", ErrCode403, message)
	}
	if strings.EqualFold(code, ErrCode404.Error()) {
		return fmt.Errorf("%w: %s", ErrCode404, message)
	}
	if strings.EqualFold(code, ErrCode429.Error()) {
		return fmt.Errorf("%w: %s", ErrCode429, message)
	}
	if strings.EqualFold(code, ErrCode422.Error()) {
		return fmt.Errorf("%w: %s", ErrCode422, message)
	}
	if strings.EqualFold(code, ErrCode500.Error()) {
		return fmt.Errorf("%w: %s", ErrCode500, message)
	}

	return ErrCodeUnknown
}
