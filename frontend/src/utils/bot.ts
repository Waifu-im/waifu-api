const BOT_PATTERNS = /Googlebot|Bingbot|Slurp|DuckDuckBot|Baiduspider|YandexBot|facebookexternalhit|Twitterbot|Discordbot|WhatsApp|TelegramBot|LinkedInBot/i;

export function isBot(): boolean {
  return BOT_PATTERNS.test(navigator.userAgent);
}
