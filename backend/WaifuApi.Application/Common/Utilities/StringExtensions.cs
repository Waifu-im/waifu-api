using System.Globalization;
using System.Text;
using System.Text.RegularExpressions;

namespace WaifuApi.Application.Common.Utilities;

public static class StringExtensions
{
    public static string? ToNullIfEmpty(this string? value)
    {
        return string.IsNullOrWhiteSpace(value) ? null : value.Trim();
    }

    public static string ToSlug(this string text)
    {
        if (string.IsNullOrWhiteSpace(text)) return string.Empty;

        var normalizedString = text.Normalize(NormalizationForm.FormD);
        var stringBuilder = new StringBuilder();

        foreach (var c in normalizedString)
        {
            var unicodeCategory = CharUnicodeInfo.GetUnicodeCategory(c);
            if (unicodeCategory != UnicodeCategory.NonSpacingMark)
            {
                stringBuilder.Append(c);
            }
        }

        var cleanText = stringBuilder.ToString().Normalize(NormalizationForm.FormC).ToLowerInvariant();
        cleanText = Regex.Replace(cleanText, @"[^a-z0-9\s-]", "");
        cleanText = Regex.Replace(cleanText, @"\s+", "-").Trim('-');

        return cleanText;
    }
}