using FluentValidation;
using System.Text.RegularExpressions;

namespace WaifuApi.Application.Features.GetImages;

public partial class GetImagesQueryValidator : AbstractValidator<GetImagesQuery>
{
    public GetImagesQueryValidator()
    {
        RuleForEach(x => x.IncludedArtists).Must(BeAValidLong).WithMessage("Artist IDs must be valid numbers.");
        RuleForEach(x => x.ExcludedArtists).Must(BeAValidLong).WithMessage("Artist IDs must be valid numbers.");
        RuleForEach(x => x.IncludedIds).Must(BeAValidLong).WithMessage("Image IDs must be valid numbers.");
        RuleForEach(x => x.ExcludedIds).Must(BeAValidLong).WithMessage("Image IDs must be valid numbers.");

        RuleForEach(x => x.IncludedTags).Must(BeAValidTag).WithMessage("Tags must be a valid ID or a slug (lowercase letters, numbers, and hyphens).");
        RuleForEach(x => x.ExcludedTags).Must(BeAValidTag).WithMessage("Tags must be a valid ID or a slug (lowercase letters, numbers, and hyphens).");
    }

    private bool BeAValidLong(string value)
    {
        return long.TryParse(value, out _);
    }

    private bool BeAValidTag(string value)
    {
        if (long.TryParse(value, out _))
        {
            return true;
        }
        
        return SlugRegex().IsMatch(value);
    }

    [GeneratedRegex("^[a-z0-9-]+$")]
    private static partial Regex SlugRegex();
}