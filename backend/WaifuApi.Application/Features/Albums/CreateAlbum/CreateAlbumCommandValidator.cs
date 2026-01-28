using FluentValidation;

namespace WaifuApi.Application.Features.Albums.CreateAlbum;

public class CreateAlbumCommandValidator : AbstractValidator<CreateAlbumCommand>
{
    public CreateAlbumCommandValidator()
    {
        RuleFor(v => v.Name)
            .NotEmpty().WithMessage("Name is required.")
            .MaximumLength(100).WithMessage("Name must not exceed 100 characters.");
            
        RuleFor(v => v.Description)
            .MaximumLength(500).WithMessage("Description must not exceed 500 characters.");
    }
}
