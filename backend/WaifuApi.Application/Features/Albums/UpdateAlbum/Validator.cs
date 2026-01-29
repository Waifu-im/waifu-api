using FluentValidation;

namespace WaifuApi.Application.Features.Albums.UpdateAlbum;

public class UpdateAlbumCommandValidator : AbstractValidator<UpdateAlbumCommand>
{
    public UpdateAlbumCommandValidator()
    {
        RuleFor(x => x.AlbumId)
            .GreaterThan(0).WithMessage("Invalid Album ID.");
            
        RuleFor(x => x.UserId)
            .GreaterThan(0).WithMessage("Invalid User ID.");

        RuleFor(x => x.Name)
            .NotEmpty().WithMessage("Name is required.")
            .MaximumLength(100).WithMessage("Name must not exceed 100 characters.");

        RuleFor(x => x.Description)
            .MaximumLength(500).WithMessage("Description must not exceed 500 characters.");
    }
}
