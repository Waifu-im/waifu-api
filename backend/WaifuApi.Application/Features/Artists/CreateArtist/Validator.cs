using FluentValidation;

namespace WaifuApi.Application.Features.Artists.CreateArtist;

public class CreateArtistCommandValidator : AbstractValidator<CreateArtistCommand>
{
    public CreateArtistCommandValidator()
    {
        RuleFor(x => x.Name)
            .NotEmpty().WithMessage("Name is required.")
            .MaximumLength(30).WithMessage("Name must not exceed 30 characters.");

        RuleFor(x => x.Patreon)
            .MaximumLength(200).WithMessage("Patreon link must not exceed 200 characters.");
            
        RuleFor(x => x.Pixiv)
            .MaximumLength(200).WithMessage("Pixiv link must not exceed 200 characters.");
            
        RuleFor(x => x.Twitter)
            .MaximumLength(200).WithMessage("Twitter link must not exceed 200 characters.");
            
        RuleFor(x => x.DeviantArt)
            .MaximumLength(200).WithMessage("DeviantArt link must not exceed 200 characters.");
    }
}
