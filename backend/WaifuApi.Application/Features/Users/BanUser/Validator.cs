using FluentValidation;

namespace WaifuApi.Application.Features.Users.BanUser;

public class BanUserCommandValidator : AbstractValidator<BanUserCommand>
{
    public BanUserCommandValidator()
    {
        RuleFor(x => x.Reason)
            .MaximumLength(200).WithMessage("Blacklist reason must not exceed 200 characters.")
            .When(x => x.Reason != null);
    }
}
