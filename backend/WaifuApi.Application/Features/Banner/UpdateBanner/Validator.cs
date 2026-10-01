using FluentValidation;

namespace WaifuApi.Application.Features.Banner.UpdateBanner;

public class UpdateBannerCommandValidator : AbstractValidator<UpdateBannerCommand>
{
    public const int MaxMessageLength = 500;

    public UpdateBannerCommandValidator()
    {
        RuleFor(x => x.Message)
            .NotNull().WithMessage("Message is required.")
            .MaximumLength(MaxMessageLength).WithMessage($"Message must not exceed {MaxMessageLength} characters.");

        RuleFor(x => x.Message)
            .Must(m => !string.IsNullOrWhiteSpace(m))
            .When(x => x.IsEnabled)
            .WithMessage("An enabled banner needs a message.");

        RuleFor(x => x.Variant)
            .IsInEnum().WithMessage("Invalid banner variant.");
    }
}
