package image

import (
	"fmt"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/labstack/echo/v4"
	"golang.org/x/exp/slices"
	"regexp"
	"strings"
)

const RegexRule = `^[a-zA-Z0-9-]+$`

func getThreeState() []string {
	return []string{database.Null, database.True, database.False}
}

func getOrderBy(includeLikedAt bool) []string {
	arr := []string{database.Favorites, database.UploadedAt, database.Random}
	if includeLikedAt {
		arr = append(arr, database.LikedAt)
	}
	return arr
}

func getOrientation() []string {
	return []string{database.Portrait, database.Landscape}
}

func isSafeString(value string) bool {
	reg := regexp.MustCompile(RegexRule).MatchString(value)
	return reg
}

func isThreeState(value string) bool {
	return slices.Contains(getThreeState(), strings.ToLower(value))
}

func isOrderBy(value string, favMode bool) bool {
	if favMode {
		return slices.Contains(getOrderBy(true), strings.ToUpper(value))
	}
	return slices.Contains(getOrderBy(false), strings.ToUpper(value))
}

func isOrientation(value string) bool {
	return slices.Contains(getOrientation(), strings.ToUpper(value))
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
	many *bool,
	full *bool,
) error {
	threeStateError := fmt.Sprintf("field value must be one of %v", strings.Join(getThreeState(), ", "))
	regexError := "field value must match regex : " + RegexRule
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
				if isSafeString(val) {
					if !slices.Contains(*includedTags, val) {
						*includedTags = append(*includedTags, val)
					}
					continue
				}
				return []error{echo.NewBindingError("included_tags", []string{val}, regexError, nil)}
			}
			return nil
		}).
		CustomFunc("excluded_tags", func(values []string) []error {
			for _, val := range values {
				if isSafeString(val) {
					if !slices.Contains(*excludedTags, val) {
						*excludedTags = append(*excludedTags, val)
					}
					continue
				}
				return []error{echo.NewBindingError("excluded_tags", []string{val}, regexError, nil)}
			}
			return nil
		}).
		CustomFunc("included_files", func(values []string) []error {
			for _, val := range values {
				if isSafeString(val) {
					if !slices.Contains(*includedFiles, val) {
						*includedFiles = append(*includedFiles, val)
					}
					continue
				}
				return []error{echo.NewBindingError("included_files", []string{val}, regexError, nil)}
			}
			return nil
		}).
		CustomFunc("excluded_files", func(values []string) []error {
			for _, val := range values {
				if isSafeString(val) {
					if !slices.Contains(*excludedFiles, val) {
						*excludedFiles = append(*excludedFiles, val)
					}
					continue
				}
				return []error{echo.NewBindingError("excluded_files", []string{val}, regexError, nil)}
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
		Bool("many", many).
		Bool("full", full).
		BindError()
}
