using FluentValidation;

namespace WaifuApi.Application.Features.Reports.CreateReport;

public class CreateReportCommandValidator : AbstractValidator<CreateReportCommand>
{
    public CreateReportCommandValidator()
    {
        RuleFor(x => x.Description)
            .NotEmpty().WithMessage("Description is required.")
            .MaximumLength(500).WithMessage("Description must not exceed 500 characters.");

        RuleFor(x => x.ImageId)
            .GreaterThan(0).WithMessage("Invalid Image ID.");
            
        RuleFor(x => x.UserId)
            .GreaterThan(0).WithMessage("Invalid User ID.");
    }
}
