using FluentValidation;
using WaifuApi.Application.Common.Utilities;

namespace WaifuApi.Application.Features.Tags.CreateTag;

public class CreateTagCommandValidator : AbstractValidator<CreateTagCommand>
{
    public CreateTagCommandValidator()
    {
        RuleFor(x => x.Name)
            .NotEmpty().WithMessage("Name is required.")
            .MaximumLength(30).WithMessage("Name must not exceed 30 characters.");

        RuleFor(x => x.Description)
            .NotEmpty().WithMessage("Description is required.")
            .MaximumLength(150).WithMessage("Description must not exceed 150 characters.");
            
        RuleFor(x => x.Slug)
            .MaximumLength(50).WithMessage("Slug must not exceed 50 characters.")
            .Must(BeAValidSlug).When(x => !string.IsNullOrEmpty(x.Slug))
            .WithMessage("Slug can only contain lowercase letters, numbers, and hyphens.");
    }

    private bool BeAValidSlug(string slug)
    {
        return System.Text.RegularExpressions.Regex.IsMatch(slug, "^[a-z0-9-]+$");
    }
}
