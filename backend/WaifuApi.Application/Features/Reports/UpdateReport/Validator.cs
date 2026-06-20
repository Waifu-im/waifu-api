using FluentValidation;

namespace WaifuApi.Application.Features.Reports.UpdateReport;

public class UpdateReportCommandValidator : AbstractValidator<UpdateReportCommand>
{
    public UpdateReportCommandValidator()
    {
        RuleFor(x => x.Description)
            .NotEmpty().WithMessage("Description is required.")
            .MaximumLength(500).WithMessage("Description must not exceed 500 characters.");
    }
}
