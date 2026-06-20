using FluentValidation;

namespace WaifuApi.Application.Features.Review.SubmitEdit;

public class SubmitEditCommandValidator : AbstractValidator<SubmitEditCommand>
{
    public SubmitEditCommandValidator()
    {
        RuleFor(x => x.SubmitterId)
            .GreaterThan(0).WithMessage("Invalid User ID.");

        RuleFor(x => x.TargetId)
            .GreaterThan(0).WithMessage("Invalid target ID.");

        RuleFor(x => x.TargetType)
            .IsInEnum().WithMessage("Invalid target type.");

        RuleFor(x => x.PayloadJson)
            .NotEmpty().WithMessage("Proposed changes are required.");

        RuleFor(x => x.Reason)
            .MaximumLength(500).WithMessage("Reason must not exceed 500 characters.");
    }
}
