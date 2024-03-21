package neaktor_api

import (
	"fmt"
	"strings"

	neturl "net/url"
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

func mustUrlJoinPath(base string, path ...string) string {
	url, err := neturl.JoinPath(base, path...)
	if err != nil {
		panic(err)
	}

	return url
}

func mustParseUrl(rawUrl string) *neturl.URL {
	parsedUrl, err := neturl.Parse(rawUrl)
	if err != nil {
		panic(err)
	}

	return parsedUrl
}
