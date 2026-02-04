using FluentValidation;

namespace WaifuApi.Application.Features.Auth.UpdateApiKey;

public class UpdateApiKeyCommandValidator : AbstractValidator<UpdateApiKeyCommand>
{
    public UpdateApiKeyCommandValidator()
    {
        RuleFor(x => x.UserId)
            .GreaterThan(0).WithMessage("Invalid User ID.");
            
        RuleFor(x => x.ApiKeyId)
            .GreaterThan(0).WithMessage("Invalid API Key ID.");

        RuleFor(x => x.Description)
            .NotEmpty().WithMessage("Description is required.")
            .MinimumLength(3).WithMessage("Description must be at least 3 characters long.")
            .MaximumLength(100).WithMessage("Description must not exceed 100 characters.");
    }
}
