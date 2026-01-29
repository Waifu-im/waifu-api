using FluentValidation;

namespace WaifuApi.Application.Features.Images.UpdateImage;

public class UpdateImageCommandValidator : AbstractValidator<UpdateImageCommand>
{
    public UpdateImageCommandValidator()
    {
        RuleFor(x => x.Id)
            .GreaterThan(0).WithMessage("Invalid Image ID.");

        RuleFor(x => x.Source)
            .MaximumLength(500).WithMessage("Source URL must not exceed 500 characters.")
            .When(x => !string.IsNullOrEmpty(x.Source));
            
        RuleFor(x => x.UserId)
            .GreaterThan(0).When(x => x.UserId.HasValue).WithMessage("Invalid User ID.");
            
        RuleFor(x => x.ArtistId)
            .GreaterThan(0).When(x => x.ArtistId.HasValue).WithMessage("Invalid Artist ID.");
    }
}
