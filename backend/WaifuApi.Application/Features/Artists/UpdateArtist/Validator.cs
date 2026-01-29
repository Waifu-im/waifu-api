using FluentValidation;

namespace WaifuApi.Application.Features.Artists.UpdateArtist;

public class UpdateArtistCommandValidator : AbstractValidator<UpdateArtistCommand>
{
    public UpdateArtistCommandValidator()
    {
        RuleFor(x => x.Id)
            .GreaterThan(0).WithMessage("Invalid Artist ID.");

        RuleFor(x => x.Name)
            .NotEmpty().WithMessage("Name is required.")
            .MaximumLength(30).WithMessage("Name must not exceed 30 characters.");

        RuleFor(x => x.Patreon)
            .MaximumLength(200).WithMessage("Patreon link must not exceed 200 characters.")
            .Must(BeAValidUrl).When(x => !string.IsNullOrEmpty(x.Patreon))
            .WithMessage("Patreon link must be a valid URL.");
            
        RuleFor(x => x.Pixiv)
            .MaximumLength(200).WithMessage("Pixiv link must not exceed 200 characters.")
            .Must(BeAValidUrl).When(x => !string.IsNullOrEmpty(x.Pixiv))
            .WithMessage("Pixiv link must be a valid URL.");
            
        RuleFor(x => x.Twitter)
            .MaximumLength(200).WithMessage("Twitter link must not exceed 200 characters.")
            .Must(BeAValidUrl).When(x => !string.IsNullOrEmpty(x.Twitter))
            .WithMessage("Twitter link must be a valid URL.");
            
        RuleFor(x => x.DeviantArt)
            .MaximumLength(200).WithMessage("DeviantArt link must not exceed 200 characters.")
            .Must(BeAValidUrl).When(x => !string.IsNullOrEmpty(x.DeviantArt))
            .WithMessage("DeviantArt link must be a valid URL.");
    }

    private bool BeAValidUrl(string url)
    {
        return Uri.TryCreate(url, UriKind.Absolute, out _);
    }
}
