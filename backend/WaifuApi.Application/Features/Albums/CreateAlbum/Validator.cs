using FluentValidation;

namespace WaifuApi.Application.Features.Albums.CreateAlbum;

public class CreateAlbumCommandValidator : AbstractValidator<CreateAlbumCommand>
{
    public CreateAlbumCommandValidator()
    {
        RuleFor(x => x.UserId)
            .GreaterThan(0).WithMessage("Invalid User ID.");

        RuleFor(x => x.Name)
            .NotEmpty().WithMessage("Name is required.")
            .MaximumLength(30).WithMessage("Name must not exceed 30 characters.");

        RuleFor(x => x.Description)
            .MaximumLength(150).WithMessage("Description must not exceed 150 characters.");
    }
}
