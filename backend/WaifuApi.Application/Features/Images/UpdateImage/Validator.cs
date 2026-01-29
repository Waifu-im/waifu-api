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
            .Must(BeAValidUrl).When(x => !string.IsNullOrEmpty(x.Source))
            .WithMessage("Source must be a valid URL.");
            
        RuleFor(x => x.UserId)
            .GreaterThan(0).When(x => x.UserId.HasValue).WithMessage("Invalid User ID.");
    }

    private bool BeAValidUrl(string url)
    {
        return Uri.TryCreate(url, UriKind.Absolute, out _);
    }
}