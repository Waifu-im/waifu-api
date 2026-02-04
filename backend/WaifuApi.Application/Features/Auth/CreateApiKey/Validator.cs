using FluentValidation;

namespace WaifuApi.Application.Features.Auth.CreateApiKey;

public class CreateApiKeyCommandValidator : AbstractValidator<CreateApiKeyCommand>
{
    public CreateApiKeyCommandValidator()
    {
        RuleFor(x => x.UserId)
            .GreaterThan(0).WithMessage("Invalid User ID.");

        RuleFor(x => x.Description)
            .NotEmpty().WithMessage("Description is required.")
            .MinimumLength(3).WithMessage("Description must be at least 3 characters long.")
            .MaximumLength(100).WithMessage("Description must not exceed 100 characters.");
    }
}
