package image

import (
	"fmt"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/labstack/echo/v4"
	"golang.org/x/exp/slices"
	"regexp"
	"strings"
)

const RegexRule = `^[a-zA-Z0-9-]+$`
const ComparatorRegexRule = `^(>=|<=|=|!=|>|<)[0-9]+$`

func getThreeState() []string {
	return []string{constants.Null, constants.True, constants.False}
}

func getOrderBy(includeLikedAt bool) []string {
	arr := []string{constants.Favorites, constants.UploadedAt, constants.Random}
	if includeLikedAt {
		arr = append(arr, constants.LikedAt)
	}
	return arr
}

func getOrientation() []string {
	return []string{constants.Portrait, constants.Landscape, constants.Random}
}

func matchRegex(value string, rule string) bool {
	reg := regexp.MustCompile(rule).MatchString(value)
	return reg
}

func isThreeState(value string) bool {
	return slices.Contains(getThreeState(), strings.ToLower(value))
}

func isOrderBy(value string, favMode bool) bool {
	return slices.Contains(getOrderBy(favMode), strings.ToUpper(value))
}

func isOrientation(value string) bool {
	return slices.Contains(getOrientation(), strings.ToUpper(value))
}

func regexError(rule string) string {
	return "field value must match regex : " + rule
}

func QueryParamsBinder(
	favRoute bool,
	c echo.Context,
	isNsfw *string,
	includedTags *[]string,
	excludedTags *[]string,
	includedFiles *[]string,
	excludedFiles *[]string,
	gif *string,
	orderBy *string,
	orientation *string,
	limit *int,
	full *bool,
	width *string,
	height *string,
	byteSize *string,
) error {
	threeStateError := fmt.Sprintf("field value must be one of %v", strings.Join(getThreeState(), ", "))
	orderByError := fmt.Sprintf("field value must be one of %v", strings.Join(getOrderBy(false), ", "))
	orientationError := fmt.Sprintf("field value must be one of %v", strings.Join(getOrientation(), ", "))

	return echo.QueryParamsBinder(c).
		CustomFunc("is_nsfw", func(values []string) []error {
			if len(values) == 0 {
				return nil
			}
			if isThreeState(values[0]) {
				*isNsfw = strings.ToLower(values[0])
				return nil
			}
			return []error{echo.NewBindingError("is_nsfw", values[0:1], threeStateError, nil)}
		}).
		CustomFunc("included_tags", func(values []string) []error {
			for _, val := range values {
				if matchRegex(val, RegexRule) {
					if !slices.Contains(*includedTags, val) {
						*includedTags = append(*includedTags, val)
					}
					continue
				}
				return []error{echo.NewBindingError("included_tags", []string{val}, regexError(RegexRule), nil)}
			}
			return nil
		}).
		CustomFunc("excluded_tags", func(values []string) []error {
			for _, val := range values {
				if matchRegex(val, RegexRule) {
					if !slices.Contains(*excludedTags, val) {
						*excludedTags = append(*excludedTags, val)
					}
					continue
				}
				return []error{echo.NewBindingError("excluded_tags", []string{val}, regexError(RegexRule), nil)}
			}
			return nil
		}).
		CustomFunc("included_files", func(values []string) []error {
			for _, val := range values {
				if matchRegex(val, RegexRule) {
					if !slices.Contains(*includedFiles, val) {
						*includedFiles = append(*includedFiles, val)
					}
					continue
				}
				return []error{echo.NewBindingError("included_files", []string{val}, regexError(RegexRule), nil)}
			}
			return nil
		}).
		CustomFunc("excluded_files", func(values []string) []error {
			for _, val := range values {
				if matchRegex(val, RegexRule) {
					if !slices.Contains(*excludedFiles, val) {
						*excludedFiles = append(*excludedFiles, val)
					}
					continue
				}
				return []error{echo.NewBindingError("excluded_files", []string{val}, regexError(RegexRule), nil)}
			}
			return nil
		}).
		CustomFunc("gif", func(values []string) []error {
			if len(values) == 0 {
				return nil
			}
			if isThreeState(values[0]) {
				*gif = strings.ToLower(values[0])
				return nil
			}
			return []error{echo.NewBindingError("gif", values[0:1], threeStateError, nil)}
		}).
		CustomFunc("order_by", func(values []string) []error {
			if len(values) == 0 {
				return nil
			}
			if isOrderBy(values[0], favRoute) {
				*orderBy = strings.ToUpper(values[0])
				return nil
			}
			return []error{echo.NewBindingError("order_by", values[0:1], orderByError, nil)}
		}).
		CustomFunc("orientation", func(values []string) []error {
			if len(values) == 0 {
				return nil
			}
			if isOrientation(values[0]) {
				*orientation = strings.ToUpper(values[0])
				return nil
			}
			return []error{echo.NewBindingError("orientation", values[0:1], orientationError, nil)}
		}).
		CustomFunc("width", func(values []string) []error {
			if len(values) == 0 {
				return nil
			}
			if matchRegex(values[0], ComparatorRegexRule) {
				*width = values[0]
				return nil
			}
			return []error{echo.NewBindingError("width", values[0:1], regexError(ComparatorRegexRule), nil)}
		}).
		CustomFunc("height", func(values []string) []error {
			if len(values) == 0 {
				return nil
			}
			if matchRegex(values[0], ComparatorRegexRule) {
				*height = values[0]
				return nil
			}
			return []error{echo.NewBindingError("height", values[0:1], regexError(ComparatorRegexRule), nil)}
		}).
		CustomFunc("byte_size", func(values []string) []error {
			if len(values) == 0 {
				return nil
			}
			if matchRegex(values[0], ComparatorRegexRule) {
				*byteSize = values[0]
				return nil
			}
			return []error{echo.NewBindingError("byte_size", values[0:1], regexError(ComparatorRegexRule), nil)}
		}).
		Int("limit", limit).
		Bool("full", full).
		BindError()
}
