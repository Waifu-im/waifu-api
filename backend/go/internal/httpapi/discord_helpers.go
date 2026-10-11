package httpapi

import (
	"errors"
	"time"

	"waifuapi/internal/discord"
)

// asDiscordError unwraps a Discord API error.
//
// It is a thin wrapper so the httpapi package does not need errors.As calls
// repeated in more than one place.
func asDiscordError(err error, target **discord.APIError) bool {
	return errors.As(err, target)
}

// timeNow returns the current time, isolated so expiry arithmetic can be
// reasoned about in one place.
func timeNow() time.Time { return time.Now() }
