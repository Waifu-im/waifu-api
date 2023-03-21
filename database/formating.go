package database

import (
	"fmt"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/lib/pq"
	"strings"
)

// Some useful func to generate the search/fav query inside queries.go.

func FormatComparator(field string, value string) string {
	if value == "" {
		return ""
	}
	return "and Images." + field + value + " "
}

func FormatNsfw(isNfw string) string {
	s := "Images.is_nsfw "
	if isNfw == constants.True || isNfw == constants.Null {
		return s
	}
	return "not " + s
}

func FormatNsfwTags(isNsfw string, includedTags []string, parameters *[]any) string {
	var s string
	if len(includedTags) > 0 && isNsfw != constants.Null {
		s += "and ("
		if isNsfw != constants.Null {
			s += FormatNsfw(isNsfw) + " or "
		}
		s += fmt.Sprintf("EXISTS(SELECT name from Tags T2 WHERE T2.is_nsfw AND T2.name ILIKE ANY($%v))) ", len(*parameters)+1)
		*parameters = append(*parameters, pq.Array(includedTags))
	} else if isNsfw != constants.Null {
		s = "and " + FormatNsfw(isNsfw)
	}
	return s
}
func FormatGif(gif string) string {
	s := "Images.extension='.gif' "
	if gif == constants.True {
		return "and " + s
	} else if gif == constants.False {
		return "and not " + s
	} else {
		return ""
	}
}

func FormatOrientation(orientation string) string {
	if orientation == constants.Random {
		return ""
	}
	var s string
	if orientation == constants.Landscape {
		s = " > "
	} else {
		s = " < "
	}
	return "and Images.width" + s + "Images.height "
}

func FormatOrderBy(orderBy string, prefix string, randomEnabled bool) string {
	if orderBy == constants.Random {
		if randomEnabled {
			return "ORDER BY RANDOM()"
		}
		return ""
	}
	return "ORDER BY " + prefix + strings.ToLower(orderBy) + " DESC "
}
